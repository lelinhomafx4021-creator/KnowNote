package chat

import (
	"context"
	"errors"
	"reflect"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"knownote/internal/llm"
	"knownote/internal/model"
	"knownote/internal/service"
)

var errSourcesChanged = errors.New("生成概要期间来源发生变化")

type guideSource struct {
	ID        string
	Title     string
	Summary   string
	UpdatedAt time.Time
}

func guideSources(ctx context.Context, database *gorm.DB, userID, notebookID string) ([]guideSource, error) {
	sources := make([]guideSource, 0)
	err := database.WithContext(ctx).Model(&model.Source{}).Select("id", "title", "summary", "updated_at").Where("user_id = ? AND notebook_id = ? AND status = ?", userID, notebookID, "ready").Order("created_at, id").Scan(&sources).Error
	return sources, err
}

func (s *Service) Guide(ctx context.Context, userID, notebookID string) (llm.Guide, error) {
	if err := s.notebook(ctx, userID, notebookID); err != nil {
		return llm.Guide{}, err
	}
	// 单个调用者断开不会取消其他正在等待同一概要的调用者。
	result := s.guides.DoChan(userID+":"+notebookID, func() (any, error) {
		generateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
		defer cancel()
		for attempt := 0; attempt < 3; attempt++ {
			guide, err := s.generateGuide(generateCtx, userID, notebookID)
			if !errors.Is(err, errSourcesChanged) {
				return guide, err
			}
		}
		return llm.Guide{}, &service.Error{Status: 409, Code: 40901, Message: "来源正在更新，请稍后获取概要"}
	})
	select {
	case <-ctx.Done():
		return llm.Guide{}, ctx.Err()
	case result := <-result:
		if result.Err != nil {
			return llm.Guide{}, result.Err
		}
		return result.Val.(llm.Guide), nil
	}
}

func (s *Service) generateGuide(ctx context.Context, userID, notebookID string) (llm.Guide, error) {
	var cached model.NotebookGuide
	err := s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ?", userID, notebookID).First(&cached).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return llm.Guide{}, err
	}
	if err == nil && !cached.Stale {
		return llm.Guide{Summary: cached.Summary, SuggestedQuestions: cached.SuggestedQuestions}, nil
	}
	sources, err := guideSources(ctx, s.DB, userID, notebookID)
	if err != nil {
		return llm.Guide{}, err
	}
	if len(sources) == 0 {
		return llm.Guide{SuggestedQuestions: []string{}}, nil
	}
	inputs := make([]llm.GuideSource, 0, len(sources))
	for _, source := range sources {
		inputs = append(inputs, llm.GuideSource{Title: source.Title, Summary: source.Summary})
	}
	var settings model.UserSettings
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).First(&settings).Error; err != nil {
		return llm.Guide{}, err
	}
	raw, err := s.Client.Chat(ctx, llm.GuideMessages(inputs), llm.ChatOptions{Model: settings.Model, Temperature: 0.2})
	if err != nil {
		return llm.Guide{}, err
	}
	guide, err := llm.ParseGuide(raw)
	if err != nil {
		s.Logger.Warn("概要格式不符合要求，使用摘要摘录降级", "notebookID", notebookID)
		guide = llm.FallbackGuide(inputs)
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 和 T2 共用笔记本行锁，避免生成期间的来源失效标记被覆盖。
		var notebook model.Notebook
		err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, notebookID).First(&notebook).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return service.ErrNotFound
		}
		if err != nil {
			return err
		}
		current, err := guideSources(ctx, tx, userID, notebookID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(sources, current) {
			return errSourcesChanged
		}
		cache := model.NotebookGuide{NotebookOwner: model.NotebookOwner{UserID: userID, NotebookID: notebookID}, Summary: guide.Summary, SuggestedQuestions: guide.SuggestedQuestions, Stale: false}
		// Stale 的模型默认值为 true，显式 Update 保证新缓存也能标为有效。
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "notebook_id"}}, DoUpdates: clause.AssignmentColumns([]string{"summary", "suggested_questions", "updated_at"})}).Create(&cache).Error; err != nil {
			return err
		}
		return tx.Model(&model.NotebookGuide{}).Where("user_id = ? AND notebook_id = ?", userID, notebookID).Update("stale", false).Error
	})
	return guide, err
}
