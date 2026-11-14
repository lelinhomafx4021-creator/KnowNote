package studio

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"knownote/internal/config"
	"knownote/internal/llm"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/storage"
)

type Service struct {
	DB       *gorm.DB
	Client   llm.Client
	Files    storage.Store
	Logger   *slog.Logger
	TTSModel string
	Voices   []string
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	wake     chan struct{}
}

func New(ctx context.Context, database *gorm.DB, client llm.Client, files storage.Store, cfg config.Config, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{DB: database, Client: client, Files: files, TTSModel: cfg.LLMTTSModel, Voices: append([]string(nil), cfg.LLMTTSVoices...), Logger: logger, cancel: cancel, wake: make(chan struct{}, 1)}
	s.wg.Add(1)
	go s.audioWorker(ctx)
	s.notify()
	return s
}
func (s *Service) Close() { s.cancel(); s.wg.Wait() }
func (s *Service) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func invalid(message string) error { return &service.Error{Status: 400, Code: 40001, Message: message} }
func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return service.ErrNotFound
	}
	return err
}
func notebook(tx *gorm.DB, userID, id string, lock bool) (model.Notebook, error) {
	var item model.Notebook
	if lock {
		tx = tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := tx.Where("user_id = ? AND id = ?", userID, id).First(&item).Error
	return item, notFound(err)
}

func (s *Service) context(ctx context.Context, userID, notebookID string) (model.Notebook, string, error) {
	book, err := notebook(s.DB.WithContext(ctx), userID, notebookID, false)
	if err != nil {
		return book, "", err
	}
	var sources []model.Source
	if err := s.DB.WithContext(ctx).Select("id", "title", "summary", "topics").Where("user_id = ? AND notebook_id = ? AND status = ?", userID, notebookID, "ready").Order("created_at,id").Find(&sources).Error; err != nil {
		return book, "", err
	}
	if len(sources) == 0 {
		return book, "", invalid("请先添加来源")
	}
	// 主题命中数优先，再按长度和原文顺序稳定排序；每来源最多取3块，不加载向量。
	var chunks []struct {
		SourceID   string
		Content    string
		ChunkIndex int
	}
	err = s.DB.WithContext(ctx).Raw(`SELECT s.id AS source_id, picked.content, picked.chunk_index
		FROM sources s CROSS JOIN LATERAL (
			SELECT c.content, c.chunk_index FROM chunks c
			WHERE c.user_id = ? AND c.notebook_id = ? AND c.source_id = s.id
			ORDER BY (SELECT COUNT(*) FROM jsonb_array_elements_text(s.topics) topic WHERE strpos(c.content,topic)>0) DESC,
			char_length(c.content) DESC, c.chunk_index ASC LIMIT 3
		) picked WHERE s.user_id = ? AND s.notebook_id = ? AND s.status = 'ready'
		ORDER BY s.created_at,s.id,picked.chunk_index`, userID, notebookID, userID, notebookID).Scan(&chunks).Error
	if err != nil {
		return book, "", err
	}
	bySource := map[string][]string{}
	for _, chunk := range chunks {
		bySource[chunk.SourceID] = append(bySource[chunk.SourceID], chunk.Content)
	}
	inputs := make([]llm.StudioSource, 0, len(sources))
	for _, source := range sources {
		inputs = append(inputs, llm.StudioSource{Title: source.Title, Summary: source.Summary, Topics: source.Topics, Chunks: bySource[source.ID]})
	}
	input, err := llm.BuildStudioContext(inputs, llm.StudioContextLimit)
	if err != nil {
		return book, "", invalid(err.Error())
	}
	return book, input, nil
}

func (s *Service) options(ctx context.Context, userID string) (llm.ChatOptions, error) {
	var settings model.UserSettings
	err := s.DB.WithContext(ctx).Where("user_id = ?", userID).First(&settings).Error
	return llm.ChatOptions{Model: settings.Model, Temperature: 0.3, MaxTokens: 5000}, err
}

func (s *Service) Artifacts(ctx context.Context, userID, notebookID string) ([]model.Artifact, error) {
	if _, err := notebook(s.DB.WithContext(ctx), userID, notebookID, false); err != nil {
		return nil, err
	}
	items := make([]model.Artifact, 0)
	err := s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ?", userID, notebookID).Order("created_at DESC,id DESC").Find(&items).Error
	return items, err
}

func (s *Service) CreateArtifact(ctx context.Context, userID, notebookID, kind string) (model.Artifact, error) {
	title, ok := llm.StudioTitles[kind]
	if !ok {
		return model.Artifact{}, invalid("不支持的工作室产物类型")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	book, input, err := s.context(ctx, userID, notebookID)
	if err != nil {
		return model.Artifact{}, err
	}
	opts, err := s.options(ctx, userID)
	if err != nil {
		return model.Artifact{}, err
	}
	item := model.Artifact{NotebookOwner: model.NotebookOwner{UserID: userID, NotebookID: notebookID}, Kind: kind, Title: title}
	messages := llm.StudioMessages(kind, book.Title, input)
	for attempt := 0; attempt < 2; attempt++ {
		raw, generateErr := s.Client.Chat(ctx, messages, opts)
		if generateErr == nil && kind == "mindmap" {
			item.Mindmap, generateErr = ParseMindmap(raw)
		}
		if generateErr == nil && kind != "mindmap" {
			item.Content = strings.TrimSpace(raw)
			if item.Content == "" || strings.ContainsRune(item.Content, '\x00') {
				generateErr = errors.New("产物内容为空或无效")
			}
		}
		if generateErr == nil {
			err = nil
			break
		}
		err = generateErr
		if kind != "mindmap" {
			return model.Artifact{}, &service.Error{Status: 502, Code: 50201, Message: "内容生成失败，请重试"}
		}
		messages = append(messages, llm.Message{Role: "user", Content: "上次输出格式无效。请重新生成完整 JSON，严格遵守深度、分支数和标签长度限制。"})
	}
	if err != nil {
		return model.Artifact{}, &service.Error{Status: 502, Code: 50201, Message: "思维导图生成失败，请重试"}
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := notebook(tx, userID, notebookID, true); err != nil {
			return err
		}
		return tx.Create(&item).Error
	})
	return item, err
}

func (s *Service) DeleteArtifact(ctx context.Context, userID, id string) error {
	result := s.DB.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).Delete(&model.Artifact{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return service.ErrNotFound
	}
	return nil
}
