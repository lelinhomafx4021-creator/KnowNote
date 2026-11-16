package db

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"knownote/internal/model"
)

func Open(ctx context.Context, databaseURL string) (*gorm.DB, error) {
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true, DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化数据库连接失败")
	}
	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接池失败")
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("连接数据库失败，请检查 DATABASE_URL 与数据库状态")
	}
	return database, nil
}

func Migrate(ctx context.Context, database *gorm.DB, dimension int) error {
	if dimension < 1 || dimension > 2000 {
		return fmt.Errorf("向量维度必须在 1 到 2000 之间")
	}
	return database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 多个服务同时启动时，串行执行迁移，避免重复创建表和索引。
		if err := tx.Exec("SELECT pg_advisory_xact_lock(482947103)").Error; err != nil {
			return fmt.Errorf("获取迁移锁失败")
		}
		if err := tx.Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
			return fmt.Errorf("创建 vector 扩展失败，请检查数据库权限")
		}
		var existing int
		if err := tx.Raw(`SELECT COALESCE(MAX(a.atttypmod), 0) FROM pg_attribute a
			JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE c.relname = 'chunks' AND n.nspname = current_schema() AND a.attname = 'embedding' AND NOT a.attisdropped`).Scan(&existing).Error; err != nil {
			return fmt.Errorf("检查向量维度失败")
		}
		if existing != 0 && existing != dimension {
			return fmt.Errorf("数据库向量维度为 %d，EMBEDDING_DIM 为 %d；请停止服务，使用目标配置运行 scripts/reindex.sh 重建所有向量（保留原文与文件）", existing, dimension)
		}
		tx = tx.Set("embedding_dimension", dimension)
		for _, entry := range model.AllModels() {
			if err := tx.AutoMigrate(entry); err != nil {
				return fmt.Errorf("迁移模型 %T 失败: %w", entry, err)
			}
			switch entry.(type) {
			case *model.Notebook:
				if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_notebooks_owner ON notebooks (id, user_id)").Error; err != nil {
					return err
				}
			case *model.Source:
				if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_sources_owner ON sources (id, notebook_id, user_id)").Error; err != nil {
					return err
				}
			}
		}
		for _, statement := range []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_notebooks_sample_owner ON notebooks (user_id, sample_slug) WHERE sample_slug <> ''",
			"CREATE INDEX IF NOT EXISTS idx_login_logs_email_failed_created ON login_logs (email, created_at DESC) WHERE success = false",
			"CREATE INDEX IF NOT EXISTS idx_login_logs_user_created ON login_logs (user_id, created_at DESC)",
			"CREATE INDEX IF NOT EXISTS idx_login_logs_email_created ON login_logs (email, created_at DESC)",
			"CREATE INDEX IF NOT EXISTS idx_llm_usage_user_created ON llm_usage (user_id, created_at)",
			"CREATE INDEX IF NOT EXISTS idx_notebooks_user_updated ON notebooks (user_id, updated_at DESC)",
			"CREATE INDEX IF NOT EXISTS idx_sources_notebook_created ON sources (user_id, notebook_id, created_at)",
			"CREATE INDEX IF NOT EXISTS idx_messages_notebook_created ON messages (user_id, notebook_id, created_at)",
			"CREATE INDEX IF NOT EXISTS idx_notes_notebook_updated ON notes (user_id, notebook_id, updated_at DESC)",
			"CREATE INDEX IF NOT EXISTS idx_artifacts_notebook_created ON artifacts (user_id, notebook_id, created_at DESC)",
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_guides_notebook ON notebook_guides (notebook_id)",
			"CREATE INDEX IF NOT EXISTS idx_chunks_source_owner ON chunks (source_id, notebook_id, user_id)",
			"CREATE INDEX IF NOT EXISTS idx_chunks_user_notebook ON chunks (user_id, notebook_id)",
			"CREATE INDEX IF NOT EXISTS idx_guides_user_notebook ON notebook_guides (user_id, notebook_id)",
			"CREATE INDEX IF NOT EXISTS idx_chunks_embedding_cosine ON chunks USING hnsw (embedding vector_cosine_ops)",
		} {
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("创建查询索引失败: %w", err)
			}
		}
		return migrateCitations(tx)
	})
}
