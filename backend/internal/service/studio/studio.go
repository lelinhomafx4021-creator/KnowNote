package studio

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"knownote/internal/exporter"
	"knownote/internal/llm"
	"knownote/internal/model"
	"knownote/internal/rag"
	"knownote/internal/service"
)

type Service struct {
	DB        *gorm.DB
	Client    llm.Client
	Retriever *rag.Retriever
	Logger    *slog.Logger
}

type CreateArtifactRequest struct {
	Kind      string
	Topic     string
	Preset    string
	SourceIDs []string
}

func New(database *gorm.DB, client llm.Client, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		DB:        database,
		Client:    client,
		Retriever: rag.NewRetriever(database),
		Logger:    logger,
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

func (s *Service) selectReadySources(ctx context.Context, userID, notebookID string, sourceIDs []string) (model.Notebook, []model.Source, error) {
	return s.selectReadySourcesInTx(s.DB.WithContext(ctx), userID, notebookID, sourceIDs, false)
}

func (s *Service) selectReadySourcesInTx(tx *gorm.DB, userID, notebookID string, sourceIDs []string, lock bool) (model.Notebook, []model.Source, error) {
	book, err := notebook(tx, userID, notebookID, lock)
	if err != nil {
		return book, nil, err
	}
	var sources []model.Source
	if sourceIDs != nil {
		ids := make([]string, 0, len(sourceIDs))
		for _, id := range sourceIDs {
			trimmed := strings.TrimSpace(id)
			if trimmed == "" {
				continue
			}
			if _, err := uuid.Parse(trimmed); err != nil {
				return book, nil, invalid("来源标识格式无效")
			}
			ids = append(ids, trimmed)
		}
		slices.Sort(ids)
		ids = slices.Compact(ids)
		if len(ids) == 0 {
			return book, nil, invalid("请先选择至少一个已就绪来源")
		}
		if err := tx.Select("id", "title", "status", "summary", "topics").
			Where("user_id = ? AND notebook_id = ? AND id IN ?", userID, notebookID, ids).
			Order("created_at,id").Find(&sources).Error; err != nil {
			return book, nil, err
		}
		if len(sources) != len(ids) {
			return book, nil, service.ErrNotFound
		}
		for _, source := range sources {
			if source.Status != "ready" {
				return book, nil, invalid("所选来源尚未处理完成，请等待来源就绪")
			}
		}
		return book, sources, nil
	}
	if err := tx.Select("id", "title", "status", "summary", "topics").
		Where("user_id = ? AND notebook_id = ? AND status = ?", userID, notebookID, "ready").
		Order("created_at,id").Find(&sources).Error; err != nil {
		return book, nil, err
	}
	if len(sources) == 0 {
		return book, nil, invalid("请先添加来源")
	}
	return book, sources, nil
}

func (s *Service) summaryContext(ctx context.Context, userID, notebookID string, sources []model.Source, perSourceLimit int) (string, error) {
	ids := make([]string, len(sources))
	for i, src := range sources {
		ids[i] = src.ID
	}
	var chunks []struct {
		SourceID   string
		Content    string
		ChunkIndex int
	}
	// 主题命中数优先，再按长度和原文顺序稳定排序；不加载向量。
	err := s.DB.WithContext(ctx).Raw(`SELECT s.id AS source_id, picked.content, picked.chunk_index
		FROM sources s CROSS JOIN LATERAL (
			SELECT c.content, c.chunk_index FROM chunks c
			WHERE c.user_id = ? AND c.notebook_id = ? AND c.source_id = s.id
			ORDER BY (SELECT COUNT(*) FROM jsonb_array_elements_text(s.topics) topic WHERE strpos(c.content,topic)>0) DESC,
			char_length(c.content) DESC, c.chunk_index ASC LIMIT ?
		) picked WHERE s.user_id = ? AND s.notebook_id = ? AND s.id IN ?
		ORDER BY s.created_at,s.id,picked.chunk_index`,
		userID, notebookID, max(1, perSourceLimit), userID, notebookID, ids).Scan(&chunks).Error
	if err != nil {
		return "", err
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
		return "", invalid(err.Error())
	}
	return input, nil
}

func (s *Service) ragContext(ctx context.Context, userID, notebookID string, sources []model.Source, topic string) (string, error) {
	ids := make([]string, len(sources))
	byID := make(map[string]model.Source, len(sources))
	for i, src := range sources {
		ids[i] = src.ID
		byID[src.ID] = src
	}
	vectors, err := s.Client.Embed(ctx, []string{topic})
	if err != nil {
		var limit *llm.LimitError
		if errors.As(err, &limit) {
			return "", err
		}
		return "", &service.Error{Status: http.StatusBadGateway, Code: 50201, Message: llm.PublicError(err)}
	}
	if len(vectors) != 1 {
		return "", &service.Error{Status: http.StatusBadGateway, Code: 50201, Message: "主题检索向量数量异常"}
	}
	chunks, err := s.Retriever.Search(ctx, userID, notebookID, ids, vectors[0], 18)
	if err != nil {
		return "", err
	}

	const minSim = 0.2
	selected := make([]rag.ScoredChunk, 0, len(chunks))
	candidates := make([]rag.ScoredChunk, 0, len(chunks))
	for _, c := range chunks {
		if strings.TrimSpace(c.Content) == "" {
			continue
		}
		candidates = append(candidates, c)
		if c.Score >= minSim {
			selected = append(selected, c)
		}
	}
	if len(selected) == 0 && len(candidates) > 0 {
		sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
		selected = candidates[:min(6, len(candidates))]
	}
	if len(selected) == 0 {
		return s.summaryContext(ctx, userID, notebookID, sources, 3)
	}

	// 按最高检索相关性排前各来源，来源内按 ChunkIndex 正序恢复连贯阅读顺序。
	chunksBySource := map[string][]rag.ScoredChunk{}
	bestScore := map[string]float64{}
	var matchedIDs []string
	for _, c := range selected {
		if _, ok := chunksBySource[c.SourceID]; !ok {
			matchedIDs = append(matchedIDs, c.SourceID)
			bestScore[c.SourceID] = c.Score
		} else if c.Score > bestScore[c.SourceID] {
			bestScore[c.SourceID] = c.Score
		}
		chunksBySource[c.SourceID] = append(chunksBySource[c.SourceID], c)
	}
	sort.SliceStable(matchedIDs, func(i, j int) bool { return bestScore[matchedIDs[i]] > bestScore[matchedIDs[j]] })

	inputs := make([]llm.StudioSource, 0, len(sources))
	seen := make(map[string]bool, len(matchedIDs))
	for _, sid := range matchedIDs {
		seen[sid] = true
		src := byID[sid]
		group := chunksBySource[sid]
		sort.SliceStable(group, func(i, j int) bool { return group[i].ChunkIndex < group[j].ChunkIndex })
		passages := make([]string, 0, len(group))
		for _, c := range group {
			if c.LocationLabel != "" {
				passages = append(passages, fmt.Sprintf("[%s] %s", c.LocationLabel, c.Content))
			} else {
				passages = append(passages, c.Content)
			}
		}
		inputs = append(inputs, llm.StudioSource{Title: src.Title, Summary: src.Summary, Topics: src.Topics, Chunks: passages})
	}
	for _, src := range sources {
		if !seen[src.ID] {
			inputs = append(inputs, llm.StudioSource{Title: src.Title, Summary: src.Summary, Topics: src.Topics})
		}
	}
	input, err := llm.BuildStudioContext(inputs, llm.StudioContextLimit)
	if err != nil {
		return "", invalid(err.Error())
	}
	return input, nil
}

func (s *Service) options(ctx context.Context, userID string) (llm.ChatOptions, error) {
	var settings model.UserSettings
	err := s.DB.WithContext(ctx).Where("user_id = ?", userID).First(&settings).Error
	if err != nil {
		return llm.ChatOptions{}, err
	}
	var role string
	_ = s.DB.WithContext(ctx).Model(&model.User{}).Select("role").Where("id = ?", userID).Scan(&role).Error
	modelID := settings.Model
	if role != "admin" {
		modelID = ""
	}
	return llm.ChatOptions{Model: modelID, Temperature: 0.3}, nil
}

func (s *Service) Artifacts(ctx context.Context, userID, notebookID string) ([]model.Artifact, error) {
	if _, err := notebook(s.DB.WithContext(ctx), userID, notebookID, false); err != nil {
		return nil, err
	}
	items := make([]model.Artifact, 0)
	err := s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ?", userID, notebookID).Order("created_at DESC,id DESC").Find(&items).Error
	return items, err
}

func (s *Service) Artifact(ctx context.Context, userID, id string) (model.Artifact, error) {
	var item model.Artifact
	err := s.DB.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).First(&item).Error
	return item, notFound(err)
}

func (s *Service) CreateArtifact(ctx context.Context, userID, notebookID, kind string) (model.Artifact, error) {
	return s.CreateArtifactWithRequest(ctx, userID, notebookID, CreateArtifactRequest{Kind: kind})
}

func (s *Service) CreateArtifactWithRequest(ctx context.Context, userID, notebookID string, req CreateArtifactRequest) (model.Artifact, error) {
	ctx = llm.WithUser(ctx, userID)
	baseTitle, ok := llm.StudioTitles[req.Kind]
	if !ok {
		return model.Artifact{}, invalid("不支持的工作室产物类型")
	}
	topic := strings.TrimSpace(req.Topic)
	if strings.ContainsRune(topic, '\x00') || utf8.RuneCountInString(topic) > 500 {
		return model.Artifact{}, invalid("备课主题不能超过500个字符或包含空字符")
	}
	preset := strings.TrimSpace(req.Preset)
	if presets, hasPresets := llm.StudioPresets[req.Kind]; preset != "" {
		if !hasPresets || presets[preset] == "" {
			preset = ""
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	book, sources, err := s.selectReadySources(ctx, userID, notebookID, req.SourceIDs)
	if err != nil {
		return model.Artifact{}, err
	}

	var input string
	retrievalMode := "sources"
	if topic != "" {
		retrievalMode = "rag"
		input, err = s.ragContext(ctx, userID, notebookID, sources, topic)
	} else {
		perSource := 3
		if req.Kind == "ppt" || req.Kind == "word" || req.Kind == "pdf" {
			perSource = 5
		}
		input, err = s.summaryContext(ctx, userID, notebookID, sources, perSource)
	}
	if err != nil {
		return model.Artifact{}, err
	}

	opts, err := s.options(ctx, userID)
	if err != nil {
		return model.Artifact{}, err
	}
	isStructuredJSON := req.Kind == "mindmap" || req.Kind == "ppt"
	opts.JSON = isStructuredJSON

	title := baseTitle
	if topic != "" {
		runes := []rune(topic)
		if len(runes) > 28 {
			topicLabel := strings.TrimSpace(string(runes[:28])) + "…"
			title = fmt.Sprintf("%s · %s", baseTitle, topicLabel)
		} else {
			title = fmt.Sprintf("%s · %s", baseTitle, topic)
		}
	}

	item := model.Artifact{
		NotebookOwner: model.NotebookOwner{UserID: userID, NotebookID: notebookID},
		Kind:          req.Kind,
		Title:         title,
		Format:        exporter.DefaultFormat(req.Kind),
		Preset:        preset,
		Prompt:        topic,
		RetrievalMode: retrievalMode,
	}

	messages := llm.StudioMessagesWithFocus(req.Kind, book.Title, topic, preset, input)
	for attempt := 0; attempt < 2; attempt++ {
		raw, generateErr := s.Client.Chat(ctx, messages, opts)
		var limit *llm.LimitError
		if errors.As(generateErr, &limit) {
			return model.Artifact{}, generateErr
		}
		if generateErr == nil {
			switch req.Kind {
			case "mindmap":
				item.Mindmap, generateErr = ParseMindmap(raw)
			case "ppt":
				var mdContent string
				item.Slides, mdContent, generateErr = ParseSlideDeck(raw)
				if generateErr == nil {
					item.Content = mdContent
					if topic == "" && item.Slides != nil && item.Slides.Title != "" {
						item.Title = "PPT · " + item.Slides.Title
					}
				}
			default:
				item.Content = strings.TrimSpace(raw)
				if item.Content == "" || strings.ContainsRune(item.Content, '\x00') {
					generateErr = errors.New("产物内容为空或无效")
				}
			}
		}
		if generateErr == nil {
			err = nil
			break
		}
		err = generateErr
		if !isStructuredJSON {
			return model.Artifact{}, &service.Error{Status: 502, Code: 50201, Message: "内容生成失败，请重试"}
		}
		retryHint := "上次输出格式无效。请重新生成完整 JSON，严格遵守深度、分支数和标签长度限制。"
		if req.Kind == "ppt" {
			retryHint = "上次输出的 PPT 课件 JSON 格式无效。请仅输出合法 JSON 对象 {\"title\":...,\"subtitle\":...,\"slides\":[{\"title\":...,\"bullets\":[...],\"notes\":...}]}，不要输出 Markdown 代码围栏。"
		}
		messages = append(messages, llm.Message{Role: "user", Content: retryHint})
	}
	if err != nil {
		msg := "思维导图生成失败，请重试"
		if req.Kind == "ppt" {
			msg = "PPT 课件生成失败，请重试"
		}
		return model.Artifact{}, &service.Error{Status: 502, Code: 50201, Message: msg}
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
