package handler_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/service/chat"
)

type chatClient struct {
	llm.Client
	embeds   atomic.Int32
	guides   atomic.Int32
	started  chan struct{}
	release  chan struct{}
	onGuide  func()
	mu       sync.Mutex
	options  llm.ChatOptions
	messages []llm.Message
}

func (c *chatClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	c.embeds.Add(1)
	if strings.Contains(texts[0], "模拟向量错误") {
		return nil, errors.New("模拟向量错误")
	}
	return c.Client.Embed(ctx, texts)
}

func (c *chatClient) Chat(ctx context.Context, messages []llm.Message, options llm.ChatOptions) (string, error) {
	if strings.HasPrefix(messages[0].Content, llm.NotebookGuidePrompt) {
		c.guides.Add(1)
		if c.started != nil {
			c.started <- struct{}{}
			select {
			case <-c.release:
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		if c.onGuide != nil {
			c.onGuide()
		}
	}
	return c.Client.Chat(ctx, messages, options)
}

func (c *chatClient) ChatStream(ctx context.Context, messages []llm.Message, options llm.ChatOptions, delta func(string) error) (string, error) {
	c.mu.Lock()
	c.options, c.messages = options, messages
	c.mu.Unlock()
	question := messages[len(messages)-1].Content
	if strings.Contains(question, "模拟模型错误") {
		if err := delta("已生成部分回答 [2]"); err != nil {
			return "", err
		}
		return "已生成部分回答 [2]", errors.New("模拟模型错误")
	}
	return c.Client.ChatStream(ctx, messages, options, delta)
}

func streamChat(t *testing.T, a *testAPI, path, token, body string) []chat.Event {
	t.Helper()
	request := httptest.NewRequest("POST", path+"/chat", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	writer := httptest.NewRecorder()
	a.router.ServeHTTP(writer, request)
	if writer.Code != 200 || writer.Header().Get("Cache-Control") != "no-cache" || writer.Header().Get("X-Accel-Buffering") != "no" || !strings.Contains(writer.Header().Get("Content-Type"), "text/event-stream") || !writer.Flushed {
		t.Fatalf("SSE 状态、响应头或 Flush 不正确：%d", writer.Code)
	}
	var events []chat.Event
	scanner := bufio.NewScanner(writer.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			t.Fatal("事件不是 data 行")
		}
		var event chat.Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func TestChatIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := sourceDatabase(t)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	core := service.New(database, middleware.NewTokens(strings.Repeat("c", 32)), "fake-model", logger)
	client := &chatClient{Client: llm.NewFake(1536)}
	chats := chat.New(database, client, logger)
	a := &testAPI{t: t, database: database, router: handler.NewRouter(core, nil, handler.ChatRoutes(core, chats))}
	owner, other := a.register(), a.register()
	notebook := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	path := "/api/notebooks/" + notebook.ID
	if string(a.call("GET", path+"/messages", owner.Token, "", 200)) != "[]" {
		t.Fatal("历史应为空数组")
	}
	empty := decode[llm.Guide](t, a.call("GET", path+"/guide", owner.Token, "", 200))
	if empty.Summary != "" || empty.SuggestedQuestions == nil || len(empty.SuggestedQuestions) != 0 {
		t.Fatal("无来源概要错误")
	}
	for _, suffix := range []string{"/messages", "/guide"} {
		a.call("GET", path+suffix, other.Token, "", 404)
	}
	a.call("DELETE", path+"/messages", other.Token, "", 404)
	a.call("POST", path+"/chat", other.Token, `{"question":"问题","sourceIds":[]}`, 404)
	for _, input := range []string{`{"question":" ","sourceIds":[]}`, `{"question":"问题"}`, `{"question":"问题","sourceIds":null}`, `{"question":"问题","sourceIds":["非法"]}`} {
		a.call("POST", path+"/chat", owner.Token, input, 400)
	}
	events := streamChat(t, a, path, owner.Token, `{"question":"未选来源","sourceIds":[]}`)
	if len(events) != 3 || events[0].Type != "citations" || events[0].Citations == nil || events[1].Type != "delta" || !strings.Contains(events[1].Text, "请选择来源") || events[2].Type != "done" || client.embeds.Load() != 0 {
		t.Fatal("空来源未跳过模型或 SSE 顺序错误")
	}
	messageID := events[2].MessageID
	for _, value := range []string{`"up"`, `"down"`, `null`} {
		a.call("POST", "/api/messages/"+messageID+"/feedback", owner.Token, `{"value":`+value+`}`, 200)
	}
	a.call("POST", "/api/messages/"+messageID+"/feedback", other.Token, `{"value":"up"}`, 404)
	for _, body := range []string{`{}`, `{"value":"bad"}`, `{"value":1}`} {
		a.call("POST", "/api/messages/"+messageID+"/feedback", owner.Token, body, 400)
	}
	a.call("DELETE", path+"/messages", owner.Token, "", 200)

	ownerKey := model.NotebookOwner{UserID: owner.User.ID, NotebookID: notebook.ID}
	var sources []model.Source
	for _, text := range []string{"机器学习模型训练依赖训练数据，独立验证数据用于评估模型效果。", "机器学习模型验证用于衡量泛化能力，训练与验证数据需要分离。"} {
		source := model.Source{NotebookOwner: ownerKey, Type: "text", Title: "模型训练与验证", Status: "ready", Content: text, Summary: text}
		if err := database.Create(&source).Error; err != nil {
			t.Fatal(err)
		}
		vectors, err := client.Client.Embed(context.Background(), []string{text})
		if err != nil {
			t.Fatal(err)
		}
		chunk := model.Chunk{NotebookOwner: ownerKey, SourceID: source.ID, Content: text, Embedding: model.NewVector(vectors[0])}
		if err := database.Create(&chunk).Error; err != nil {
			t.Fatal(err)
		}
		sources = append(sources, source)
	}
	foreignNotebook := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	foreign := model.Source{NotebookOwner: model.NotebookOwner{UserID: owner.User.ID, NotebookID: foreignNotebook.ID}, Type: "text", Title: "其他笔记本", Status: "ready"}
	if err := database.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	for _, sourceID := range []string{foreign.ID, uuid.NewString()} {
		a.call("POST", path+"/chat", owner.Token, `{"question":"问题","sourceIds":["`+sourceID+`"]}`, 404)
	}
	if err := database.Model(&model.Source{}).Where("user_id = ? AND id = ?", owner.User.ID, foreign.ID).Updates(map[string]any{"notebook_id": notebook.ID, "status": "processing"}).Error; err != nil {
		t.Fatal(err)
	}
	a.call("POST", path+"/chat", owner.Token, `{"question":"问题","sourceIds":["`+foreign.ID+`"]}`, 400)
	a.call("PATCH", "/api/me/settings", owner.Token, `{"answerStyle":"learning","model":"selected-model"}`, 200)
	body := `{"question":"机器学习模型训练与验证","sourceIds":["` + sources[0].ID + `","` + sources[1].ID + `"]}`
	events = streamChat(t, a, path, owner.Token, body)
	if len(events) < 3 || events[0].Type != "citations" || len(events[0].Citations) != 2 || events[len(events)-1].Type != "done" {
		t.Fatal("事件顺序或检索引用不正确")
	}
	var content strings.Builder
	for _, event := range events[1 : len(events)-1] {
		if event.Type != "delta" {
			t.Fatal("citations 和 done 之间必须都是 delta")
		}
		content.WriteString(event.Text)
	}
	if !strings.Contains(content.String(), "[1]") || !strings.Contains(content.String(), "[2]") {
		t.Fatal("Fake 没有引用两份资料")
	}
	messages := decode[[]model.Message](t, a.call("GET", path+"/messages", owner.Token, "", 200))
	if len(messages) != 2 || messages[0].Role != "user" || messages[1].ID != events[len(events)-1].MessageID || messages[1].Content != content.String() || len(messages[1].Citations) != 2 {
		t.Fatal("SSE 内容或消息没有落库")
	}
	if client.options.Model != "selected-model" || !strings.Contains(client.messages[0].Content, "学习引导") {
		t.Fatal("未使用用户模型或风格设置")
	}
	// 完全不相关的中文问题在 Fake 下低于阈值，也必须有可回溯的上下文与落库引用。
	fallbackBody := strings.Replace(body, "机器学习模型训练与验证", "为什么毕设用 pgvector？", 1)
	events = streamChat(t, a, path, owner.Token, fallbackBody)
	if len(events[0].Citations) == 0 || events[len(events)-1].Type != "done" || !strings.Contains(logs.String(), "检索片段均低于阈值") {
		t.Fatal("低相似度检索未兜底或没有记录日志")
	}
	fallbackMessages := decode[[]model.Message](t, a.call("GET", path+"/messages", owner.Token, "", 200))
	fallbackAnswer := fallbackMessages[len(fallbackMessages)-1]
	if len(fallbackAnswer.Citations) == 0 || !strings.Contains(fallbackAnswer.Content, "[1]") {
		t.Fatal("兜底上下文未用于流式回答或引用未落库")
	}
	failedBody := strings.Replace(body, "机器学习模型训练与验证", "机器学习模型训练与验证，模拟模型错误", 1)
	events = streamChat(t, a, path, owner.Token, failedBody)
	if events[0].Type != "citations" || events[len(events)-1].Type != "error" {
		t.Fatal("模型错误没有发 error 事件")
	}
	messages = decode[[]model.Message](t, a.call("GET", path+"/messages", owner.Token, "", 200))
	last := messages[len(messages)-1]
	if last.Content != "已生成部分回答 [2]" || len(last.Citations) != 1 || last.Citations[0].Index != 2 {
		t.Fatal("错误时未保存部分内容或引用被重新编号")
	}
	events = streamChat(t, a, path, owner.Token, strings.Replace(body, "机器学习模型训练与验证", "模拟向量错误", 1))
	if len(events) != 1 || events[0].Type != "error" {
		t.Fatal("向量错误未发送 error")
	}

	// 通过真实 HTTP 连接，在首个 delta 到达后取消，验证及时停止并独立落库。
	server := httptest.NewServer(a.router)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	request, err := http.NewRequestWithContext(ctx, "POST", server.URL+path+"/chat", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+owner.Token)
	request.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), `"type":"delta"`) {
			break
		}
	}
	cancel()
	res.Body.Close()
	deadline := time.Now().Add(5 * time.Second)
	var partial model.Message
	for time.Now().Before(deadline) {
		partial = model.Message{}
		if err := database.Where("user_id = ? AND notebook_id = ? AND role = ?", owner.User.ID, notebook.ID, "assistant").Order("created_at DESC").First(&partial).Error; err != nil {
			t.Fatal(err)
		}
		if partial.Content != "" && partial.Content != last.Content && partial.Content != content.String() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if partial.Content == "" || partial.Content == content.String() || partial.Content == last.Content {
		t.Fatal("断开连接未保存部分回答")
	}
	// 等待 HTTP handler 收敛后读取日志，避免与日志写入并发。
	server.Close()
	if !strings.Contains(logs.String(), "停止生成并保存部分回答") {
		t.Fatal("断开日志缺失")
	}

	// 并发概要只调一次模型，后续缓存命中；来源变化后重新生成。
	client.started, client.release = make(chan struct{}, 10), make(chan struct{})
	var wait sync.WaitGroup
	results := make(chan llm.Guide, 8)
	errorsChannel := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			guide, err := chats.Guide(context.Background(), owner.User.ID, notebook.ID)
			results <- guide
			errorsChannel <- err
		}()
	}
	select {
	case <-client.started:
	case <-time.After(5 * time.Second):
		t.Fatal("概要未开始生成")
	}
	close(client.release)
	wait.Wait()
	close(results)
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatal(err)
		}
	}
	for guide := range results {
		if len([]rune(guide.Summary)) < 120 || len([]rune(guide.Summary)) > 220 || len(guide.SuggestedQuestions) != 3 {
			t.Fatal("概要内容不符合契约")
		}
	}
	if client.guides.Load() != 1 {
		t.Fatal("概要并发生成未合并")
	}
	client.started = nil
	a.call("GET", path+"/guide", owner.Token, "", 200)
	if client.guides.Load() != 1 {
		t.Fatal("概要未命中缓存")
	}
	if err := database.Model(&model.NotebookGuide{}).Where("user_id = ? AND notebook_id = ?", owner.User.ID, notebook.ID).Update("stale", true).Error; err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	client.onGuide = func() {
		once.Do(func() {
			if err := database.Model(&model.Source{}).Where("user_id = ? AND id = ?", owner.User.ID, sources[0].ID).Update("summary", "更新后的机器学习资料：训练和验证数据需要隔离。").Error; err != nil {
				t.Error(err)
			}
		})
	}
	a.call("GET", path+"/guide", owner.Token, "", 200)
	if client.guides.Load() != 3 {
		t.Fatal("生成期间来源变化没有重新生成")
	}
	a.call("DELETE", path+"/messages", owner.Token, "", 200)
	if string(a.call("GET", path+"/messages", owner.Token, "", 200)) != "[]" {
		t.Fatal("清空消息失败")
	}
}
