package handler_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"knownote/internal/config"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/service/studio"
	"knownote/internal/storage"
)

type studioClient struct {
	llm.Client
	mode    atomic.Int32
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	mu      sync.Mutex
	voices  []string
	audio   [][]byte
	input   string
}

func (c *studioClient) Name() string { return "测试模型" }
func (c *studioClient) Chat(ctx context.Context, messages []llm.Message, opts llm.ChatOptions) (string, error) {
	if strings.HasPrefix(messages[0].Content, llm.StudioPrompt) {
		c.mu.Lock()
		c.input = messages[1].Content
		c.mu.Unlock()
	}
	if strings.HasPrefix(messages[0].Content, llm.StudioPrompt+"mindmap") {
		n := c.calls.Add(1)
		if c.mode.Load() == 1 && (n%2) == 1 {
			return "```json\n{\"label\":\"错误", nil
		}
		if c.mode.Load() == 2 {
			return `{"label":"","children":[]}`, nil
		}
	}
	if strings.HasPrefix(messages[0].Content, llm.AudioPrompt) && c.mode.Load() == 3 {
		select {
		case c.started <- struct{}{}:
		default:
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-c.release:
		}
	}
	return c.Client.Chat(ctx, messages, opts)
}
func (c *studioClient) TTS(ctx context.Context, text, voice string) ([]byte, error) {
	if c.mode.Load() == 4 {
		return nil, errors.New("模拟语音服务错误")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data := []byte(voice + ":" + text)
	c.mu.Lock()
	c.voices = append(c.voices, voice)
	c.audio = append(c.audio, data)
	c.mu.Unlock()
	return data, nil
}

func studioAPI(t *testing.T, tts bool) (*testAPI, *studio.Service, *studioClient, *storage.Local) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database := sourceDatabase(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	files, err := storage.New(context.Background(), t.TempDir(), logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { files.Close() })
	core := service.New(database, middleware.NewTokens(strings.Repeat("s", 32)), "fake-model", logger)
	core.Files, core.Storage = files, files
	client := &studioClient{Client: llm.NewFake(1536), started: make(chan struct{}, 1), release: make(chan struct{})}
	cfg := config.Config{LLMTTSVoices: []string{"知音", "页音"}}
	if tts {
		cfg.LLMTTSModel = "测试语音"
	}
	workshop := studio.New(context.Background(), database, client, files, cfg, logger)
	t.Cleanup(workshop.Close)
	a := &testAPI{t: t, database: database, router: handler.NewRouter(core, nil, handler.StudioRoutes(core, workshop))}
	return a, workshop, client, files
}

func addStudioSource(t *testing.T, a *testAPI, owner service.AuthResult, book service.NotebookView) {
	t.Helper()
	key := model.NotebookOwner{UserID: owner.User.ID, NotebookID: book.ID}
	source := model.Source{NotebookOwner: key, Type: "text", Title: "资料甲", Status: "ready", Summary: "资料介绍机器学习中的训练集、验证集与测试集划分方法，通过独立验证评估模型泛化能力，避免数据泄漏。", Topics: []string{"独立验证", "泛化能力"}}
	if err := a.database.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	texts := []string{"无关的开篇说明。", "独立验证衡量泛化能力。主题代表片段。", "独立验证要求分离数据集，泛化能力依赖合理评估。", "泛化能力的验证实验需要记录设置。", "结尾附录。"}
	vectors, err := llm.NewFake(1536).Embed(context.Background(), texts)
	if err != nil {
		t.Fatal(err)
	}
	for i, text := range texts {
		chunk := model.Chunk{NotebookOwner: key, SourceID: source.ID, ChunkIndex: i, Content: text, Embedding: model.NewVector(vectors[i])}
		if err := a.database.Create(&chunk).Error; err != nil {
			t.Fatal(err)
		}
	}
	bad := model.Source{NotebookOwner: key, Type: "text", Title: "不可见来源", Status: "processing", Summary: "秘密未就绪摘要"}
	if err := a.database.Create(&bad).Error; err != nil {
		t.Fatal(err)
	}
}

func TestNotesAndArtifactsIntegration(t *testing.T) {
	a, _, client, _ := studioAPI(t, false)
	owner, other := a.register(), a.register()
	book := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	path := "/api/notebooks/" + book.ID
	for _, suffix := range []string{"/notes", "/artifacts"} {
		if string(a.call("GET", path+suffix, owner.Token, "", 200)) != "[]" {
			t.Fatal("空列表必须返回数组")
		}
		a.call("GET", path+suffix, other.Token, "", 404)
	}
	a.call("POST", path+"/artifacts", owner.Token, `{"kind":"study-guide"}`, 400)
	for _, body := range []string{`{"title":"笔记","content":"内容","origin":"bad"}`, `{"title":"笔记","content":"内容"}`, `{"title":" ","content":"内容","origin":"manual"}`, `{"title":"笔记","content":null,"origin":"manual"}`} {
		a.call("POST", path+"/notes", owner.Token, body, 400)
	}
	var notes []model.Note
	for _, origin := range []string{"manual", "chat", "studio"} {
		notes = append(notes, decode[model.Note](t, a.call("POST", path+"/notes", owner.Token, `{"title":"笔记","content":"原文","origin":"`+origin+`"}`, 200)))
	}
	updated := decode[model.Note](t, a.call("PATCH", "/api/notes/"+notes[0].ID, owner.Token, `{"title":"修改标题","content":""}`, 200))
	if updated.Content != "" || updated.Title != "修改标题" || updated.Origin != "manual" || !updated.UpdatedAt.After(notes[0].UpdatedAt) {
		t.Fatal("部分更新未正确持久化")
	}
	list := decode[[]model.Note](t, a.call("GET", path+"/notes", owner.Token, "", 200))
	if list[0].ID != updated.ID {
		t.Fatal("未按更新时间倒序")
	}
	a.call("POST", path+"/notes", other.Token, `{"title":"笔记","content":"","origin":"manual"}`, 404)
	for _, method := range []string{"PATCH", "DELETE"} {
		a.call(method, "/api/notes/"+notes[0].ID, other.Token, `{}`, 404)
	}
	a.call("PATCH", "/api/notes/"+notes[0].ID, owner.Token, `{"origin":"studio"}`, 400)
	a.call("DELETE", "/api/notes/"+notes[0].ID, owner.Token, "", 200)
	a.call("PATCH", "/api/notes/"+notes[0].ID, owner.Token, `{}`, 404)
	addStudioSource(t, a, owner, book)
	var artifacts []model.Artifact
	for _, kind := range []string{"study-guide", "briefing", "faq", "timeline", "mindmap"} {
		item := decode[model.Artifact](t, a.call("POST", path+"/artifacts", owner.Token, `{"kind":"`+kind+`"}`, 200))
		artifacts = append(artifacts, item)
		if item.Title != llm.StudioTitles[kind] || item.NotebookID != book.ID {
			t.Fatal("产物标题或归属错误")
		}
		if kind == "mindmap" {
			if item.Mindmap == nil || item.Mindmap.ID == "" {
				t.Fatal("思维导图未持久化")
			}
		} else if item.Content == "" {
			t.Fatal("产物内容为空")
		}
	}
	client.mu.Lock()
	input := client.input
	client.mu.Unlock()
	if strings.Contains(input, "秘密未就绪摘要") || strings.Contains(input, "开篇说明") || !strings.Contains(input, "主题代表片段") {
		t.Fatal("来源状态或代表片段筛选错误")
	}
	items := decode[[]model.Artifact](t, a.call("GET", path+"/artifacts", owner.Token, "", 200))
	if len(items) != 5 || items[0].ID != artifacts[4].ID || items[0].Mindmap == nil {
		t.Fatal("产物倒序列表或JSONB错误")
	}
	a.call("POST", path+"/artifacts", owner.Token, `{"kind":"unknown"}`, 400)
	a.call("POST", path+"/artifacts", other.Token, `{"kind":"faq"}`, 404)
	a.call("DELETE", "/api/artifacts/"+items[0].ID, other.Token, "", 404)
	a.call("DELETE", "/api/artifacts/"+items[0].ID, owner.Token, "", 200)
	client.calls.Store(0)
	client.mode.Store(1)
	a.call("POST", path+"/artifacts", owner.Token, `{"kind":"mindmap"}`, 200)
	if client.calls.Load() != 2 {
		t.Fatal("格式失败未重试一次")
	}
	client.calls.Store(0)
	client.mode.Store(2)
	a.call("POST", path+"/artifacts", owner.Token, `{"kind":"mindmap"}`, 502)
	if client.calls.Load() != 2 {
		t.Fatal("无效格式重试次数错误")
	}
	var count int64
	if err := a.database.Model(&model.Artifact{}).Where("user_id = ? AND notebook_id = ?", owner.User.ID, book.ID).Count(&count).Error; err != nil || count != 5 {
		t.Fatal("失败产物不应入库")
	}
}

func awaitAudio(t *testing.T, a *testAPI, path, token, status string) studio.AudioView {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		item := decode[studio.AudioView](t, a.call("GET", path+"/audio", token, "", 200))
		if item.Status == status {
			return item
		}
		if item.Status == "error" && status != "error" {
			t.Fatal("音频生成失败")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("音频生成未及时完成")
	return studio.AudioView{}
}

func TestAudioIntegration(t *testing.T) {
	a, workshop, client, files := studioAPI(t, true)
	owner, other := a.register(), a.register()
	book := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{"title":"机器学习"}`, 200))
	path := "/api/notebooks/" + book.ID
	if item := decode[studio.AudioView](t, a.call("GET", path+"/audio", owner.Token, "", 200)); item.Status != "idle" {
		t.Fatal("未生成时状态错误")
	}
	a.call("POST", path+"/audio", owner.Token, "", 400)
	addStudioSource(t, a, owner, book)
	client.mode.Store(3)
	item := decode[studio.AudioView](t, a.call("POST", path+"/audio", owner.Token, "", 200))
	if item.Status != "generating" {
		t.Fatal("POST未立即返回generating")
	}
	select {
	case <-client.started:
	case <-time.After(3 * time.Second):
		t.Fatal("后台任务未启动")
	}
	second := decode[studio.AudioView](t, a.call("POST", path+"/audio", owner.Token, "", 200))
	if second.Status != "generating" {
		t.Fatal("并发请求应合并")
	}
	close(client.release)
	ready := awaitAudio(t, a, path, owner.Token, "ready")
	if ready.Transcript == "" || ready.URL != "/api/files/audio/"+book.ID+".mp3" || ready.DurationSec <= 0 || ready.Title != "深度对谈：机器学习" {
		t.Fatal("音频概览字段错误")
	}
	client.mu.Lock()
	voices := append([]string(nil), client.voices...)
	expected := bytes.Join(client.audio, nil)
	client.mu.Unlock()
	if len(voices) < 2 {
		t.Fatal("未逐段调用语音合成")
	}
	for i, voice := range voices {
		want := "知音"
		if i%2 == 1 {
			want = "页音"
		}
		if voice != want {
			t.Fatal("主持人与音色不匹配")
		}
	}
	request := httptest.NewRequest("GET", ready.URL+"?token="+owner.Token, nil)
	request.Header.Set("Range", "bytes=2-11")
	writer := httptest.NewRecorder()
	a.router.ServeHTTP(writer, request)
	if writer.Code != 206 || writer.Header().Get("Accept-Ranges") != "bytes" || writer.Header().Get("Content-Type") != "audio/mpeg" || !bytes.Equal(writer.Body.Bytes(), expected[2:12]) {
		t.Fatal("Range播放响应错误")
	}
	request = httptest.NewRequest("GET", ready.URL+"?token="+owner.Token, nil)
	writer = httptest.NewRecorder()
	a.router.ServeHTTP(writer, request)
	if writer.Code != 200 || !bytes.Equal(writer.Body.Bytes(), expected) {
		t.Fatal("MP3片段拼接顺序错误")
	}
	for _, suffix := range []string{"/audio"} {
		a.call("GET", path+suffix, other.Token, "", 404)
		a.call("POST", path+suffix, other.Token, "", 404)
	}
	a.call("GET", ready.URL, other.Token, "", 404)
	a.call("GET", ready.URL, "", "", 401)
	a.call("GET", "/api/files/audio/invalid.mp3", owner.Token, "", 404)
	request = httptest.NewRequest("GET", ready.URL+"?token="+owner.Token, nil)
	request.Header.Set("Authorization", "Bearer 无效")
	writer = httptest.NewRecorder()
	a.router.ServeHTTP(writer, request)
	if writer.Code != 401 {
		t.Fatal("Authorization错误不应回退查询token")
	}
	client.mode.Store(4)
	a.call("POST", path+"/audio", owner.Token, "", 200)
	failed := awaitAudio(t, a, path, owner.Token, "error")
	if failed.Transcript == "" {
		t.Fatal("TTS失败应保留脚本")
	}
	a.call("GET", ready.URL, owner.Token, "", 404)
	a.call("DELETE", path, owner.Token, "", 200)
	if file, err := files.Open(context.Background(), storage.AudioPath(owner.User.ID, book.ID)); !errors.Is(err, os.ErrNotExist) {
		if file != nil {
			file.Close()
		}
		t.Fatal("删除笔记本没有清理音频")
	}
	// 关闭并重新创建服务，验证持久化 generating 恢复与仅文稿模式。
	workshop.Close()
	book = decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	path = "/api/notebooks/" + book.ID
	addStudioSource(t, a, owner, book)
	a.call("POST", path+"/audio", owner.Token, "", 200)
	restored := studio.New(context.Background(), a.database, llm.NewFake(1536), files, config.Config{LLMTTSModel: "即使配置TTS，Fake仍只生成文稿"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(restored.Close)
	ready = awaitAudio(t, a, path, owner.Token, "ready")
	if ready.Transcript == "" || ready.URL != "" || ready.DurationSec <= 0 {
		t.Fatal("仅文稿模式或任务恢复错误")
	}
	a.call("DELETE", path, owner.Token, "", 200)
}

func TestStudioTeacherFormatsRAGAndBinaryExports(t *testing.T) {
	a, _, _, _ := studioAPI(t, false)
	owner, other := a.register(), a.register()
	book := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{"title":"机器学习备课"}`, 200))
	addStudioSource(t, a, owner, book)
	path := "/api/notebooks/" + book.ID

	var sources []model.Source
	if err := a.database.Where("user_id = ? AND notebook_id = ?", owner.User.ID, book.ID).Find(&sources).Error; err != nil || len(sources) == 0 {
		t.Fatal("未找到就绪来源")
	}

	// 空来源选择必须明确报错。
	a.call("POST", path+"/artifacts", owner.Token, `{"kind":"ppt","sourceIds":[]}`, 400)

	// 携带聚焦主题时触发 RAG 向量检索生成 PPT / Word / PDF 三类产物。
	pptArt := decode[model.Artifact](t, a.call("POST", path+"/artifacts", owner.Token, `{"kind":"ppt","topic":"独立验证与泛化能力","preset":"classroom","sourceIds":["`+sources[0].ID+`"]}`, 200))
	if pptArt.Kind != "ppt" || pptArt.Format != "pptx" || pptArt.RetrievalMode != "rag" || pptArt.Slides == nil || len(pptArt.Slides.Slides) == 0 {
		t.Fatalf("PPT RAG 生成异常: %+v", pptArt)
	}
	wordArt := decode[model.Artifact](t, a.call("POST", path+"/artifacts", owner.Token, `{"kind":"word","topic":"独立验证实验要点","preset":"lesson-plan"}`, 200))
	if wordArt.Format != "docx" || wordArt.RetrievalMode != "rag" || wordArt.Content == "" {
		t.Fatalf("Word RAG 生成异常: %+v", wordArt)
	}
	pdfArt := decode[model.Artifact](t, a.call("POST", path+"/artifacts", owner.Token, `{"kind":"pdf","preset":"handout"}`, 200))
	if pdfArt.Format != "pdf" || pdfArt.RetrievalMode != "sources" || pdfArt.Content == "" {
		t.Fatalf("PDF 生成异常: %+v", pdfArt)
	}

	download := func(url, token string, wantStatus int) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		a.router.ServeHTTP(rec, req)
		if rec.Code != wantStatus {
			t.Fatalf("GET %s 状态码 = %d，期望 %d：%s", url, rec.Code, wantStatus, rec.Body.String())
		}
		return rec
	}

	// 验证 .pptx 与 .docx 二进制导出头与 ZIP 魔数，并确认跨用户返回 404。
	pptxRec := download("/api/artifacts/"+pptArt.ID+"/export?format=pptx", owner.Token, 200)
	if !bytes.HasPrefix(pptxRec.Body.Bytes(), []byte("PK\x03\x04")) || !strings.Contains(pptxRec.Header().Get("Content-Type"), "presentationml") {
		t.Fatal("PPTX 导出格式异常")
	}
	docxRec := download("/api/artifacts/"+wordArt.ID+"/export?format=docx", owner.Token, 200)
	if !bytes.HasPrefix(docxRec.Body.Bytes(), []byte("PK\x03\x04")) || !strings.Contains(docxRec.Header().Get("Content-Type"), "wordprocessingml") {
		t.Fatal("DOCX 导出格式异常")
	}
	download("/api/artifacts/"+pptArt.ID+"/export?format=pptx", other.Token, 404)
	download("/api/artifacts/"+pptArt.ID+"/export?format=txt", owner.Token, 400)

	// 验证笔记导出接口。
	note := decode[model.Note](t, a.call("POST", path+"/notes", owner.Token, `{"title":"随堂梳理","content":"## 要点\n- 独立验证","origin":"manual"}`, 200))
	noteDocx := download("/api/notes/"+note.ID+"/export?format=docx", owner.Token, 200)
	if !bytes.HasPrefix(noteDocx.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Fatal("笔记 DOCX 导出格式异常")
	}
	download("/api/notes/"+note.ID+"/export?format=docx", other.Token, 404)
}
