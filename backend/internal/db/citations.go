package db

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"knownote/internal/model"
	"time"
)

// migrateCitations 在启动迁移锁和同一事务内运行；旧快照无法可靠反推分块，chunk_id 留空。
func migrateCitations(tx *gorm.DB) error {
	tx = tx.Session(&gorm.Session{})
	const version = "t10_message_citations"
	var count int64
	if err := tx.Model(&model.SchemaMigration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var batch []model.Message
	err := tx.Where("jsonb_array_length(citations) > 0").FindInBatches(&batch, 100, func(batchTx *gorm.DB, _ int) error {
		batchTx = tx.Session(&gorm.Session{NewDB: true})
		for _, message := range batch {
			for _, citation := range message.Citations {
				if citation.Index < 1 {
					continue
				}
				row := model.MessageCitation{MessageID: message.ID, SourceRef: citation.SourceID, SourceTitle: citation.SourceTitle, Index: citation.Index, Quote: citation.Quote, Page: citation.Page, LocationLabel: citation.LocationLabel}
				var source model.Source
				// 用文本比较兼容已删除或非 UUID 的旧快照，不将失效链接伪装成有效外键。
				if err := batchTx.Select("id").Where("id::text = ? AND user_id = ? AND notebook_id = ?", citation.SourceID, message.UserID, message.NotebookID).Limit(1).Find(&source).Error; err != nil {
					return err
				}
				if source.ID != "" {
					row.SourceID = &source.ID
				}
				if err := batchTx.Omit(clause.Associations).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}).Error
	if err != nil {
		return err
	}
	return tx.Create(&model.SchemaMigration{Version: version, AppliedAt: time.Now()}).Error
}
