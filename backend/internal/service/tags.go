package service

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"knownote/internal/model"
	"slices"
	"time"
)

func (s *Service) Tags(ctx context.Context, userID string) ([]model.Tag, error) {
	items := make([]model.Tag, 0)
	err := s.DB.WithContext(ctx).Where("user_id = ?", userID).Order("created_at, id").Find(&items).Error
	return items, err
}
func tagError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return &Error{409, 40901, "该标签名称已存在"}
	}
	return err
}
func (s *Service) CreateTag(ctx context.Context, userID, name, color string) (model.Tag, error) {
	item := model.Tag{UserID: userID, Name: name, Color: color}
	err := s.DB.WithContext(ctx).Create(&item).Error
	return item, tagError(err)
}
func (s *Service) UpdateTag(ctx context.Context, userID, id string, updates map[string]any) (model.Tag, error) {
	var item model.Tag
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, id).First(&item).Error; err != nil {
			return databaseError(err)
		}
		if err := tx.Model(&item).Updates(updates).Error; err != nil {
			return tagError(err)
		}
		return tx.Where("user_id = ? AND id = ?", userID, id).First(&item).Error
	})
	return item, err
}
func (s *Service) DeleteTag(ctx context.Context, userID, id string) error {
	result := s.DB.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).Delete(&model.Tag{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Service) SetNotebookTags(ctx context.Context, userID, id string, ids []string) (NotebookView, error) {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockNotebook(tx, userID, id); err != nil {
			return err
		}
		if len(ids) > 0 {
			var tags []model.Tag
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id = ? AND id IN ?", userID, ids).Order("id").Find(&tags).Error; err != nil {
				return err
			}
			if len(tags) != len(ids) {
				return ErrNotFound
			}
		}
		if err := tx.Where("notebook_id = ?", id).Delete(&model.NotebookTag{}).Error; err != nil {
			return err
		}
		for _, tagID := range ids {
			if err := tx.Omit(clause.Associations).Create(&model.NotebookTag{NotebookID: id, TagID: tagID}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.Notebook{}).Where("user_id = ? AND id = ?", userID, id).Update("updated_at", time.Now()).Error
	})
	if err != nil {
		return NotebookView{}, err
	}
	return s.Notebook(ctx, userID, id)
}

// attachTags 批量装配，列表不产生逐笔记本查询。
func (s *Service) attachTags(ctx context.Context, userID string, items []NotebookView) error {
	ids := make([]string, len(items))
	positions := map[string]int{}
	for i := range items {
		ids[i] = items[i].ID
		positions[ids[i]] = i
		items[i].Tags = []model.Tag{}
	}
	if len(ids) == 0 {
		return nil
	}
	var rows []struct {
		model.Tag
		NotebookID string
	}
	err := s.DB.WithContext(ctx).Table("tags").Select("tags.*, notebook_tags.notebook_id").Joins("JOIN notebook_tags ON notebook_tags.tag_id = tags.id").Joins("JOIN notebooks ON notebooks.id = notebook_tags.notebook_id").Where("tags.user_id = ? AND notebooks.user_id = ? AND notebooks.id IN ?", userID, userID, ids).Order("tags.created_at, tags.id").Scan(&rows).Error
	for _, row := range rows {
		i := positions[row.NotebookID]
		items[i].Tags = append(items[i].Tags, row.Tag)
	}
	return err
}
