package main

import (
	"context"
	"knownote/internal/config"
	"knownote/internal/db"
	"knownote/internal/llm"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("重建失败，原向量事务已回滚", "error", err)
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	c, err := config.Load(logger)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := llm.New(c)
	if err := llm.ProbeEmbedding(ctx, client, c); err != nil {
		return err
	}
	database, err := db.Open(ctx, c.DatabaseURL)
	if err != nil {
		return err
	}
	sqlDB, _ := database.DB()
	defer sqlDB.Close()
	logger.Info("开始重建所有来源向量，请确保业务服务已停止", "模型", c.EmbedModel, "维度", c.EmbeddingDim)
	// 旧数据库先使用原维度迁移用量表，重建成功后才改变向量列。
	var dimension int
	if err := database.Raw("SELECT atttypmod FROM pg_attribute WHERE attrelid='chunks'::regclass AND attname='embedding'").Scan(&dimension).Error; err != nil {
		return err
	}
	if err := db.Migrate(ctx, database, dimension); err != nil {
		return err
	}
	if err := db.Reindex(ctx, database, llm.NewGuard(database, client, c), c.EmbeddingDim); err != nil {
		return err
	}
	logger.Info("所有来源向量重建完成，可以启动服务")
	return nil
}
