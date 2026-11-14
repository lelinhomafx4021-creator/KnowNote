package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"knownote/internal/config"
	"knownote/internal/db"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/service"
	"knownote/internal/service/ingest"
	"knownote/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("服务启动或运行失败", "error", err)
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
	database, err := db.Open(ctx, c.DatabaseURL)
	if err != nil {
		return err
	}
	sqlDB, err := database.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败")
	}
	defer sqlDB.Close()
	migrateCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	err = db.Migrate(migrateCtx, database, c.EmbeddingDim)
	cancel()
	if err != nil {
		return err
	}
	logger.Info("数据库迁移完成", "embeddingDim", c.EmbeddingDim)
	client := llm.New(c)
	if client.Name() == "fake" {
		logger.Warn("未配置 LLM_API_KEY，启用 Fake LLM，可离线演示")
	} else {
		logger.Info("已启用 OpenAI 兼容模型客户端")
	}
	tokens := middleware.NewTokens(c.JWTSecret)
	s := service.New(database, tokens, c.LLMChatModel, logger)
	files, err := storage.New(context.Background(), c.DataDir, logger)
	if err != nil {
		return err
	}
	defer files.Close()
	queue := ingest.New(ctx, database, client, files, c.EmbeddingDim, logger)
	defer queue.Close()
	s.Files, s.Storage, s.Ingest = files, files, queue
	gin.SetMode(gin.ReleaseMode)
	router := handler.NewRouter(s, c.CORSOrigins, handler.SourceRoutes(s))
	if err := router.SetTrustedProxies(nil); err != nil {
		return fmt.Errorf("配置代理规则失败")
	}
	server := &http.Server{Addr: fmt.Sprintf(":%d", c.Port), Handler: router,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
		// 不设置全局写超时，避免截断后续 SSE 与同步产物生成；模型调用自身有超时。
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logger.Info("KnowNote 服务已启动", "port", c.Port, "llm", client.Name())
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP 服务监听失败")
		}
		return nil
	case <-ctx.Done():
		logger.Info("收到停止信号，等待请求完成")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("等待请求完成超时，已强制关闭连接")
	}
	queue.Close()
	logger.Info("服务已安全停止")
	return nil
}
