package ingest

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"knownote/internal/llm"
	"knownote/internal/model"
	"knownote/internal/parser"
	"knownote/internal/rag"
	"knownote/internal/service"
	"knownote/internal/storage"
)

const WorkerCount = 3

// Queue 以 processing 来源作为持久队列，内存只保存正在执行的任务。
type Queue struct {
	db        *gorm.DB
	client    llm.Client
	storage   *storage.Local
	parser    *parser.Parser
	logger    *slog.Logger
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	wake      chan struct{}
	jobs      chan model.Source
	mu        sync.Mutex
	active    map[string]bool
	dimension int
}

func New(ctx context.Context, database *gorm.DB, client llm.Client, files *storage.Local, dimension int, logger *slog.Logger) *Queue {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(ctx)
	q := &Queue{db: database, client: client, storage: files, parser: parser.New(), logger: logger, cancel: cancel,
		wake: make(chan struct{}, 1), jobs: make(chan model.Source, WorkerCount), active: make(map[string]bool), dimension: dimension}
	q.wg.Add(WorkerCount + 1)
	for i := 0; i < WorkerCount; i++ {
		go q.worker(ctx)
	}
	go q.dispatch(ctx)
	q.Notify()
	return q
}

func (q *Queue) Notify() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}
func (q *Queue) Close() { q.cancel(); q.wg.Wait() }

func (q *Queue) dispatch(ctx context.Context) {
	defer q.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
		case <-ticker.C:
		}
		q.mu.Lock()
		capacity := WorkerCount - len(q.active)
		ids := make([]string, 0, len(q.active))
		for id := range q.active {
			ids = append(ids, id)
		}
		q.mu.Unlock()
		if capacity <= 0 {
			continue
		}
		var sources []model.Source
		query := q.db.WithContext(ctx).Select("id", "user_id", "notebook_id").Where("status = ?", "processing")
		if len(ids) > 0 {
			query = query.Where("id NOT IN ?", ids)
		}
		if err := query.Order("created_at ASC, id ASC").Limit(capacity).Find(&sources).Error; err != nil {
			if ctx.Err() == nil {
				q.logger.Warn("读取来源处理队列失败，将自动重试")
			}
			continue
		}
		for _, source := range sources {
			q.mu.Lock()
			q.active[source.ID] = true
			q.mu.Unlock()
			select {
			case q.jobs <- source:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case source := <-q.jobs:
			q.run(ctx, source)
			q.mu.Lock()
			delete(q.active, source.ID)
			q.mu.Unlock()
			q.Notify()
		}
	}
}

type stageError struct{ message string }

func (e stageError) Error() string { return e.message }

func (q *Queue) run(ctx context.Context, source model.Source) {
	// 取消时保留 processing，供下次启动恢复；删除后的记录不会被重新创建。
	defer func() {
		if recover() != nil {
			q.fail(ctx, source, "来源处理异常，请重新添加来源")
		}
	}()
	jobCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	err := q.process(jobCtx, source)
	if err == nil || ctx.Err() != nil || errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	message := "来源处理失败，请稍后重新添加"
	var stage stageError
	if errors.As(err, &stage) {
		message = stage.message
	}
	if errors.Is(jobCtx.Err(), context.DeadlineExceeded) {
		message = "来源处理超时，请稍后重新添加"
	}
	q.fail(ctx, source, message)
}

func (q *Queue) fail(ctx context.Context, source model.Source, message string) {
	if ctx.Err() != nil {
		return
	}
	result := q.db.WithContext(ctx).Model(&model.Source{}).Where("user_id = ? AND notebook_id = ? AND id = ? AND status = ?", source.UserID, source.NotebookID, source.ID, "processing").Updates(map[string]any{"status": "error", "error": message})
	if result.Error != nil {
		q.logger.Error("保存来源处理错误失败，重启后会重新处理", "sourceID", source.ID)
	} else if result.RowsAffected > 0 {
		q.logger.Warn("来源处理失败", "sourceID", source.ID, "reason", message)
	}
}

func (q *Queue) process(ctx context.Context, key model.Source) error {
	var source model.Source
	if err := q.db.WithContext(ctx).Where("user_id = ? AND notebook_id = ? AND id = ? AND status = ?", key.UserID, key.NotebookID, key.ID, "processing").First(&source).Error; err != nil {
		return err
	}
	input := parser.Input{Type: source.Type, URL: source.URL, Text: source.Content}
	if source.FilePath != "" {
		file, err := q.storage.Open(source.FilePath)
		if err != nil {
			return stageError{"读取上传文件失败，文件可能已被删除"}
		}
		defer file.Close()
		input.File = file
	}
	parsed, err := q.parser.Parse(ctx, input)
	if err != nil {
		return stageError{err.Error()}
	}
	content := parser.Clean(parsed.Content)
	if content == "" {
		return stageError{"未能从来源中提取文字，请提供包含正文的资料"}
	}
	parts := rag.Chunk(content)
	vectors, err := q.client.Embed(ctx, parts)
	if err != nil {
		return stageError{"来源向量化失败，请检查模型配置或稍后重试"}
	}
	if len(vectors) != len(parts) {
		return stageError{"向量化结果数量不正确"}
	}
	chunks := make([]model.Chunk, len(parts))
	for i, part := range parts {
		if len(vectors[i]) != q.dimension {
			return stageError{"向量维度与数据库配置不一致"}
		}
		chunks[i] = model.Chunk{NotebookOwner: source.NotebookOwner, SourceID: source.ID, ChunkIndex: i, Content: part, TokenCount: rag.WordCount(part), Embedding: model.NewVector(vectors[i])}
	}
	// 先事务替换分块；只有最终 ready 后检索器才会读取这些分块。
	if err := q.lockSource(ctx, source, func(tx *gorm.DB, _ model.Source) error {
		if err := tx.Where("user_id = ? AND notebook_id = ? AND source_id = ?", source.UserID, source.NotebookID, source.ID).Delete(&model.Chunk{}).Error; err != nil {
			return err
		}
		return tx.Omit(clause.Associations).CreateInBatches(&chunks, 64).Error
	}); err != nil {
		return err
	}
	raw, err := q.client.Chat(ctx, llm.SourceSummaryMessages(content), llm.ChatOptions{Temperature: 0.2, MaxTokens: 700})
	if err != nil {
		return stageError{"来源摘要生成失败，请检查模型配置或稍后重试"}
	}
	summary, err := llm.ParseSourceSummary(raw)
	if err != nil {
		summary = llm.FallbackSourceSummary(content)
	}
	if parsed.Placeholder {
		summary.Summary = "本来源为音频或视频，本期暂不支持自动转写，尚未提取真实音视频内容。系统当前仅保存转写功能的说明性占位正文，没有分析其中的对话、观点或事实。可将字幕或转写文本粘贴为新的文本来源，处理完成后再生成内容摘要并参与带引用的问答。需要了解实际内容时，请查看原始音视频并核对字幕。"
		summary.Topics = []string{"音视频来源", "暂不支持自动转写", "可粘贴字幕文本"}
	}
	return q.lockSource(ctx, source, func(tx *gorm.DB, current model.Source) error {
		current.Content = content
		current.Summary = summary.Summary
		current.Topics = summary.Topics
		current.WordCount = rag.WordCount(content)
		current.Status = "ready"
		current.Progress = 100
		current.Error = ""
		urlRunes := []rune(source.URL)
		defaultTitle := string(urlRunes[:min(500, len(urlRunes))])
		if parsed.Title != "" && current.Title == defaultTitle {
			title := []rune(parsed.Title)
			current.Title = string(title[:min(500, len(title))])
		}
		if err := tx.Omit(clause.Associations).Save(&current).Error; err != nil {
			return err
		}
		return service.MarkSourcesChanged(tx, source.UserID, source.NotebookID)
	})
}

func (q *Queue) lockSource(ctx context.Context, source model.Source, fn func(*gorm.DB, model.Source) error) error {
	return q.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var notebook model.Notebook
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", source.UserID, source.NotebookID).First(&notebook).Error; err != nil {
			return err
		}
		var current model.Source
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND notebook_id = ? AND id = ? AND status = ?", source.UserID, source.NotebookID, source.ID, "processing").First(&current).Error; err != nil {
			return err
		}
		return fn(tx, current)
	})
}
