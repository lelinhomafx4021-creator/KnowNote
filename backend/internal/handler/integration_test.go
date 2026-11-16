package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"knownote/internal/db"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
)

type testAPI struct {
	t        *testing.T
	router   http.Handler
	database *gorm.DB
}

func (a *testAPI) call(method, path, token, body string, status int) json.RawMessage {
	a.t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	writer := httptest.NewRecorder()
	a.router.ServeHTTP(writer, request)
	if writer.Code != status {
		a.t.Fatalf("%s %s: 状态码 %d，预期 %d", method, path, writer.Code, status)
	}
	var envelope struct {
		Code    int             `json:"code"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(writer.Body.Bytes(), &envelope); err != nil {
		a.t.Fatal(err)
	}
	if (status == 200 && envelope.Code != 0) || (status != 200 && envelope.Code == 0) {
		a.t.Fatal("业务错误码与 HTTP 状态不匹配")
	}
	return envelope.Data
}

func decode[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var data T
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func (a *testAPI) register() service.AuthResult {
	a.t.Helper()
	email := "t1-" + uuid.NewString() + "@example.com"
	input := `{"email":"` + email + `","password":"test-password","nickname":"集成测试"}`
	data := decode[service.AuthResult](a.t, a.call("POST", "/api/auth/register", "", input, 200))
	a.t.Cleanup(func() {
		if err := a.database.Where("id = ?", data.User.ID).Delete(&model.User{}).Error; err != nil {
			a.t.Error("清理测试用户失败")
		}
	})
	if data.User.Email != email || data.Token == "" || data.User.Nickname != "集成测试" {
		a.t.Fatal("注册响应错误")
	}
	return data
}

func TestAuthAndNotebookIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://knownote:knownote@localhost:5432/knownote?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	database, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Skip("DATABASE_URL 不可用，跳过真实数据库集成测试")
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal("获取测试连接池失败")
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	// 数据库可连接后，迁移失败必须报告失败，不能误报为跳过。
	if err := db.Migrate(ctx, database, 1536); err != nil {
		t.Fatal(err)
	}
	// 第二次迁移验证服务重启时不会反复改表或丢失向量维度。
	if err := db.Migrate(ctx, database, 1536); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, database, 512); err == nil {
		t.Fatal("不兼容维度应明确拒绝启动")
	}
	var vectorType string
	if err := database.Raw(`SELECT format_type(a.atttypid, a.atttypmod) FROM pg_attribute a WHERE a.attrelid = 'chunks'::regclass AND a.attname = 'embedding'`).Scan(&vectorType).Error; err != nil || vectorType != "vector(1536)" {
		t.Fatal("数据库向量列维度错误")
	}
	gin.SetMode(gin.TestMode)
	s := service.New(database, middleware.NewTokens(strings.Repeat("i", 32)), "gpt-4o-mini", slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.Models.Chat = append(s.Models.Chat, llm.ChatModel{ID: "custom-chat", Name: "自定义模型"})
	a := &testAPI{t: t, router: handler.NewRouter(s, []string{"http://localhost:5173"}), database: database}
	health := decode[map[string]string](t, a.call("GET", "/api/health", "", "", 200))
	if health["db"] != "ok" {
		t.Fatal("健康检查没有检测数据库")
	}
	a.call("GET", "/api/me", "", "", 401)
	a.call("POST", "/api/auth/register", "", `{"email":"invalid","password":"123","nickname":"测试"}`, 400)
	owner, other := a.register(), a.register()
	a.call("POST", "/api/auth/register", "", `{"email":"`+strings.ToUpper(owner.User.Email)+`","password":"test-password","nickname":"重复邮箱"}`, 409)
	a.call("POST", "/api/auth/login", "", `{"email":"`+owner.User.Email+`","password":"wrong"}`, 401)
	login := decode[service.AuthResult](t, a.call("POST", "/api/auth/login", "", `{"email":"`+owner.User.Email+`","password":"test-password"}`, 200))
	if login.User.ID != owner.User.ID {
		t.Fatal("登录用户错误")
	}
	me := decode[service.UserView](t, a.call("GET", "/api/me", login.Token, "", 200))
	if me.ID != owner.User.ID {
		t.Fatal("当前用户错误")
	}
	var stored model.User
	if err := database.Where("id = ?", owner.User.ID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.PasswordHash == "test-password" || bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("test-password")) != nil {
		t.Fatal("密码未使用 bcrypt 存储")
	}
	settings := decode[service.SettingsView](t, a.call("GET", "/api/me/settings", owner.Token, "", 200))
	expectedModel := ""
	if owner.User.Role == "admin" {
		expectedModel = "gpt-4o-mini"
	}
	if settings.Model != expectedModel || settings.AnswerStyle != "concise" || settings.Language != "zh-CN" || settings.StorageUsed != 0 || settings.StorageTotal != service.StorageTotal {
		t.Fatal("默认设置错误")
	}
	if owner.User.Role == "admin" {
		settings = decode[service.SettingsView](t, a.call("PATCH", "/api/me/settings", owner.Token, `{"model":"custom-chat","answerStyle":"learning","language":"en-US","storageUsed":999,"storageTotal":1}`, 200))
		if settings.Model != "custom-chat" || settings.AnswerStyle != "learning" || settings.Language != "en-US" || settings.StorageUsed != 0 || settings.StorageTotal != service.StorageTotal {
			t.Fatal("设置更新或只读字段处理错误")
		}
		a.call("PATCH", "/api/me/settings", other.Token, `{"model":"custom-chat"}`, 403)
	} else {
		a.call("PATCH", "/api/me/settings", owner.Token, `{"model":"custom-chat"}`, 403)
		settings = decode[service.SettingsView](t, a.call("PATCH", "/api/me/settings", owner.Token, `{"answerStyle":"learning","language":"en-US","storageUsed":999,"storageTotal":1}`, 200))
		if settings.Model != "" || settings.AnswerStyle != "learning" || settings.Language != "en-US" || settings.StorageUsed != 0 || settings.StorageTotal != service.StorageTotal {
			t.Fatal("设置更新或只读字段处理错误")
		}
	}
	a.call("PATCH", "/api/me/settings", owner.Token, `{"answerStyle":"invalid"}`, 400)
	if string(a.call("GET", "/api/notebooks", owner.Token, "", 200)) != "[]" {
		t.Fatal("空列表应返回数组")
	}
	notebook := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	if notebook.Title != "未命名笔记本" || notebook.Color < 0 || notebook.Color > 7 || notebook.Emoji == "" || notebook.SourceCount != 0 {
		t.Fatal("默认笔记本错误")
	}
	second := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{"title":"第二本"}`, 200))
	foreign := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", other.Token, `{"title":"其他用户"}`, 200))
	path := "/api/notebooks/" + notebook.ID
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		body := ""
		if method == "PATCH" {
			body = `{"title":"越权"}`
		}
		a.call(method, path, other.Token, body, 404)
	}
	a.call("GET", "/api/notebooks/"+uuid.NewString(), owner.Token, "", 404)
	a.call("GET", "/api/notebooks/invalid", owner.Token, "", 400)
	for _, body := range []string{`{"color":8}`, `{"title":"  "}`, `{"unknown":1}`, `null`, `{} {}`, `{"color":"red"}`} {
		a.call("PATCH", path, owner.Token, body, 400)
	}
	seedNotebookData(t, database, owner.User.ID, notebook.ID)
	foreignSource := model.Source{NotebookOwner: model.NotebookOwner{UserID: other.User.ID, NotebookID: foreign.ID}, Type: "text", Title: "他人来源", Size: 9876}
	if err := database.Create(&foreignSource).Error; err != nil {
		t.Fatal(err)
	}
	invalidSource := model.Source{NotebookOwner: model.NotebookOwner{UserID: other.User.ID, NotebookID: notebook.ID}, Type: "text", Title: "归属不匹配"}
	if err := database.Create(&invalidSource).Error; err == nil {
		t.Fatal("数据库应拒绝跨用户子资源")
	}
	item := decode[service.NotebookView](t, a.call("GET", path, owner.Token, "", 200))
	if item.SourceCount != 1 {
		t.Fatal("来源数量未实时统计")
	}
	settings = decode[service.SettingsView](t, a.call("GET", "/api/me/settings", owner.Token, "", 200))
	if settings.StorageUsed != 1234 {
		t.Fatal("存储用量未隔离用户并实时求和")
	}
	item = decode[service.NotebookView](t, a.call("PATCH", path, owner.Token, `{"title":"更新标题","emoji":"🌱","color":0,"description":""}`, 200))
	if item.Title != "更新标题" || item.Emoji != "🌱" || item.Color != 0 || item.Description != "" || !item.UpdatedAt.After(notebook.UpdatedAt) {
		t.Fatal("笔记本更新错误")
	}
	items := decode[[]service.NotebookView](t, a.call("GET", "/api/notebooks", owner.Token, "", 200))
	if len(items) != 2 || items[0].ID != notebook.ID || items[1].ID != second.ID || items[0].SourceCount != 1 {
		t.Fatal("笔记本列表未隔离或排序错误")
	}
	if string(a.call("DELETE", path, owner.Token, "", 200)) != "null" {
		t.Fatal("删除应返回 null")
	}
	a.call("GET", path, owner.Token, "", 404)
	a.call("DELETE", path, owner.Token, "", 404)
	for _, table := range []string{"sources", "chunks", "messages", "notes", "artifacts", "notebook_guides"} {
		var count int64
		if err := database.Table(table).Where("user_id = ? AND notebook_id = ?", owner.User.ID, notebook.ID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s 未级联删除", table)
		}
	}
	settings = decode[service.SettingsView](t, a.call("GET", "/api/me/settings", owner.Token, "", 200))
	if settings.StorageUsed != 0 {
		t.Fatal("删除后存储用量未更新")
	}
	a.call("GET", "/api/notebooks/"+foreign.ID, other.Token, "", 200)
}

func seedNotebookData(t *testing.T, database *gorm.DB, userID, notebookID string) {
	t.Helper()
	owner := model.NotebookOwner{UserID: userID, NotebookID: notebookID}
	source := model.Source{NotebookOwner: owner, Type: "text", Title: "测试来源", Status: "ready", Size: 1234, Topics: []string{"测试"}}
	if err := database.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	vector := make([]float32, 1536)
	vector[0] = 1
	for _, entry := range []any{
		&model.Chunk{NotebookOwner: owner, SourceID: source.ID, Content: "测试正文", Embedding: model.NewVector(vector)},
		&model.Message{NotebookOwner: owner, Role: "assistant", Content: "回答 [1]", Citations: []model.Citation{{Index: 1, SourceID: source.ID, SourceTitle: source.Title, Quote: "测试正文"}}},
		&model.Note{NotebookOwner: owner, Title: "测试笔记", Content: "正文", Origin: "manual"},
		&model.Artifact{NotebookOwner: owner, Kind: "mindmap", Title: "测试导图", Mindmap: &model.MindMapNode{ID: "root", Label: "根节点"}},
		&model.NotebookGuide{NotebookOwner: owner, Summary: "测试概要", SuggestedQuestions: []string{"问题一？"}},
	} {
		if err := database.Create(entry).Error; err != nil {
			t.Fatal(err)
		}
	}
	var chunk model.Chunk
	if err := database.Where("user_id = ? AND source_id = ?", userID, source.ID).First(&chunk).Error; err != nil || len(chunk.Embedding.Slice()) != 1536 {
		t.Fatal("pgvector 向量读写失败")
	}
	var message model.Message
	if err := database.Where("user_id = ? AND notebook_id = ?", userID, notebookID).First(&message).Error; err != nil || len(message.Citations) != 1 || message.Citations[0].SourceID != source.ID {
		t.Fatal("引用 JSON 读写失败")
	}
}
