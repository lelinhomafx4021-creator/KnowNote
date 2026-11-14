package service

import (
	"context"
	"crypto/rand"
	"math/big"

	"gorm.io/gorm"
	"knownote/internal/model"
)

type NotebookView struct {
	model.Notebook
	SourceCount int64 `json:"sourceCount"`
}

// NotebookFileCleaner 由 T2 的 storage 实现；数据库提交后清理来源与音频文件。
// 实现须保证重复删除安全，并为失败清理安排重试。
type NotebookFileCleaner interface {
	DeleteNotebookFiles(ctx context.Context, userID, notebookID string, paths []string) error
}

func (s *Service) notebookQuery(ctx context.Context, userID string) *gorm.DB {
	return s.DB.WithContext(ctx).Model(&model.Notebook{}).
		Select("notebooks.*, (SELECT COUNT(*) FROM sources WHERE sources.notebook_id = notebooks.id AND sources.user_id = ?) AS source_count", userID).
		Where("notebooks.user_id = ?", userID)
}

func (s *Service) Notebooks(ctx context.Context, userID string) ([]NotebookView, error) {
	items := make([]NotebookView, 0)
	err := s.notebookQuery(ctx, userID).Order("notebooks.updated_at DESC, notebooks.id DESC").Scan(&items).Error
	return items, err
}

func (s *Service) Notebook(ctx context.Context, userID, id string) (NotebookView, error) {
	var item NotebookView
	result := s.notebookQuery(ctx, userID).Where("notebooks.id = ?", id).Scan(&item)
	if result.Error != nil {
		return NotebookView{}, result.Error
	}
	if result.RowsAffected == 0 {
		return NotebookView{}, ErrNotFound
	}
	return item, nil
}

func (s *Service) CreateNotebook(ctx context.Context, userID, title string) (NotebookView, error) {
	if title == "" {
		title = "未命名笔记本"
	}
	emojis := []string{"📘", "📚", "🌱", "💡", "🔬", "🎨", "🌍", "✨"}
	emojiIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(emojis))))
	if err != nil {
		return NotebookView{}, err
	}
	color, err := rand.Int(rand.Reader, big.NewInt(8))
	if err != nil {
		return NotebookView{}, err
	}
	notebook := model.Notebook{UserID: userID, Title: title, Emoji: emojis[emojiIndex.Int64()], Color: int(color.Int64())}
	if err := s.DB.WithContext(ctx).Create(&notebook).Error; err != nil {
		return NotebookView{}, err
	}
	return NotebookView{Notebook: notebook}, nil
}

func (s *Service) UpdateNotebook(ctx context.Context, userID, id string, updates map[string]any) (NotebookView, error) {
	if len(updates) != 0 {
		result := s.DB.WithContext(ctx).Model(&model.Notebook{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates)
		if result.Error != nil {
			return NotebookView{}, result.Error
		}
		if result.RowsAffected == 0 {
			return NotebookView{}, ErrNotFound
		}
	}
	return s.Notebook(ctx, userID, id)
}

func (s *Service) DeleteNotebook(ctx context.Context, userID, id string) error {
	paths := make([]string, 0)
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 与上传共用笔记本行锁，确保收集到并发上传的文件后再级联删除。
		if err := lockNotebook(tx, userID, id); err != nil {
			return err
		}
		if s.Files != nil {
			if err := tx.Model(&model.Source{}).Where("user_id = ? AND notebook_id = ? AND file_path <> ''", userID, id).Pluck("file_path", &paths).Error; err != nil {
				return err
			}
			var audioPaths []string
			if err := tx.Model(&model.AudioOverview{}).Where("user_id = ? AND notebook_id = ? AND file_path <> ''", userID, id).Pluck("file_path", &audioPaths).Error; err != nil {
				return err
			}
			paths = append(paths, audioPaths...)
		}
		result := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Notebook{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	if s.Files != nil {
		if err := s.Files.DeleteNotebookFiles(ctx, userID, id, paths); err != nil {
			s.Logger.Warn("笔记本数据已删除，文件清理需要重试", "notebookID", id)
		}
	}
	return nil
}
