package chat

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"knownote/internal/llm"
	"knownote/internal/model"
	"knownote/internal/rag"
	"knownote/internal/service"
	"knownote/pkg/response"
)

type Event struct {
	Type      string           `json:"type"`
	Citations []model.Citation `json:"citations,omitempty"`
	Text      string           `json:"text,omitempty"`
	MessageID string           `json:"messageId,omitempty"`
	Message   string           `json:"message,omitempty"`
}

type Service struct {
	DB        *gorm.DB
	Client    llm.Client
	Retriever *rag.Retriever
	Logger    *slog.Logger
	guides    singleflight.Group
}

func New(database *gorm.DB, client llm.Client, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{DB: database, Client: client, Retriever: rag.NewRetriever(database), Logger: logger}
}

func invalid(message string) error {
	return &service.Error{Status: http.StatusBadRequest, Code: response.CodeInvalid, Message: message}
}

func (s *Service) notebook(ctx context.Context, userID, notebookID string) error {
	var notebook model.Notebook
	err := s.DB.WithContext(ctx).Select("id").Where("user_id = ? AND id = ?", userID, notebookID).First(&notebook).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return service.ErrNotFound
	}
	return err
}

func (s *Service) Messages(ctx context.Context, userID, notebookID string) ([]model.Message, error) {
	if err := s.notebook(ctx, userID, notebookID); err != nil {
		return nil, err
	}
	items := make([]model.Message, 0)
	err := s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ?", userID, notebookID).Order("created_at, id").Find(&items).Error
	return items, err
}

func (s *Service) Clear(ctx context.Context, userID, notebookID string) error {
	if err := s.notebook(ctx, userID, notebookID); err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ?", userID, notebookID).Delete(&model.Message{}).Error
}

func (s *Service) Feedback(ctx context.Context, userID, messageID string, value *string) error {
	result := s.DB.WithContext(ctx).Model(&model.Message{}).Where("user_id = ? AND id = ?", userID, messageID).Update("feedback", value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return service.ErrNotFound
	}
	return nil
}

type Request struct {
	UserID, NotebookID, Question string
	SourceIDs                    []string
	Settings                     model.UserSettings
	History                      []model.Message
}

// Prepare 在发送 SSE 头之前完成归属和就绪校验，使参数错误仍使用标准 HTTP 响应。
func (s *Service) Prepare(ctx context.Context, userID, notebookID, question string, sourceIDs []string) (*Request, error) {
	if err := s.notebook(ctx, userID, notebookID); err != nil {
		return nil, err
	}
	ids := slices.Clone(sourceIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) > 0 {
		var sources []model.Source
		if err := s.DB.WithContext(ctx).Select("id", "status").Where("user_id = ? AND notebook_id = ? AND id IN ?", userID, notebookID, ids).Find(&sources).Error; err != nil {
			return nil, err
		}
		if len(sources) != len(ids) {
			return nil, service.ErrNotFound
		}
		for _, source := range sources {
			if source.Status != "ready" {
				return nil, invalid("所选来源尚未处理完成，请等待来源就绪")
			}
		}
	}
	request := &Request{UserID: userID, NotebookID: notebookID, Question: question, SourceIDs: ids}
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).First(&request.Settings).Error; err != nil {
		return nil, err
	}
	// 最多十二条消息对应六轮；prompt 层再按 user 边界裁剪。
	if err := s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ?", userID, notebookID).Order("created_at DESC, id DESC").Limit(12).Find(&request.History).Error; err != nil {
		return nil, err
	}
	slices.Reverse(request.History)
	return request, nil
}

func (s *Service) Stream(ctx context.Context, request *Request, emit func(Event) error) {
	ctx = llm.WithUser(ctx, request.UserID)
	fail := func(message string) { _ = emit(Event{Type: "error", Message: message}) }
	owner := model.NotebookOwner{UserID: request.UserID, NotebookID: request.NotebookID}
	user := model.Message{NotebookOwner: owner, Role: "user", Content: request.Question, Citations: []model.Citation{}}
	if err := s.DB.WithContext(ctx).Create(&user).Error; err != nil {
		fail("问题保存失败，请重试")
		return
	}
	var output strings.Builder
	citations := make([]model.Citation, 0)
	var streamErr error
	if len(request.SourceIDs) == 0 {
		streamErr = emit(Event{Type: "citations", Citations: citations})
		if streamErr == nil {
			output.WriteString("请选择来源后再提问。")
			streamErr = emit(Event{Type: "delta", Text: output.String()})
		}
	} else {
		var passages []rag.Passage
		vectors, err := s.Client.Embed(ctx, []string{request.Question})
		if err == nil && len(vectors) != 1 {
			err = errors.New("问题向量数量不正确")
		}
		if err == nil {
			chunks, searchErr := s.Retriever.Search(ctx, request.UserID, request.NotebookID, request.SourceIDs, vectors[0], 6)
			err = searchErr
			passages = BuildPassages(chunks, s.Logger)
		}
		if err == nil {
			for _, passage := range passages {
				citations = append(citations, passage.Citation)
			}
			err = emit(Event{Type: "citations", Citations: citations})
		}
		if err == nil {
			_, err = s.Client.ChatStream(ctx, rag.BuildPrompt(request.Question, request.Settings.AnswerStyle, request.History, passages), llm.ChatOptions{Model: request.Settings.Model, Temperature: 0.2}, func(delta string) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if delta == "" {
					return nil
				}
				output.WriteString(delta)
				return emit(Event{Type: "delta", Text: delta})
			})
		}
		streamErr = err
	}
	// 请求取消后仍给持久化保留独立、有限的时间；不继续调用模型。
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	assistant := model.Message{NotebookOwner: owner, Role: "assistant", Content: output.String(), Citations: FilterCitations(output.String(), citations)}
	err := s.DB.WithContext(saveCtx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&assistant).Error; err != nil {
			return err
		}
		return tx.Model(&model.Notebook{}).Where("user_id = ? AND id = ?", request.UserID, request.NotebookID).Update("updated_at", time.Now()).Error
	})
	if err != nil {
		s.Logger.Error("回答保存失败", "notebookID", request.NotebookID)
		fail("回答保存失败，请稍后重试")
		return
	}
	if ctx.Err() != nil {
		s.Logger.Info("客户端已断开，停止生成并保存部分回答", "messageID", assistant.ID, "characters", utf8.RuneCountInString(assistant.Content))
		return
	}
	if streamErr != nil {
		s.Logger.Warn("问答生成中断，已保存生成内容", "messageID", assistant.ID)
		fail(llm.PublicError(streamErr))
		return
	}
	_ = emit(Event{Type: "done", MessageID: assistant.ID})
}
