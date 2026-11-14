package db

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"knownote/internal/llm"
	"knownote/internal/model"
)

// Reindex 是显式运维操作，遍历所有用户已有分块；事务期间资料读写会等待。
// 整个变更在一个事务中，失败回滚到原维度和原向量，不修改原文、文件或引用位置。
func Reindex(ctx context.Context, database *gorm.DB, client llm.Client, dimension int) error {
	return ReindexWithProgress(ctx, database, client, dimension, nil)
}

// ReindexWithProgress 复用同一事务重建；进度为暂存处理数，提交后才持久生效。
func ReindexWithProgress(ctx context.Context, database *gorm.DB, client llm.Client, dimension int, progress func(total, completed int64)) error {
	if dimension < 1 || dimension > 2000 {
		return fmt.Errorf("向量维度必须在 1 到 2000 之间")
	}
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(482947103)").Error; err != nil {
			return err
		}
		if err := tx.Exec("LOCK TABLE sources, chunks IN ACCESS EXCLUSIVE MODE").Error; err != nil {
			return err
		}
		if err := tx.Exec("CREATE TEMP TABLE reindex_chunks ON COMMIT DROP AS SELECT id, created_at, updated_at, user_id, notebook_id, source_id, page, location_label, section_index, chunk_index, content, token_count FROM chunks").Error; err != nil {
			return err
		}
		if err := tx.Exec("CREATE TEMP TABLE reindex_citation_links ON COMMIT DROP AS SELECT id, chunk_id FROM message_citations WHERE chunk_id IS NOT NULL").Error; err != nil {
			return err
		}
		for _, sql := range []string{"DELETE FROM chunks", "DROP INDEX IF EXISTS idx_chunks_embedding_cosine", fmt.Sprintf("ALTER TABLE chunks ALTER COLUMN embedding TYPE vector(%d)", dimension)} {
			if err := tx.Exec(sql).Error; err != nil {
				return err
			}
		}
		var total, completed int64
		if err := tx.Table("reindex_chunks").Count(&total).Error; err != nil {
			return err
		}
		if progress != nil {
			progress(total, 0)
		}
		last := "00000000-0000-0000-0000-000000000000"
		for {
			var chunks []model.Chunk
			if err := tx.Table("reindex_chunks").Where("id > ?", last).Order("id").Limit(64).Find(&chunks).Error; err != nil {
				return err
			}
			if len(chunks) == 0 {
				break
			}
			// 按用户调用，确保向量重建同样遵守额度并记录归属。
			grouped := map[string][]int{}
			for i := range chunks {
				grouped[chunks[i].UserID] = append(grouped[chunks[i].UserID], i)
			}
			for userID, positions := range grouped {
				texts := make([]string, len(positions))
				for i, pos := range positions {
					texts[i] = chunks[pos].Content
				}
				values, err := client.Embed(llm.WithUser(ctx, userID), texts)
				if err != nil {
					return fmt.Errorf("重建向量失败：%s；事务已回滚，可调整配置或额度后重试", llm.PublicError(err))
				}
				if len(values) != len(positions) {
					return fmt.Errorf("向量服务返回数量不匹配，重建已回滚")
				}
				for i, pos := range positions {
					if len(values[i]) != dimension {
						return fmt.Errorf("向量服务维度不匹配，重建已回滚")
					}
					chunks[pos].Embedding = model.NewVector(values[i])
				}
			}
			if err := tx.Omit("Notebook", "Source").CreateInBatches(&chunks, 64).Error; err != nil {
				return err
			}
			completed += int64(len(chunks))
			if progress != nil {
				progress(total, completed)
			}
			last = chunks[len(chunks)-1].ID
		}
		if err := tx.Exec("UPDATE message_citations SET chunk_id = links.chunk_id FROM reindex_citation_links links WHERE message_citations.id = links.id").Error; err != nil {
			return err
		}
		if err := tx.Exec("CREATE INDEX idx_chunks_embedding_cosine ON chunks USING hnsw (embedding vector_cosine_ops)").Error; err != nil {
			return err
		}
		// 未建立分块的失败来源在重启后由持久队列重新处理。
		return tx.Exec("UPDATE sources SET status='processing', progress=0, error='' WHERE content <> '' AND status IN ('ready','error') AND NOT EXISTS (SELECT 1 FROM chunks WHERE chunks.source_id=sources.id AND chunks.user_id=sources.user_id)").Error
	})
}
