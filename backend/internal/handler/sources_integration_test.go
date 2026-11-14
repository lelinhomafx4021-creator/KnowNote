package handler_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/text/encoding/simplifiedchinese"
	"gorm.io/gorm"
	"knownote/internal/db"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/service/ingest"
	"knownote/internal/storage"
)

type noQueue struct{}

func (noQueue) Notify() {}

// 来源队列会恢复所有 processing 记录，因此测试使用独立 schema 隔离开发数据。
func sourceDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://knownote:knownote@localhost:5432/knownote?sslmode=disable"
	}
	admin, err := db.Open(ctx, dsn)
	if err != nil {
		t.Skip("DATABASE_URL 不可用，跳过来源接口集成测试")
	}
	adminSQL, _ := admin.DB()
	t.Cleanup(func() { adminSQL.Close() })
	schema := "t2_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		query := u.Query()
		query.Set("search_path", schema+",public")
		u.RawQuery = query.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema + ",public"
	}
	database, err := db.Open(ctx, dsn)
	if err != nil {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	t.Cleanup(func() { sqlDB.Close(); admin.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if err := db.Migrate(ctx, database, 1536); err != nil {
		t.Fatal(err)
	}
	return database
}

func sourceUpload(t *testing.T, a *testAPI, notebookID, token, name string, data []byte, status int) service.SourceView {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(data)
	writer.Close()
	request := httptest.NewRequest("POST", "/api/notebooks/"+notebookID+"/sources/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	a.router.ServeHTTP(response, request)
	if response.Code != status {
		t.Fatalf("上传 %s 返回 %d，预期 %d：%s", name, response.Code, status, response.Body.String())
	}
	var envelope struct {
		Code int                `json:"code"`
		Data service.SourceView `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if (status == 200) != (envelope.Code == 0) {
		t.Fatal("上传业务错误码不符")
	}
	return envelope.Data
}

func waitSource(t *testing.T, a *testAPI, token, id, status string) service.SourceView {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		item := decode[service.SourceView](t, a.call("GET", "/api/sources/"+id, token, "", 200))
		if item.Status == status {
			return item
		}
		if item.Status == "error" {
			t.Fatalf("来源处理失败：%s", item.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("来源处理未在规定时间内完成")
	return service.SourceView{}
}

type pipelineClient struct {
	llm.Client
	started chan struct{}
	release chan struct{}
	active  atomic.Int32
	maximum atomic.Int32
}

func (c *pipelineClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if strings.Contains(texts[0], "EMBED_ERROR") {
		return nil, errors.New("模拟向量化故障")
	}
	if strings.Contains(texts[0], "WAIT_TASK") {
		n := c.active.Add(1)
		defer c.active.Add(-1)
		for {
			old := c.maximum.Load()
			if old >= n || c.maximum.CompareAndSwap(old, n) {
				break
			}
		}
		c.started <- struct{}{}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.release:
		}
	}
	return c.Client.Embed(ctx, texts)
}

func (c *pipelineClient) Chat(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (string, error) {
	text := messages[len(messages)-1].Content
	if strings.Contains(text, "CHAT_ERROR") {
		return "", errors.New("模拟摘要故障")
	}
	if strings.Contains(text, "BAD_JSON") {
		return "无效 JSON", nil
	}
	return c.Client.Chat(ctx, messages, opts)
}

func TestSourcesIntegration(t *testing.T) {
	database := sourceDatabase(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	files, err := storage.New(context.Background(), t.TempDir(), logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { files.Close() })
	s := service.New(database, middleware.NewTokens(strings.Repeat("s", 32)), "fake", logger)
	s.Files, s.Storage, s.Ingest = files, files, noQueue{}
	gin.SetMode(gin.TestMode)
	a := &testAPI{t: t, router: handler.NewRouter(s, nil, handler.SourceRoutes(s)), database: database}
	owner, other := a.register(), a.register()
	notebook := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	path := "/api/notebooks/" + notebook.ID + "/sources"
	a.call("GET", path, "", "", 401)
	a.call("GET", path, other.Token, "", 404)
	a.call("POST", path, other.Token, `{"type":"text","text":"越权"}`, 404)
	sourceUpload(t, a, notebook.ID, other.Token, "a.md", []byte("越权"), 404)
	if string(a.call("GET", path, owner.Token, "", 200)) != "[]" {
		t.Fatal("空来源列表应返回数组")
	}
	for _, body := range []string{`{"type":"pdf"}`, `{"type":"text","text":"  "}`, `{"type":"web","url":"file:///x"}`, `{"type":"web","url":"http://127.0.0.1"}`, `{"type":"text","text":"a","unknown":1}`, `{"type":"text","text":"a","title":"\u0000"}`} {
		a.call("POST", path, owner.Token, body, 400)
	}
	sourceUpload(t, a, notebook.ID, owner.Token, "app.exe", []byte("其他"), 400)
	guide := model.NotebookGuide{NotebookOwner: model.NotebookOwner{UserID: owner.User.ID, NotebookID: notebook.ID}, Stale: true}
	if err := database.Create(&guide).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&guide).Update("stale", false).Error; err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("知识管理与向量检索用于整理文档。", 260)
	textBody := `{"type":"text","title":"完整文本","text":"` + text + `"}`
	textSource := decode[service.SourceView](t, a.call("POST", path, owner.Token, textBody, 200))
	markdown := []byte("# Markdown 来源\n\n资料切分与向量检索。\n\n第二段用于生成摘要。")
	mdSource := sourceUpload(t, a, notebook.ID, owner.Token, "资料.MD", markdown, 200)
	gbk, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文编码测试资料"))
	txtSource := sourceUpload(t, a, notebook.ID, owner.Token, "中文.txt", gbk, 200)
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	entry, _ := zipWriter.Create("word/document.xml")
	io.WriteString(entry, `<document><body><p><r><t>文档段落与来源解析。</t></r></p></body></document>`)
	zipWriter.Close()
	docSource := sourceUpload(t, a, notebook.ID, owner.Token, "文档.docx", archive.Bytes(), 200)
	audio := sourceUpload(t, a, notebook.ID, owner.Token, "音频.mp3", []byte("音频占位"), 200)
	video := decode[service.SourceView](t, a.call("POST", path, owner.Token, `{"type":"web","url":"https://www.bilibili.com/video/test"}`, 200))
	youtube := decode[service.SourceView](t, a.call("POST", path, owner.Token, `{"type":"youtube","url":"https://youtu.be/test"}`, 200))
	broken := sourceUpload(t, a, notebook.ID, owner.Token, "损坏.pdf", []byte("非 PDF"), 200)
	for _, source := range []service.SourceView{textSource, mdSource, txtSource, docSource, audio, video, youtube, broken} {
		if source.Status != "processing" {
			t.Fatal("创建应立即返回 processing")
		}
	}
	client := &pipelineClient{Client: llm.NewFake(1536), started: make(chan struct{}, 10), release: make(chan struct{})}
	queue := ingest.New(context.Background(), database, client, files, 1536, logger)
	t.Cleanup(queue.Close)
	s.Ingest = queue
	for _, source := range []service.SourceView{textSource, mdSource, txtSource, docSource, audio, video, youtube} {
		ready := waitSource(t, a, owner.Token, source.ID, "ready")
		if ready.Content == "" || ready.Summary == "" || len(ready.Topics) < 3 || len(ready.Topics) > 6 || ready.WordCount == 0 || ready.Progress != 100 {
			t.Fatal("来源完成字段缺失")
		}
	}
	waitSource(t, a, owner.Token, broken.ID, "error")
	full := waitSource(t, a, owner.Token, textSource.ID, "ready")
	if len([]rune(full.Content)) != 3000 || full.WordCount != len([]rune(text)) {
		t.Fatal("原文预览或完整字符数错误")
	}
	var stored model.Source
	if err := database.Where("user_id = ? AND id = ?", owner.User.ID, textSource.ID).First(&stored).Error; err != nil || stored.Content != text {
		t.Fatal("完整原文未保留")
	}
	var chunks []model.Chunk
	if err := database.Where("user_id = ? AND source_id = ?", owner.User.ID, textSource.ID).Find(&chunks).Error; err != nil || len(chunks) < 2 {
		t.Fatal("未生成多个分块")
	}
	for _, chunk := range chunks {
		if len(chunk.Embedding.Slice()) != 1536 {
			t.Fatal("向量维度错误")
		}
	}
	if err := database.Where("user_id = ? AND id = ?", owner.User.ID, guide.ID).First(&guide).Error; err != nil || !guide.Stale {
		t.Fatal("来源变化没有使概要失效")
	}
	raw := string(a.call("GET", "/api/sources/"+mdSource.ID, owner.Token, "", 200))
	for _, field := range []string{"filePath", "userId", "updatedAt"} {
		if strings.Contains(raw, `"`+field+`"`) {
			t.Fatal("来源响应暴露内部字段")
		}
	}
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		a.call(method, "/api/sources/"+mdSource.ID, other.Token, `{"title":"越权"}`, 404)
	}
	renamed := decode[service.SourceView](t, a.call("PATCH", "/api/sources/"+mdSource.ID, owner.Token, `{"title":"新标题"}`, 200))
	if renamed.Title != "新标题" {
		t.Fatal("重命名失败")
	}
	a.call("PATCH", "/api/sources/"+mdSource.ID, owner.Token, `{"title":" "}`, 400)
	expectedSize := int64(len(markdown) + len(gbk) + archive.Len() + len("音频占位") + len("非 PDF"))
	settings := decode[service.SettingsView](t, a.call("GET", "/api/me/settings", owner.Token, "", 200))
	if settings.StorageUsed != expectedSize {
		t.Fatalf("存储用量不符：%d != %d", settings.StorageUsed, expectedSize)
	}
	cleaned := decode[service.SourceView](t, a.call("POST", path, owner.Token, `{"type":"text","text":"\u0000清洗\t  文本\r\n\n\n下一段\u0001"}`, 200))
	if ready := waitSource(t, a, owner.Token, cleaned.ID, "ready"); ready.Content != "清洗 文本\n\n下一段" {
		t.Fatal("文本来源没有清理控制字符与多余空白")
	}
	// 摘要格式失败可降级；模型调用失败则进入 error。
	for _, marker := range []string{"BAD_JSON", "EMBED_ERROR", "CHAT_ERROR"} {
		item := decode[service.SourceView](t, a.call("POST", path, owner.Token, `{"type":"text","text":"`+marker+` 流水线测试"}`, 200))
		status := "error"
		if marker == "BAD_JSON" {
			status = "ready"
		}
		waitSource(t, a, owner.Token, item.ID, status)
	}
	// 同时提交五个任务，只允许三个进入向量化；删除正在处理的来源不得重建分块。
	var waiting []service.SourceView
	for i := 0; i < 5; i++ {
		waiting = append(waiting, decode[service.SourceView](t, a.call("POST", path, owner.Token, `{"type":"text","text":"WAIT_TASK 并发测试"}`, 200)))
	}
	for i := 0; i < 3; i++ {
		select {
		case <-client.started:
		case <-time.After(5 * time.Second):
			t.Fatal("worker 未开始处理")
		}
	}
	if client.maximum.Load() != 3 {
		t.Fatal("并发上限不正确")
	}
	a.call("DELETE", "/api/sources/"+waiting[0].ID, owner.Token, "", 200)
	close(client.release)
	for _, item := range waiting[1:] {
		waitSource(t, a, owner.Token, item.ID, "ready")
	}
	if client.maximum.Load() > 3 {
		t.Fatal("超过并发上限")
	}
	a.call("GET", "/api/sources/"+waiting[0].ID, owner.Token, "", 404)
	var count int64
	if err := database.Model(&model.Chunk{}).Where("user_id = ? AND source_id = ?", owner.User.ID, waiting[0].ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("删除期间重建了分块")
	}
	var mdStored model.Source
	if err := database.Where("user_id = ? AND id = ?", owner.User.ID, mdSource.ID).First(&mdStored).Error; err != nil {
		t.Fatal(err)
	}
	a.call("DELETE", "/api/sources/"+mdSource.ID, owner.Token, "", 200)
	if _, err := files.Open(context.Background(), mdStored.FilePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("删除来源没有清理文件")
	}
	if err := database.Model(&model.Chunk{}).Where("user_id = ? AND source_id = ?", owner.User.ID, mdSource.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("来源分块未级联删除")
	}
	// 关闭时保留未完成任务，下一次启动恢复并替换旧分块。
	queue.Close()
	restartClient := &pipelineClient{Client: llm.NewFake(1536), started: make(chan struct{}, 1), release: make(chan struct{})}
	pausedQueue := ingest.New(context.Background(), database, restartClient, files, 1536, logger)
	s.Ingest = pausedQueue
	resumed := decode[service.SourceView](t, a.call("POST", path, owner.Token, `{"type":"text","text":"WAIT_TASK 重启恢复"}`, 200))
	select {
	case <-restartClient.started:
	case <-time.After(5 * time.Second):
		pausedQueue.Close()
		t.Fatal("恢复测试未开始")
	}
	pausedQueue.Close()
	paused := decode[service.SourceView](t, a.call("GET", "/api/sources/"+resumed.ID, owner.Token, "", 200))
	if paused.Status != "processing" {
		t.Fatal("关闭时应保留 processing 状态")
	}
	recoveredQueue := ingest.New(context.Background(), database, llm.NewFake(1536), files, 1536, logger)
	t.Cleanup(recoveredQueue.Close)
	s.Ingest = recoveredQueue
	waitSource(t, a, owner.Token, resumed.ID, "ready")
	a.call("DELETE", "/api/notebooks/"+notebook.ID, owner.Token, "", 200)
	if _, err := files.Open(context.Background(), filepath.Join("uploads", owner.User.ID, txtSource.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("笔记本删除后没有清理来源目录")
	}
	settings = decode[service.SettingsView](t, a.call("GET", "/api/me/settings", owner.Token, "", 200))
	if settings.StorageUsed != 0 {
		t.Fatal("删除笔记本后存储统计未归零")
	}
}
