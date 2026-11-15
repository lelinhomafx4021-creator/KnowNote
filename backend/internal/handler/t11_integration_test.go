package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"knownote/internal/config"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/settings"
	"knownote/internal/storage"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestT11AdminSettingsAndHotSwap(t *testing.T) {
	database := sourceDatabase(t)
	ctx := context.Background()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	started, release := make(chan struct{}), make(chan struct{})
	var callsA, callsB atomic.Int32
	var badEmbed, blockEmbed atomic.Bool
	reindexStarted, reindexRelease := make(chan struct{}), make(chan struct{})
	const plain = "sk-local-test-never-return-1234"
	server := func(name string, count *atomic.Int32) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			var input struct {
				Stream bool     `json:"stream"`
				Input  []string `json:"input"`
			}
			_ = json.NewDecoder(r.Body).Decode(&input)
			switch r.URL.Path {
			case "/chat/completions":
				count.Add(1)
				if input.Stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"%s开始\"}}]}\n\n", name)
					w.(http.Flusher).Flush()
					if name == "A" {
						close(started)
						select {
						case <-release:
						case <-r.Context().Done():
							return
						}
					}
					fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"%s结束\"}}]}\n\ndata: [DONE]\n\n", name)
				} else {
					fmt.Fprintf(w, `{"choices":[{"message":{"content":"%s回答"}}],"usage":{"prompt_tokens":2,"completion_tokens":3}}`, name)
				}
			case "/embeddings":
				if blockEmbed.Load() {
					close(reindexStarted)
					select {
					case <-reindexRelease:
					case <-r.Context().Done():
						return
					}
				}
				dim := 1536
				if badEmbed.Load() {
					dim = 2
				}
				data := []any{}
				for i := range input.Input {
					v := make([]float32, dim)
					v[0] = 1
					data = append(data, map[string]any{"index": i, "embedding": v})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
			default:
				w.WriteHeader(401)
				fmt.Fprintf(w, `{"error":{"message":"%s"}}`, plain)
			}
		}))
	}
	aServer, bServer := server("A", &callsA), server("B", &callsB)
	defer aServer.Close()
	defer bServer.Close()
	cfg := config.Config{ChatBaseURL: aServer.URL, ChatAPIKey: "sk-env-local-0000", ChatModel: "chat-a", ChatModels: []string{"chat-a"}, EmbedBaseURL: aServer.URL, EmbedAPIKey: "sk-embed-local-0000", EmbedModel: "text-embedding-v4", EmbeddingDim: 1536, JWTSecret: strings.Repeat("t", 32), QuotaDailyChatRequests: 200, QuotaDailyTokens: 200000, LLMMaxConcurrency: 8, SettingSources: map[string]string{"CHAT_BASE_URL": "env", "CHAT_API_KEY": "env", "CHAT_MODEL": "env", "EMBED_API_KEY": "env"}}
	manager, err := settings.New(database, cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	s := service.New(database, middleware.NewTokens(cfg.JWTSecret), cfg.ChatModel, logger)
	s.Provider = manager.Provider
	s.Usage = llm.NewGuard(database, manager.Provider, cfg)
	s.Admin = service.NewAdminService(ctx, database, manager)
	defer s.Admin.Close()
	a := &testAPI{t: t, router: handler.NewRouter(s, nil), database: database}
	admin, ordinary := a.register(), a.register()
	if admin.User.Role != "admin" || ordinary.User.Role != "user" {
		t.Fatal("首用户管理员认定错误")
	}
	a.call("GET", "/api/admin/settings", ordinary.Token, "", 403)
	me := decode[service.UserView](t, a.call("GET", "/api/me", admin.Token, "", 200))
	if me.Role != "admin" {
		t.Fatal("me 缺少角色")
	}
	view := decode[settings.View](t, a.call("GET", "/api/admin/settings", admin.Token, "", 200))
	if view.Chat[0].Source != "env" || view.Quota[0].Source != "default" || view.Chat[1].Value != "sk-****0000" {
		t.Fatal("环境配置来源或密钥掩码错误")
	}
	save := func(v map[string]string, status int) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"values": v})
		return a.call("PATCH", "/api/admin/settings", admin.Token, string(b), status)
	}
	testBody, _ := json.Marshal(map[string]any{"values": map[string]string{"CHAT_BASE_URL": bServer.URL}, "capability": "chat"})
	result := decode[settings.TestResult](t, a.call("POST", "/api/admin/settings/test", admin.Token, string(testBody), 200))
	if !result.OK {
		t.Fatal("只测试失败")
	}
	var count int64
	database.Model(&model.SystemSetting{}).Count(&count)
	if count != 0 {
		t.Fatal("只测试不应写配置")
	}
	streamResult := make(chan string, 1)
	go func() {
		out, e := s.Usage.ChatStream(llm.WithUser(ctx, admin.User.ID), nil, llm.ChatOptions{}, func(string) error { return nil })
		if e != nil {
			streamResult <- "失败"
		} else {
			streamResult <- out
		}
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("旧流未启动")
	}
	view = decode[settings.View](t, save(map[string]string{"CHAT_BASE_URL": bServer.URL, "CHAT_API_KEY": plain, "CHAT_MODEL": "chat-b", "CHAT_MODELS": "chat-b,chat-c"}, 200))
	if view.Chat[0].Source != "database" || view.Chat[1].Value != "sk-****1234" {
		t.Fatal("数据库优先级或新掩码错误")
	}
	out, e := s.Usage.Chat(llm.WithUser(ctx, admin.User.ID), nil, llm.ChatOptions{})
	if e != nil || out != "B回答" {
		t.Fatal("保存后新请求未到 B")
	}
	close(release)
	select {
	case out := <-streamResult:
		if out != "A开始A结束" {
			t.Fatal("热更新中断或替换了旧流")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("旧流未结束")
	}
	models := decode[llm.ModelCatalog](t, a.call("GET", "/api/models", admin.Token, "", 200))
	if models.Chat[0].ID != "chat-b" {
		t.Fatal("模型目录未热更新")
	}
	current := decode[service.SettingsView](t, a.call("GET", "/api/me/settings", admin.Token, "", 200))
	if current.Model != "chat-b" {
		t.Fatal("旧用户模型未回退新默认")
	}
	var row model.SystemSetting
	database.Where("key = 'CHAT_API_KEY'").First(&row)
	if strings.Contains(row.Value, plain) || !row.IsSecret {
		t.Fatal("密钥未加密")
	}
	decrypted, err := manager.Decrypt(row.Value)
	if err != nil || decrypted != plain {
		t.Fatal("密钥解密不一致")
	}
	encrypted, _ := manager.Encrypt(plain)
	if encrypted == row.Value {
		t.Fatal("加密应使用随机 nonce")
	}
	before := row.Value
	save(map[string]string{"CHAT_API_KEY": ""}, 200)
	database.Where("key = 'CHAT_API_KEY'").First(&row)
	if row.Value != before {
		t.Fatal("空值修改了密钥")
	}
	badTestBody, _ := json.Marshal(map[string]any{"values": map[string]string{"CHAT_BASE_URL": bServer.URL + "/wrong"}, "capability": "chat"})
	badTest := decode[settings.TestResult](t, a.call("POST", "/api/admin/settings/test", admin.Token, string(badTestBody), 200))
	if badTest.OK || strings.Contains(badTest.Message, plain) {
		t.Fatal("失败连接测试未脱敏")
	}
	badSave := save(map[string]string{"CHAT_BASE_URL": bServer.URL + "/wrong"}, 400)
	if strings.Contains(string(badSave), plain) {
		t.Fatal("错误响应泄露密钥")
	}
	out, e = s.Usage.Chat(llm.WithUser(ctx, admin.User.ID), nil, llm.ChatOptions{})
	if e != nil || out != "B回答" || manager.Provider.Current().Config.ChatBaseURL != bServer.URL {
		t.Fatal("测试失败污染原配置")
	}
	save(map[string]string{"EMBEDDING_DIM": "64"}, 400)
	save(map[string]string{"QUOTA_DAILY_TOKENS": "abc"}, 400)
	save(map[string]string{"QUOTA_DAILY_CHAT_REQUESTS": "1"}, 200)
	if _, e = s.Usage.Chat(llm.WithUser(ctx, admin.User.ID), nil, llm.ChatOptions{}); e != llm.ErrQuota {
		t.Fatal("Guard 未读取热更新额度")
	}
	save(map[string]string{"QUOTA_DAILY_CHAT_REQUESTS": "0", "QUOTA_DAILY_TOKENS": "0"}, 200)
	otherCfg := cfg
	otherCfg.SettingsEncryptionKey = "changed-master-key"
	fallback, err := settings.New(database, otherCfg, logger)
	if err != nil || fallback.Provider.Current().Config.ChatAPIKey != cfg.ChatAPIKey {
		t.Fatal("主密钥变化后未回退")
	}
	if !strings.Contains(logs.String(), "解密失败") || strings.Contains(logs.String(), plain) {
		t.Fatal("解密告警缺失或泄露密钥")
	}
	save(map[string]string{"CHAT_API_KEY": settings.Clear}, 200)
	database.Model(&model.SystemSetting{}).Where("key = 'CHAT_API_KEY'").Count(&count)
	if count != 0 || manager.Provider.Current().Config.ChatAPIKey != cfg.ChatAPIKey {
		t.Fatal("清除未回退环境配置")
	}
	badEmbed.Store(true)
	save(map[string]string{"EMBED_MODEL": "embed-new"}, 400)
	if manager.Provider.Current().Config.EmbedModel != "text-embedding-v4" {
		t.Fatal("维度测试失败仍保存")
	}
	badEmbed.Store(false)
	view = decode[settings.View](t, save(map[string]string{"EMBED_MODEL": "embed-new"}, 200))
	if !view.ReindexRequired {
		t.Fatal("缺少向量重建提示")
	}
	notebook := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", admin.Token, `{}`, 200))
	seedNotebookData(t, database, admin.User.ID, notebook.ID)
	a.call("POST", "/api/admin/reindex", admin.Token, `{}`, 200)
	deadline := time.Now().Add(10 * time.Second)
	for s.Admin.Progress().Running && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	p := decode[service.ReindexProgress](t, a.call("GET", "/api/admin/reindex", admin.Token, "", 200))
	if p.Running || p.Total != 1 || p.Completed != 1 || p.Failed != 0 {
		t.Fatal("异步重建进度错误", p)
	}
	blockEmbed.Store(true)
	a.call("POST", "/api/admin/reindex", admin.Token, `{}`, 200)
	select {
	case <-reindexStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("重建任务未启动")
	}
	a.call("POST", "/api/admin/reindex", admin.Token, `{}`, 409)
	save(map[string]string{"EMBED_MODEL": "blocked-model"}, 400)
	// 重建期间额度仍可热更新。
	save(map[string]string{"QUOTA_DAILY_TOKENS": "0"}, 200)
	blockEmbed.Store(false)
	badEmbed.Store(true)
	close(reindexRelease)
	deadline = time.Now().Add(10 * time.Second)
	for s.Admin.Progress().Running && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	failed := s.Admin.Progress()
	if failed.Running || failed.Failed == 0 || failed.Completed != 0 {
		t.Fatal("失败重建状态错误", failed)
	}
	var chunkCount int64
	database.Model(&model.Chunk{}).Where("user_id = ?", admin.User.ID).Count(&chunkCount)
	if chunkCount != 1 {
		t.Fatal("失败重建未回滚")
	}
	badEmbed.Store(false)
	view = decode[settings.View](t, a.call("GET", "/api/admin/settings", admin.Token, "", 200))
	if view.ReindexRequired {
		t.Fatal("重建成功未清理提示")
	}
	page := decode[service.UserPage](t, a.call("GET", "/api/admin/users?query="+admin.User.Email+"&pageSize=1", admin.Token, "", 200))
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].NotebookCount != 1 || page.Items[0].TodayTokens == 0 {
		t.Fatal("管理员用户列表统计错误")
	}
	usage := decode[service.AdminUsage](t, a.call("GET", "/api/admin/usage", admin.Token, "", 200))
	if len(usage.Days) != 7 || len(usage.TopUsers) == 0 {
		t.Fatal("7 天统计或 Top10 缺失")
	}
	path := "/api/admin/users/" + ordinary.User.ID
	a.call("PATCH", path, ordinary.Token, `{"role":"admin"}`, 403)
	a.call("PATCH", "/api/admin/users/"+admin.User.ID, admin.Token, `{"disabled":true}`, 400)
	a.call("PATCH", "/api/admin/users/"+admin.User.ID, admin.Token, `{"role":"user"}`, 400)
	a.call("PATCH", path, admin.Token, `{"role":"admin"}`, 200)
	a.call("GET", "/api/admin/users", ordinary.Token, "", 200)
	a.call("PATCH", path, admin.Token, `{"role":"user","disabled":true}`, 200)
	a.call("GET", "/api/me", ordinary.Token, "", 403)
	a.call("POST", "/api/auth/login", "", `{"email":"`+ordinary.User.Email+`","password":"test-password"}`, 403)
	a.call("PATCH", path, admin.Token, `{"disabled":false}`, 200)
	a.call("GET", "/api/me", ordinary.Token, "", 200)
	a.call("GET", "/api/admin/users", ordinary.Token, "", 403)
	entries := decode[[]model.AuditLog](t, a.call("GET", "/api/admin/audit-logs", admin.Token, "", 200))
	encoded, _ := json.Marshal(entries)
	for _, key := range []string{plain, cfg.ChatAPIKey, cfg.EmbedAPIKey} {
		if strings.Contains(string(encoded), key) || strings.Contains(logs.String(), key) {
			t.Fatal("审计或日志泄露密钥")
		}
	}
	actions := map[string]bool{}
	for _, entry := range entries {
		actions[entry.Action] = true
	}
	for _, action := range []string{"修改配置", "变更角色", "停用用户", "启用用户", "启动重建"} {
		if !actions[action] {
			t.Fatal("缺少审计", action)
		}
	}
	if callsA.Load() == 0 || callsB.Load() == 0 {
		t.Fatal("未实际调用两个模拟服务")
	}
	for value, want := range map[string]string{"": "", "abc": "****", "sk-12345678": "sk-****5678"} {
		if settings.Mask(value) != want {
			t.Fatal("掩码边界错误")
		}
	}
}

func TestT11AdminEmailsAtStartupAndLogin(t *testing.T) {
	database := sourceDatabase(t)
	ctx := context.Background()
	s := service.New(database, middleware.NewTokens(strings.Repeat("e", 32)), "fake", slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.AdminEmails = []string{"designated@example.com"}
	a := &testAPI{t: t, router: handler.NewRouter(s, nil), database: database}
	first := a.register()
	if first.User.Role != "user" {
		t.Fatal("配置管理员邮箱时不应自动提升首用户")
	}
	designated := decode[service.AuthResult](t, a.call("POST", "/api/auth/register", "", `{"email":"designated@example.com","password":"test-password","nickname":"指定管理"}`, 200))
	if designated.User.Role != "admin" {
		t.Fatal("注册未按邮箱提升")
	}
	database.Model(&model.User{}).Where("id = ?", designated.User.ID).Update("role", "user")
	login := decode[service.AuthResult](t, a.call("POST", "/api/auth/login", "", `{"email":"designated@example.com","password":"test-password"}`, 200))
	if login.User.Role != "admin" {
		t.Fatal("登录未提升指定邮箱")
	}
	s.AdminEmails = []string{first.User.Email}
	if err := s.BootstrapAdmins(ctx); err != nil {
		t.Fatal(err)
	}
	a.call("GET", "/api/admin/users", first.Token, "", 200)
	// 旧 token 在角色撤销后也必须立即失去权限。
	database.Model(&model.User{}).Where("id = ?", first.User.ID).Update("role", "user")
	a.call("GET", "/api/admin/users", first.Token, "", 403)
	s.AdminEmails = nil
	database.Model(&model.User{}).Where("role = 'admin'").Update("role", "user")
	if err := s.BootstrapAdmins(ctx); err != nil {
		t.Fatal(err)
	}
	me := decode[service.UserView](t, a.call("GET", "/api/me", first.Token, "", 200))
	if me.Role != "admin" {
		t.Fatal("启动时未选最早用户")
	}
}

func TestT11StorageOSSSettingsAndHotSwap(t *testing.T) {
	database := sourceDatabase(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var failProbe atomic.Bool
	ossServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failProbe.Load() || !strings.HasPrefix(r.URL.Path, "/demo-bucket") || r.URL.Query().Get("list-type") != "2" {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>denied</Message></Error>`))
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult><Name>demo-bucket</Name><Prefix>team-a/</Prefix><MaxKeys>1</MaxKeys><IsTruncated>false</IsTruncated></ListBucketResult>`))
	}))
	defer ossServer.Close()

	cfg := config.Config{
		DataDir:                t.TempDir(),
		ChatBaseURL:            "https://example.invalid/v1",
		ChatModel:              "fake",
		EmbedBaseURL:           "https://example.invalid/v1",
		EmbedModel:             "fake-embed",
		EmbeddingDim:           1536,
		JWTSecret:              strings.Repeat("s", 32),
		QuotaDailyChatRequests: 200,
		QuotaDailyTokens:       200000,
		LLMMaxConcurrency:      8,
	}.Resolved()
	rawStore, err := storage.NewConfigured(ctx, cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	files := storage.NewSwitchable(rawStore)
	defer files.Close()

	manager, err := settings.New(database, cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	manager.OnStorageChange = func(changeCtx context.Context, next config.Config) error {
		nextStore, err := storage.NewConfigured(changeCtx, next, logger)
		if err != nil {
			return err
		}
		return files.Replace(nextStore)
	}
	s := service.New(database, middleware.NewTokens(cfg.JWTSecret), cfg.ChatModel, logger)
	s.Provider = manager.Provider
	s.Usage = llm.NewGuard(database, manager.Provider, cfg)
	s.Admin = service.NewAdminService(ctx, database, manager)
	s.Files, s.Storage = files, files
	defer s.Admin.Close()

	a := &testAPI{t: t, router: handler.NewRouter(s, nil), database: database}
	admin := a.register()

	view := decode[settings.View](t, a.call("GET", "/api/admin/settings", admin.Token, "", 200))
	if len(view.Storage) != 7 || view.Storage[0].Key != "STORAGE_DRIVER" || view.Storage[0].Value != "local" {
		t.Fatalf("存储默认配置异常: %+v", view.Storage)
	}

	// 未填写凭据时测试存储连通性应返回可读失败信息
	emptyTest := decode[settings.TestResult](t, a.call("POST", "/api/admin/settings/test", admin.Token, `{"values":{},"capability":"storage"}`, 200))
	if emptyTest.OK {
		t.Fatal("缺少 OSS 凭据时不应通过测试")
	}

	ossPatch := map[string]string{
		"OSS_REGION":            "cn-hangzhou",
		"OSS_ENDPOINT":          ossServer.URL,
		"OSS_BUCKET":            "demo-bucket",
		"OSS_ACCESS_KEY_ID":     "LTAI5tSecretKeyId1234",
		"OSS_ACCESS_KEY_SECRET": "SecretKeyValue987654",
		"OSS_PREFIX":            "team-a/",
	}
	testBody, _ := json.Marshal(map[string]any{"values": ossPatch, "capability": "storage"})
	okTest := decode[settings.TestResult](t, a.call("POST", "/api/admin/settings/test", admin.Token, string(testBody), 200))
	if !okTest.OK {
		t.Fatalf("模拟 OSS 连通性测试失败: %+v", okTest)
	}

	// OSS 探活失败时拒绝切换并保存
	ossPatch["STORAGE_DRIVER"] = "oss"
	saveBody, _ := json.Marshal(map[string]any{"values": ossPatch})
	failProbe.Store(true)
	a.call("PATCH", "/api/admin/settings", admin.Token, string(saveBody), 400)
	if files.Driver() != "local" {
		t.Fatal("探活失败时不应切换存储驱动")
	}

	// 探活通过后保存到数据库、凭据加密且响应掩码，同时热切换为 oss
	failProbe.Store(false)
	saved := decode[settings.View](t, a.call("PATCH", "/api/admin/settings", admin.Token, string(saveBody), 200))
	if files.Driver() != "oss" || files.Key("uploads/u/s") != "team-a/uploads/u/s" {
		t.Fatalf("未热切换为 OSS: driver=%s key=%s", files.Driver(), files.Key("uploads/u/s"))
	}
	for _, item := range saved.Storage {
		if (item.Key == "OSS_ACCESS_KEY_ID" || item.Key == "OSS_ACCESS_KEY_SECRET") && (!strings.Contains(item.Value, "****") || strings.Contains(item.Value, "SecretKey")) {
			t.Fatalf("OSS 凭据未脱敏: %+v", item)
		}
	}
	for _, secretKey := range []string{"OSS_ACCESS_KEY_ID", "OSS_ACCESS_KEY_SECRET"} {
		var row model.SystemSetting
		if err := database.Where("key = ?", secretKey).First(&row).Error; err != nil || !row.IsSecret || strings.Contains(row.Value, "SecretKey") {
			t.Fatalf("OSS 密钥未加密存库 %s: err=%v row=%+v", secretKey, err, row)
		}
	}

	// 切换回 local 不受 OSS 探活阻塞并立即热切换回本地存储
	a.call("PATCH", "/api/admin/settings", admin.Token, `{"values":{"STORAGE_DRIVER":"local"}}`, 200)
	if files.Driver() != "local" {
		t.Fatalf("未热切换回本地存储: %s", files.Driver())
	}
}
