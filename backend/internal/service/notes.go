package service

import (
	"context"

	"gorm.io/gorm"
	"knownote/internal/model"
)

func (s *Service) Notes(ctx context.Context, userID, notebookID string) ([]model.Note, error) {
	if _, err := s.Notebook(ctx, userID, notebookID); err != nil {
		return nil, err
	}
	items := make([]model.Note, 0)
	err := s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ?", userID, notebookID).Order("updated_at DESC, id DESC").Find(&items).Error
	return items, err
}

func (s *Service) Note(ctx context.Context, userID, id string) (model.Note, error) {
	var item model.Note
	err := s.DB.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).First(&item).Error
	return item, databaseError(err)
}

func (s *Service) CreateNote(ctx context.Context, userID, notebookID, title, content, origin string) (model.Note, error) {
	if origin != "manual" && origin != "chat" && origin != "studio" {
		return model.Note{}, invalidSource("笔记来源必须是 manual、chat 或 studio")
	}
	item := model.Note{NotebookOwner: model.NotebookOwner{UserID: userID, NotebookID: notebookID}, Title: title, Content: content, Origin: origin}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockNotebook(tx, userID, notebookID); err != nil {
			return err
		}
		return tx.Create(&item).Error
	})
	return item, err
}

func (s *Service) UpdateNote(ctx context.Context, userID, id string, updates map[string]any) (model.Note, error) {
	var item model.Note
	if len(updates) > 0 {
		result := s.DB.WithContext(ctx).Model(&model.Note{}).Where("user_id = ? AND id = ?", userID, id).Updates(updates)
		if result.Error != nil {
			return item, result.Error
		}
		if result.RowsAffected == 0 {
			return item, ErrNotFound
		}
	}
	err := s.DB.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).First(&item).Error
	return item, databaseError(err)
}

func (s *Service) DeleteNote(ctx context.Context, userID, id string) error {
	result := s.DB.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).Delete(&model.Note{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
