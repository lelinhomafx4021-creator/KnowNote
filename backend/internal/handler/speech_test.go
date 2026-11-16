package handler_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"knownote/internal/config"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
)

type speechRecognizer struct {
	llm.Client
	text, language string
	err            error
	calls          int
}

func (*speechRecognizer) ASREnabled() bool { return true }
func (s *speechRecognizer) Transcribe(_ context.Context, wav []byte, language string) (string, error) {
	s.calls++
	s.language = language
	return s.text, s.err
}

func handlerWAV() []byte {
	wav := make([]byte, 44+32000)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16)
	binary.LittleEndian.PutUint16(wav[20:22], 1)
	binary.LittleEndian.PutUint16(wav[22:24], 1)
	binary.LittleEndian.PutUint32(wav[24:28], 16000)
	binary.LittleEndian.PutUint32(wav[28:32], 32000)
	binary.LittleEndian.PutUint16(wav[32:34], 2)
	binary.LittleEndian.PutUint16(wav[34:36], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:44], 32000)
	return wav
}

type speechEnvelope struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

func speechCall(t *testing.T, router http.Handler, token string, audio []byte, language, extra string, status, code int) speechEnvelope {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if audio != nil {
		part, err := form.CreateFormFile("audio", "recording.wav")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(audio); err != nil {
			t.Fatal(err)
		}
	}
	if language != "" {
		if err := form.WriteField("language", language); err != nil {
			t.Fatal(err)
		}
	}
	if extra != "" {
		if err := form.WriteField("extra", extra); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/speech/transcribe", &body)
	// 不依赖 Content-Length，验证分块传输也受 MaxBytesReader 约束。
	request.ContentLength = -1
	request.Header.Set("Content-Type", form.FormDataContentType())
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	writer := httptest.NewRecorder()
	router.ServeHTTP(writer, request)
	var result speechEnvelope
	if err := json.Unmarshal(writer.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if writer.Code != status || result.Code != code {
		t.Fatalf("转写响应 HTTP=%d，code=%d，message=%s；预期 HTTP=%d，code=%d", writer.Code, result.Code, result.Message, status, code)
	}
	return result
}

func TestSpeechHandlers(t *testing.T) {
	database := sourceDatabase(t)
	var logs bytes.Buffer
	core := service.New(database, middleware.NewTokens(strings.Repeat("s", 32)), "fake", slog.New(slog.NewTextHandler(&logs, nil)))
	a := &testAPI{t: t, database: database, router: handler.NewRouter(core, nil)}
	owner := a.register()
	a.call("GET", "/api/speech/config", "", "", 401)
	speechCall(t, a.router, "", handlerWAV(), "", "", 401, 40101)
	var cfg struct {
		Enabled    bool `json:"enabled"`
		MaxSeconds int  `json:"maxSeconds"`
	}
	cfg = decode[struct {
		Enabled    bool `json:"enabled"`
		MaxSeconds int  `json:"maxSeconds"`
	}](t, a.call("GET", "/api/speech/config", owner.Token, "", 200))
	if cfg.Enabled || cfg.MaxSeconds != 60 {
		t.Fatal("无识别器时配置响应错误")
	}
	speechCall(t, a.router, owner.Token, handlerWAV(), "", "", 503, 50301)
	core.Usage = llm.NewGuard(database, llm.New(config.Config{}), config.Config{})
	result := speechCall(t, a.router, owner.Token, handlerWAV(), "", "", 503, 50301)
	if result.Message != "未配置语音识别服务" {
		t.Fatal("未配置提示不符合契约")
	}
	fake := &speechRecognizer{Client: llm.NewFake(4), text: "  私密转写内容  "}
	core.Usage = llm.NewGuard(database, fake, config.Config{ASRModel: "test-asr", LLMMaxConcurrency: 8})
	cfg = decode[struct {
		Enabled    bool `json:"enabled"`
		MaxSeconds int  `json:"maxSeconds"`
	}](t, a.call("GET", "/api/speech/config", owner.Token, "", 200))
	if !cfg.Enabled || cfg.MaxSeconds != 60 {
		t.Fatal("已配置识别器的能力响应错误")
	}
	result = speechCall(t, a.router, owner.Token, handlerWAV(), "zh", "", 200, 0)
	if data := decode[map[string]string](t, result.Data); data["text"] != "私密转写内容" || len(data) != 1 || fake.language != "zh" {
		t.Fatal("成功响应或语言传递错误")
	}
	stereo := handlerWAV()
	stereo[22] = 2
	for _, test := range []struct {
		name, extra string
		audio       []byte
	}{
		{name: "缺字段"},
		{name: "空文件", audio: []byte{}},
		{name: "格式错误", audio: []byte("私密音频内容")},
		{name: "多声道", audio: stereo},
		{name: "文件过大", audio: make([]byte, llm.SpeechMaxBytes+1)},
		{name: "表单总大小过大", audio: handlerWAV(), extra: strings.Repeat("x", llm.SpeechMaxBytes+65536)},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := fake.calls
			speechCall(t, a.router, owner.Token, test.audio, "", test.extra, 400, 40001)
			if fake.calls != before {
				t.Fatal("无效录音不应调用识别器")
			}
		})
	}
	for _, test := range []struct {
		name, text string
		err        error
		status     int
		code       int
	}{
		{"空转写", "  ", nil, 422, 42201},
		{"无语音", "", llm.ErrNoSpeech, 422, 42201},
		{"上游超时", "", llm.ErrASRTimeout, 504, 50001},
		{"上游限流", "", llm.ErrASRBusy, 502, 50001},
		{"上游原始错误", "", errors.New("私密上游内容与测试凭据"), 502, 50001},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake.text, fake.err = test.text, test.err
			result := speechCall(t, a.router, owner.Token, handlerWAV(), "", "", test.status, test.code)
			if strings.Contains(result.Message, "私密") {
				t.Fatal("上游错误泄露")
			}
		})
	}
	fake.text, fake.err = "私密转写内容", nil
	t.Run("每日请求额度", func(t *testing.T) {
		core.Usage.Config.QuotaDailyChatRequests = 1
		before := fake.calls
		result := speechCall(t, a.router, owner.Token, handlerWAV(), "", "", 429, 42901)
		if result.Message != llm.ErrQuota.Error() || fake.calls != before {
			t.Fatal("请求额度超限未拦截")
		}
		core.Usage.Config.QuotaDailyChatRequests = 0
	})
	t.Run("每日Token额度", func(t *testing.T) {
		other := a.register()
		core.Usage.Config.QuotaDailyTokens = 24
		before := fake.calls
		speechCall(t, a.router, other.Token, handlerWAV(), "", "", 429, 42901)
		if fake.calls != before {
			t.Fatal("Token 预算不足仍调用识别器")
		}
		core.Usage.Config.QuotaDailyTokens = 0
	})
	t.Run("每用户十次持久限流", func(t *testing.T) {
		other := a.register()
		for i := 0; i < 10; i++ {
			speechCall(t, a.router, other.Token, handlerWAV(), "", "", 200, 0)
		}
		core.Usage = llm.NewGuard(database, fake, core.Usage.Config)
		before := fake.calls
		result := speechCall(t, a.router, other.Token, handlerWAV(), "", "", 429, 42901)
		if result.Message != llm.ErrASRRate.Error() || fake.calls != before {
			t.Fatal("限流未跨 Guard 实例生效")
		}
		fresh := a.register()
		speechCall(t, a.router, fresh.Token, handlerWAV(), "", "", 200, 0)
	})
	if strings.Contains(logs.String(), "私密") {
		t.Fatal("音频或转写文本进入日志")
	}
}

func TestSpeechUsageSettlement(t *testing.T) {
	database := sourceDatabase(t)
	var body atomic.Value
	body.Store(`{"choices":[{"message":{"content":"识别内容"}}],"usage":{"total_tokens":37}}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body.Load().(string))
	}))
	defer server.Close()
	cfg := config.Config{ASRBaseURL: server.URL, ASRAPIKey: "仅测试凭据", LLMMaxConcurrency: 8}
	provider := llm.NewProvider(cfg)
	guard := llm.NewGuard(database, provider, cfg)
	core := service.New(database, middleware.NewTokens(strings.Repeat("u", 32)), "fake", slog.New(slog.NewTextHandler(io.Discard, nil)))
	core.Usage = guard
	a := &testAPI{t: t, database: database, router: handler.NewRouter(core, nil)}
	owner := a.register()
	for _, test := range []struct {
		body             string
		status, code     int
		tokens, requests int64
	}{
		{`{"choices":[{"message":{"content":"识别内容"}}],"usage":{"total_tokens":37,"prompt_tokens":1,"completion_tokens":2}}`, 200, 0, 37, 1},
		{`{"choices":[{"message":{"content":"识别内容"}}]}`, 200, 0, 62, 2},
		{`{"choices":[{"message":{"content":"识别内容"}}],"usage":{"total_tokens":0}}`, 200, 0, 62, 3},
		{`{"choices":[{"message":{"content":"  "}}],"usage":{"total_tokens":7}}`, 422, 42201, 69, 4},
		{`无效上游响应`, 502, 50001, 94, 5},
	} {
		body.Store(test.body)
		speechCall(t, a.router, owner.Token, handlerWAV(), "", "", test.status, test.code)
		usage, err := guard.Usage(context.Background(), owner.User.ID)
		if err != nil || usage.ChatRequests != test.requests || usage.Tokens != test.tokens || usage.ReservedTokens != 0 {
			t.Fatalf("ASR 用量结算错误：%+v，错误=%v", usage, err)
		}
	}
	var records []model.LLMUsage
	if err := database.Where("user_id = ? AND capability = ?", owner.User.ID, "asr").Order("created_at").Find(&records).Error; err != nil || len(records) != 5 {
		t.Fatalf("能力记录错误：%v", err)
	}
	for _, record := range records {
		if record.Pending || record.ReservedTokens != 0 || record.Model != "qwen3-asr-flash" {
			t.Fatal("预留未释放或模型记录错误")
		}
	}
	if records[3].Success || records[4].Success {
		t.Fatal("失败调用没有记录失败状态")
	}
	other := a.register()
	ctx := llm.WithUser(context.Background(), other.User.ID)
	// ASR 与对话共享请求额度，多个 Guard 并发也只能预留最后一次额度。
	cfg.QuotaDailyChatRequests = 1
	provider.Replace(llm.New(cfg), cfg)
	body.Store(`{"choices":[{"message":{"content":"完成"}}]}`)
	var wg sync.WaitGroup
	var successes atomic.Int32
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g := llm.NewGuard(database, provider, cfg)
			if _, err := g.Transcribe(ctx, handlerWAV(), ""); err == nil {
				successes.Add(1)
			} else if !errors.Is(err, llm.ErrQuota) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("并发语音识别突破请求额度")
	}
	if _, err := guard.Chat(ctx, nil, llm.ChatOptions{}); !errors.Is(err, llm.ErrQuota) {
		t.Fatalf("语音识别未占用共享请求额度：%v", err)
	}
}
