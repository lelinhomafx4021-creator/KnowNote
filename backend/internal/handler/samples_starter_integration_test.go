package handler_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/service"
	"knownote/internal/service/chat"
	"knownote/internal/service/ingest"
	"knownote/internal/service/samples"
	"knownote/internal/storage"
	contents "knownote/samples"
)

func TestStarterNotebookOnRegisterAndSamplesList(t *testing.T) {
	database := sourceDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := storage.New(ctx, t.TempDir(), logger)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	client := llm.NewFake(1536)
	queue := ingest.New(ctx, database, client, store, 1536, logger)
	defer queue.Close()

	s := service.New(database, middleware.NewTokens(strings.Repeat("s", 32)), "gpt-4o-mini", logger)
	s.Files, s.Storage, s.Ingest = store, store, queue
	chats := chat.New(database, client, logger)
	manager, err := samples.New(ctx, s, chats, contents.Content)
	if err != nil {
		t.Fatal(err)
	}
	manager.Start(ctx)
	defer manager.Close()
	s.OnUserCreated = manager.SeedStarterForUser

	gin.SetMode(gin.TestMode)
	router := handler.NewRouter(s, []string{"http://localhost:5174"}, handler.SampleRoutes(s, manager))
	a := &testAPI{t: t, router: router, database: database}

	// 验证新用户注册后立即拥有初始《知页使用指南》笔记本与 4 本精选示例模板。
	user := decode[service.AuthResult](t, a.call("POST", "/api/auth/register", "", `{"email":"starter-user@example.com","password":"starter-password","nickname":"新同学"}`, 200))
	notebooks := decode[[]service.NotebookView](t, a.call("GET", "/api/notebooks", user.Token, "", 200))
	if len(notebooks) != 1 || notebooks[0].Title != "知页使用指南" || notebooks[0].SourceCount < 2 {
		t.Fatalf("新用户注册后应自动获得初始指南笔记本，实际: %+v", notebooks)
	}

	// 验证多次触发幂等，不会重复克隆初始笔记本。
	if err := manager.EnsureStarter(ctx, user.User.ID); err != nil {
		t.Fatal(err)
	}
	notebooksAgain := decode[[]service.NotebookView](t, a.call("GET", "/api/notebooks", user.Token, "", 200))
	if len(notebooksAgain) != 1 {
		t.Fatalf("EnsureStarter 应保持幂等，实际数量: %d", len(notebooksAgain))
	}

	sampleViews := decode[[]samples.View](t, a.call("GET", "/api/samples", user.Token, "", 200))
	if len(sampleViews) != 4 {
		t.Fatalf("预期 4 本精选示例笔记本，实际: %d", len(sampleViews))
	}
	if sampleViews[0].Slug != samples.StarterSlug || sampleViews[0].AddedNotebookID != notebooks[0].ID {
		t.Fatalf("精选示例状态应关联初始笔记本: %+v", sampleViews[0])
	}
}
