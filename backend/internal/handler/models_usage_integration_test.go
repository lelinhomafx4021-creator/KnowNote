package handler_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"knownote/internal/config"
	"knownote/internal/db"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/service/chat"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestModelsAndUsageProtection(t *testing.T) {
	database := sourceDatabase(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var calls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if fail.Load() {
			w.WriteHeader(503)
			fmt.Fprint(w, `{"error":{"message":"敏感上游凭据 local-test-secret"}}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"正式回答"}}],"usage":{"prompt_tokens":11,"completion_tokens":7}}`)
	}))
	defer server.Close()
	cfg := config.Config{ChatBaseURL: server.URL, ChatAPIKey: "local-test-secret", ChatModel: "deepseek-flash", ChatModels: []string{"deepseek-flash", "deepseek-v4-pro"}, EmbedModel: "text-embedding-v4", EmbeddingDim: 1536, QuotaDailyChatRequests: 1, QuotaDailyTokens: 100000, LLMMaxConcurrency: 8}
	guard := llm.NewGuard(database, llm.New(cfg), cfg)
	core := service.New(database, middleware.NewTokens(strings.Repeat("u", 32)), cfg.ChatModel, logger)
	core.Models, core.Usage = llm.Catalog(cfg), guard
	chats := chat.New(database, guard, logger)
	a := &testAPI{t: t, database: database, router: handler.NewRouter(core, nil, handler.ChatRoutes(core, chats))}
	owner, other := a.register(), a.register()
	a.call("GET", "/api/models", "", "", 401)
	catalog := decode[llm.ModelCatalog](t, a.call("GET", "/api/models", owner.Token, "", 200))
	if len(catalog.Chat) != 2 || catalog.Embed.Provider != "fake" || catalog.TTS.Enabled || catalog.Image.Enabled {
		t.Fatal("模型列表能力信息错误")
	}
	a.call("PATCH", "/api/me/settings", owner.Token, `{"model":"not-allowed"}`, 400)
	selected := decode[service.SettingsView](t, a.call("PATCH", "/api/me/settings", owner.Token, `{"model":"deepseek-v4-pro"}`, 200))
	if selected.Model != "deepseek-v4-pro" {
		t.Fatal("可用模型未保存")
	}
	ctx := llm.WithUser(context.Background(), owner.User.ID)
	if _, err := guard.Chat(ctx, []llm.Message{{Role: "user", Content: "测试"}}, llm.ChatOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := guard.Chat(ctx, nil, llm.ChatOptions{}); !errors.Is(err, llm.ErrQuota) || calls.Load() != 1 {
		t.Fatal("每日请求超额仍调用上游")
	}
	usage := decode[llm.UsageView](t, a.call("GET", "/api/me/usage", owner.Token, "", 200))
	if usage.ChatRequests != 1 || usage.Tokens != 18 || usage.ReservedTokens != 0 || usage.ChatRequestsLimit != 1 {
		t.Fatalf("用量结算错误：%+v", usage)
	}
	isolated := decode[llm.UsageView](t, a.call("GET", "/api/me/usage", other.Token, "", 200))
	if isolated.Tokens != 0 {
		t.Fatal("用量未按用户隔离")
	}
	book := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	source := model.Source{NotebookOwner: model.NotebookOwner{UserID: owner.User.ID, NotebookID: book.ID}, Type: "text", Title: "资料", Status: "ready"}
	if err := database.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	events := streamChat(t, a, "/api/notebooks/"+book.ID, owner.Token, `{"question":"问题","sourceIds":["`+source.ID+`"]}`)
	if events[len(events)-1].Type != "error" || events[len(events)-1].Message != llm.ErrQuota.Error() {
		t.Fatal("SSE 未返回中文额度提示")
	}
	// 额度为零表示不限；对话、向量化与语音共用 Token 额度。
	guard.Config.QuotaDailyChatRequests = 0
	guard.Config.QuotaDailyTokens = 1
	before := calls.Load()
	if _, err := guard.Embed(llm.WithUser(context.Background(), other.User.ID), []string{"超出预算"}); !errors.Is(err, llm.ErrQuota) || calls.Load() != before {
		t.Fatal("Token 超额未阻止向量化")
	}
	guard.Config.QuotaDailyTokens = 0
	if _, err := guard.Chat(ctx, nil, llm.ChatOptions{}); err != nil {
		t.Fatal("零额度应不限")
	}
	fail.Store(true)
	_, err := guard.Chat(ctx, nil, llm.ChatOptions{})
	if err == nil || strings.Contains(err.Error(), "local-test-secret") || strings.Contains(err.Error(), "敏感") {
		t.Fatal("上游错误未脱敏")
	}
	var failed model.LLMUsage
	if err := database.Where("user_id = ? AND success = false AND capability = 'chat'", owner.User.ID).First(&failed).Error; err != nil || failed.Pending {
		t.Fatal("失败调用未记录与结算")
	}
	// 速率限制持久化，多个 Guard 实例共享同一用户窗口。
	records := make([]model.LLMUsage, 20)
	for i := range records {
		records[i] = model.LLMUsage{UserID: other.User.ID, Capability: "chat", Model: "test", Success: true}
	}
	if err := database.Create(&records).Error; err != nil {
		t.Fatal(err)
	}
	guard2 := llm.NewGuard(database, llm.New(cfg), config.Config{ChatModel: "deepseek-flash", LLMMaxConcurrency: 8})
	if _, err := guard2.Chat(llm.WithUser(context.Background(), other.User.ID), nil, llm.ChatOptions{}); !errors.Is(err, llm.ErrRate) {
		t.Fatal("每分钟限制未生效")
	}
	if _, err := core.UpdateSettings(context.Background(), owner.User.ID, map[string]any{"model": "invalid"}); err == nil {
		t.Fatal("服务层未校验模型")
	}
}

type blockedClient struct {
	llm.Client
	started chan struct{}
	release chan struct{}
}

func (c *blockedClient) Chat(ctx context.Context, m []llm.Message, o llm.ChatOptions) (string, error) {
	c.started <- struct{}{}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-c.release:
		return "完成", nil
	}
}
func TestUsageReservationConcurrencyAndCancellation(t *testing.T) {
	database := sourceDatabase(t)
	core := service.New(database, middleware.NewTokens(strings.Repeat("r", 32)), "fake", slog.New(slog.NewTextHandler(io.Discard, nil)))
	owner, err := core.Register(context.Background(), "reservation@example.com", "password", "测试")
	if err != nil {
		t.Fatal(err)
	}
	client := &blockedClient{Client: llm.NewFake(1536), started: make(chan struct{}, 2), release: make(chan struct{})}
	cfg := config.Config{ChatModel: "fake", EmbeddingDim: 1536, LLMMaxConcurrency: 2, QuotaDailyChatRequests: 1}
	guard := llm.NewGuard(database, client, cfg)
	ctx := llm.WithUser(context.Background(), owner.User.ID)
	done := make(chan error, 1)
	go func() { _, err := guard.Chat(ctx, nil, llm.ChatOptions{}); done <- err }()
	<-client.started
	if _, err := guard.Chat(ctx, nil, llm.ChatOptions{}); !errors.Is(err, llm.ErrQuota) {
		t.Fatal("并发预留未阻止超额")
	}
	usage, err := guard.Usage(ctx, owner.User.ID)
	if err != nil || usage.ReservedTokens != 4096 {
		t.Fatal("处理中预算未预留")
	}
	close(client.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// 全局并发上限覆盖所有用户，取消后释放槽位并保存失败记录。
	client.release = make(chan struct{})
	guard = llm.NewGuard(database, client, config.Config{ChatModel: "fake", EmbeddingDim: 1536, LLMMaxConcurrency: 1})
	cancelCtx, cancel := context.WithCancel(ctx)
	go func() { _, err := guard.Chat(cancelCtx, nil, llm.ChatOptions{}); done <- err }()
	<-client.started
	if _, err := guard.Embed(ctx, []string{"测试"}); !errors.Is(err, llm.ErrBusy) {
		t.Fatal("全局并发未限制")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal("调用取消未透传")
	}
	if _, err := guard.Embed(ctx, []string{"测试"}); err != nil {
		t.Fatal("取消后并发槽位未释放")
	}
	var pending int64
	database.Model(&model.LLMUsage{}).Where("user_id = ? AND pending = true", owner.User.ID).Count(&pending)
	if pending != 0 {
		t.Fatal("取消后预留未释放")
	}
	// 独立实例并发抢占最后一次额度也只能有一个成功。
	other, err := core.Register(context.Background(), "parallel@example.com", "password", "测试")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var success atomic.Int32
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g := llm.NewGuard(database, llm.NewFake(1536), cfg)
			if _, err := g.Chat(llm.WithUser(context.Background(), other.User.ID), nil, llm.ChatOptions{}); err == nil {
				success.Add(1)
			} else if !errors.Is(err, llm.ErrQuota) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatal("数据库额度预留未串行")
	}
}

type failingEmbed struct{ llm.Client }

func (f failingEmbed) Embed(context.Context, []string) ([][]float32, error) {
	return nil, errors.New("模拟失败")
}
func TestReindexPreservesSourcesAndRollsBack(t *testing.T) {
	database := sourceDatabase(t)
	core := service.New(database, middleware.NewTokens(strings.Repeat("x", 32)), "fake", slog.New(slog.NewTextHandler(io.Discard, nil)))
	user, err := core.Register(context.Background(), "reindex@example.com", "password", "测试")
	if err != nil {
		t.Fatal(err)
	}
	book, err := core.CreateNotebook(context.Background(), user.User.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	source := model.Source{NotebookOwner: model.NotebookOwner{UserID: user.User.ID, NotebookID: book.ID}, Title: "重建来源", Type: "pdf", Status: "ready", Content: "知识管理与检索", FilePath: "保留文件路径"}
	if err := database.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	values, _ := llm.NewFake(1536).Embed(context.Background(), []string{source.Content})
	page := 3
	chunk := model.Chunk{NotebookOwner: source.NotebookOwner, SourceID: source.ID, Content: source.Content, Page: &page, LocationLabel: "第 3 页", Embedding: model.NewVector(values[0])}
	if err := database.Create(&chunk).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Reindex(context.Background(), database, llm.NewFake(64), 64); err != nil {
		t.Fatal(err)
	}
	var saved model.Chunk
	database.Where("user_id = ? AND id = ?", user.User.ID, chunk.ID).First(&saved)
	if len(saved.Embedding.Slice()) != 64 || saved.Page == nil || *saved.Page != 3 || saved.LocationLabel != "第 3 页" {
		t.Fatal("重建丢失位置或维度未更改")
	}
	if err := db.Reindex(context.Background(), database, failingEmbed{llm.NewFake(512)}, 512); err == nil {
		t.Fatal("模型失败应回滚")
	}
	var dim int
	database.Raw("SELECT atttypmod FROM pg_attribute WHERE attrelid='chunks'::regclass AND attname='embedding'").Scan(&dim)
	if dim != 64 {
		t.Fatal("失败未回滚列维度")
	}
	var original model.Source
	database.Where("user_id = ? AND id = ?", user.User.ID, source.ID).First(&original)
	if original.FilePath != source.FilePath || original.Content != source.Content {
		t.Fatal("重建修改了原文或文件")
	}
	if err := db.Migrate(context.Background(), database, 64); err != nil {
		t.Fatal(err)
	}
}
