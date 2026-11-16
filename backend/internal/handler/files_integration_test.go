package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/service/chat"
	"knownote/internal/service/ingest"
	"knownote/internal/service/studio"
	"knownote/internal/storage"
	"knownote/internal/testfixture"
)

type redirectStore struct {
	storage.Store
	ttl   time.Duration
	key   string
	calls int
}

func (s *redirectStore) Driver() string { return "oss" }
func (s *redirectStore) SignedURL(_ context.Context, key string, ttl time.Duration) (string, error) {
	s.ttl = ttl
	s.key = key
	s.calls++
	return "https://example.com/signed-object", nil
}
func TestSourceFilesAndPageIntegration(t *testing.T) {
	database := sourceDatabase(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	files, err := storage.New(context.Background(), t.TempDir(), logger)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	client := llm.NewFake(1536)
	queue := ingest.New(context.Background(), database, client, files, 1536, logger)
	defer queue.Close()
	core := service.New(database, middleware.NewTokens(strings.Repeat("f", 32)), "fake", logger)
	core.Storage, core.Files, core.Ingest = files, files, queue
	workshop := studio.New(database, client, logger)
	gin.SetMode(gin.TestMode)
	a := &testAPI{t: t, database: database, router: handler.NewRouter(core, nil, handler.SourceRoutes(core), handler.ChatRoutes(core, chat.New(database, client, logger)), handler.StudioRoutes(core, workshop))}
	owner, other := a.register(), a.register()
	book := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	source := sourceUpload(t, a, book.ID, owner.Token, "资料.pdf", testfixture.PDF(), 200)
	source = waitSource(t, a, owner.Token, source.ID, "ready")
	if source.PageCount != 3 || len(source.Sections) != 3 || !source.Previewable || source.OriginalExt != "pdf" {
		t.Fatalf("来源位置元数据丢失：%+v", source)
	}
	var chunks []model.Chunk
	if err := database.Where("user_id = ? AND source_id = ?", owner.User.ID, source.ID).Order("chunk_index").Find(&chunks).Error; err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 || chunks[2].Page == nil || *chunks[2].Page != 3 || chunks[2].LocationLabel != "第 3 页" {
		t.Fatal("分块位置未入库")
	}
	for _, endpoint := range []string{"preview", "file"} {
		for _, scenario := range []struct {
			token, rangeHeader string
			status             int
		}{{owner.Token, "", 200}, {owner.Token, "bytes=0-4", 206}, {other.Token, "", 404}, {"", "", 401}} {
			request := httptest.NewRequest("GET", "/api/sources/"+source.ID+"/"+endpoint, nil)
			if scenario.token != "" {
				request.Header.Set("Authorization", "Bearer "+scenario.token)
			}
			request.Header.Set("Range", scenario.rangeHeader)
			w := httptest.NewRecorder()
			a.router.ServeHTTP(w, request)
			if w.Code != scenario.status {
				t.Fatalf("%s 状态错误 %d", endpoint, w.Code)
			}
			if w.Code == 206 && (w.Body.String() != "%PDF-" || !strings.Contains(w.Header().Get("Content-Range"), "bytes 0-4/")) {
				t.Fatal("Range 内容或响应头错误")
			}
			if w.Code == 200 {
				disposition := "attachment"
				if endpoint == "preview" {
					disposition = "inline"
				}
				if !strings.HasPrefix(w.Header().Get("Content-Disposition"), disposition) || w.Header().Get("Content-Type") != "application/pdf" {
					t.Fatal("文件响应头错误")
				}
			}
		}
	}
	request := httptest.NewRequest("GET", "/api/sources/"+source.ID+"/preview?token="+owner.Token, nil)
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, request)
	if w.Code != 200 {
		t.Fatal("query token 未生效")
	}
	request.Header.Set("Authorization", "Bearer invalid")
	w = httptest.NewRecorder()
	a.router.ServeHTTP(w, request)
	if w.Code != 401 {
		t.Fatal("错误 Authorization 不得降级 query token")
	}
	body, _ := json.Marshal(map[string]any{"question": "知识管理与资料检索是什么？", "sourceIds": []string{source.ID}})
	events := streamChat(t, a, "/api/notebooks/"+book.ID, owner.Token, string(body))
	if len(events[0].Citations) == 0 {
		t.Fatal("没有返回引用")
	}
	for _, citation := range events[0].Citations {
		if citation.Page == nil || citation.LocationLabel == "" {
			t.Fatal("SSE 引用没有位置")
		}
	}
	history := decode[[]model.Message](t, a.call("GET", "/api/notebooks/"+book.ID+"/messages", owner.Token, "", 200))
	if len(history) != 2 || len(history[1].Citations) == 0 || history[1].Citations[0].Page == nil {
		t.Fatal("引用页码未落库")
	}
	// 无 LibreOffice 时新版文档解析成功，预览独立降级。
	t.Setenv("SOFFICE_PATH", t.TempDir()+"/missing")
	docx := sourceUpload(t, a, book.ID, owner.Token, "文档.docx", testfixture.DOCX(), 200)
	docx = waitSource(t, a, owner.Token, docx.ID, "ready")
	if docx.Previewable || len(docx.Sections) != 2 {
		t.Fatal("无 Office 的新版解析或预览降级不符")
	}
	a.call("GET", "/api/sources/"+docx.ID+"/preview", owner.Token, "", 404)
	legacy := sourceUpload(t, a, book.ID, owner.Token, "旧版.doc", []byte("旧版"), 200)
	legacy = waitSource(t, a, owner.Token, legacy.ID, "error")
	if legacy.Type != "docx" || legacy.OriginalExt != "doc" || legacy.Error != "需要安装 LibreOffice 才能解析旧版 Office 文件" {
		t.Fatal("旧版格式错误未按契约返回")
	}
	redirect := &redirectStore{Store: files}
	core.Storage = redirect
	for _, path := range []string{"/api/sources/" + source.ID + "/preview", "/api/sources/" + source.ID + "/file"} {
		request := httptest.NewRequest("GET", path+"?token="+owner.Token, nil)
		w := httptest.NewRecorder()
		a.router.ServeHTTP(w, request)
		if w.Code != 302 || w.Header().Get("Location") != "https://example.com/signed-object" || redirect.ttl != 10*time.Minute {
			t.Fatal("OSS 跳转或有效期错误")
		}
	}
	before := redirect.calls
	a.call("GET", "/api/sources/"+source.ID+"/file", other.Token, "", 404)
	if redirect.calls != before {
		t.Fatal("越权请求触发了签名")
	}
	a.call("DELETE", "/api/sources/"+source.ID, owner.Token, "", 200)
	if _, err := files.Open(context.Background(), storage.OriginalKey(files, owner.User.ID, source.ID, "pdf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("删除后原件未清理")
	}
	a.call("DELETE", "/api/notebooks/"+book.ID, owner.Token, "", 200)
}
