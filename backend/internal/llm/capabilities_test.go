package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"image/png"
	"io"
	"knownote/internal/config"
	"knownote/internal/storage"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestIndependentCapabilities(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		c := config.Config{EmbeddingDim: 4, ChatBaseURL: "https://chat.example", EmbedBaseURL: "https://embed.example", TTSBaseURL: "https://tts.example", TTSModel: "tts"}
		if mask&1 != 0 {
			c.ChatAPIKey = "chat-test"
		}
		if mask&2 != 0 {
			c.EmbedAPIKey = "embed-test"
		}
		if mask&4 != 0 {
			c.TTSAPIKey = "tts-test"
		}
		client := New(c).(*Capabilities)
		for i, part := range []Client{client.chat, client.embed, client.tts} {
			if (part.Name() != "fake") != (mask&(1<<i) != 0) {
				t.Fatal("能力未独立切换真实与 Fake")
			}
		}
		if TTSEnabled(client) != (mask&4 != 0) {
			t.Fatal("语音能力判断错误")
		}
	}
}
func TestDeepSeekReasoningJSONAndUsage(t *testing.T) {
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Error("DeepSeek 路径错误")
		}
		var request struct {
			Stream bool `json:"stream"`
			Format *struct {
				Type string `json:"type"`
			} `json:"response_format"`
			StreamOptions struct {
				Include bool `json:"include_usage"`
			} `json:"stream_options"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		if request.Stream {
			if !request.StreamOptions.Include {
				t.Error("未请求最终用量")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"内部推理不可发送\"}}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"正式回答\",\"reasoning_content\":\"忽略\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":20}}\n\ndata: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if count.Add(1) == 1 {
			if request.Format == nil || request.Format.Type != "json_object" {
				t.Error("未优先请求 JSON 模式")
			}
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":{"message":"response_format unsupported"}}`)
			return
		}
		if request.Format != nil {
			t.Error("未回退文本 JSON")
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"ok\":true}"}}],"usage":{"prompt_tokens":5,"completion_tokens":3}}`)
	}))
	defer server.Close()
	client := New(config.Config{ChatBaseURL: server.URL, ChatAPIKey: "local-test-only", ChatModel: "deepseek-flash", EmbeddingDim: 4})
	capture := &tokenUsage{}
	ctx := context.WithValue(context.Background(), usageKey{}, capture)
	raw, err := client.Chat(ctx, []Message{{Role: "user", Content: "输出 JSON"}}, ChatOptions{JSON: true})
	if err != nil || raw != `{"ok":true}` || capture.prompt != 5 {
		t.Fatalf("JSON 降级或用量错误：%v", err)
	}
	capture = &tokenUsage{}
	ctx = context.WithValue(context.Background(), usageKey{}, capture)
	var delta strings.Builder
	raw, err = client.ChatStream(ctx, nil, ChatOptions{}, func(text string) error { delta.WriteString(text); return nil })
	if err != nil || raw != "正式回答" || delta.String() != raw || capture.completion != 20 {
		t.Fatalf("推理过滤或流式用量错误：%v", err)
	}
}
func TestEmbeddingDimensionsProbe(t *testing.T) {
	var wrong atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Dimensions int      `json:"dimensions"`
			Input      []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		if request.Dimensions != 4 || len(request.Input) > 10 {
			t.Error("百炼 dimensions 或批次错误")
		}
		dim := 4
		if wrong.Load() {
			dim = 2
		}
		data := make([]map[string]any, len(request.Input))
		for i := range data {
			vec := make([]float32, dim)
			vec[0] = 1
			data[i] = map[string]any{"index": i, "embedding": vec}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	c := config.Config{EmbedBaseURL: server.URL, EmbedAPIKey: "local-test-only", EmbedModel: "text-embedding-v4", EmbeddingDim: 4}
	client := New(c)
	if err := ProbeEmbedding(context.Background(), client, c); err != nil {
		t.Fatal(err)
	}
	wrong.Store(true)
	if err := ProbeEmbedding(context.Background(), client, c); err == nil || !strings.Contains(err.Error(), "返回 2 维") || !strings.Contains(err.Error(), "reindex.sh") {
		t.Fatal("维度不匹配缺少处理办法")
	}
	c.EmbedAPIKey = ""
	if err := ProbeEmbedding(context.Background(), New(c), c); err != nil {
		t.Fatal("Fake 不应请求探测")
	}
}
func TestImageFakeAndCompatibleResponses(t *testing.T) {
	ctx := WithUser(context.Background(), uuid.NewString())
	store, err := storage.New(ctx, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fake := NewImage(config.Config{}, store)
	result, err := fake.Generate(ctx, "知识管理与资料检索", ImageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	file, _ := store.Open(ctx, result.ObjectKey)
	data, _ := io.ReadAll(file)
	file.Close()
	image, err := png.Decode(bytes.NewReader(data))
	if err != nil || image.Bounds().Dx() != 768 {
		t.Fatal("Fake 图片不是有效 PNG")
	}
	other, _ := fakeImage("different prompt")
	if bytes.Equal(data, other) {
		t.Fatal("提示词未绘制到图片")
	}
	for _, mode := range []string{"b64", "url", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			var address string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/image.png" {
					if r.Header.Get("Authorization") != "" {
						t.Error("图片下载泄露 API Key")
					}
					w.Write(data)
					return
				}
				if r.URL.Path != "/images/generations" {
					t.Error("图片生成路径错误")
				}
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["prompt"] != "测试封面" || body["model"] != "image-model" {
					t.Error("图像请求参数错误")
				}
				w.Header().Set("Content-Type", "application/json")
				if mode == "url" {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"url": address + "/image.png"}}})
				} else {
					encoded := base64.StdEncoding.EncodeToString(data)
					if mode == "invalid" {
						encoded = "invalid!"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"b64_json": encoded}}})
				}
			}))
			defer server.Close()
			address = server.URL
			client := NewImage(config.Config{ImageProvider: "openai", ImageBaseURL: server.URL, ImageAPIKey: "local-image-test", ImageModel: "image-model"}, store)
			out, err := client.Generate(ctx, "测试封面", ImageOptions{Size: "1024x1024"})
			if mode == "invalid" {
				if err == nil {
					t.Fatal("非法 b64 未拒绝")
				}
				return
			}
			if err != nil || out.Width != 768 {
				t.Fatalf("图片解析或存储失败：%v", err)
			}
			file, _ := store.Open(ctx, out.ObjectKey)
			saved, _ := io.ReadAll(file)
			file.Close()
			if !bytes.Equal(saved, data) {
				t.Fatal("存储字节不同")
			}
		})
	}
}

func TestCapabilityRequestsUseOwnProviders(t *testing.T) {
	var chats, embeds, speeches atomic.Int32
	serve := func(ability string, counter *atomic.Int32) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counter.Add(1)
			if r.Header.Get("Authorization") != "Bearer local-"+ability {
				t.Error("密钥路由串到其他能力")
			}
			w.Header().Set("Content-Type", "application/json")
			switch ability {
			case "chat":
				if r.URL.Path != "/chat/completions" {
					t.Error("对话地址错误")
				}
				fmt.Fprint(w, `{"choices":[{"message":{"content":"真实对话模拟"}}]}`)
			case "embed":
				if r.URL.Path != "/embeddings" {
					t.Error("向量地址错误")
				}
				fmt.Fprint(w, `{"data":[{"index":0,"embedding":[1,0,0,0]}],"usage":{"total_tokens":9}}`)
			case "tts":
				if r.URL.Path != "/audio/speech" {
					t.Error("语音地址错误")
				}
				fmt.Fprint(w, "模拟音频")
			}
		}))
	}
	chatServer, embedServer, ttsServer := serve("chat", &chats), serve("embed", &embeds), serve("tts", &speeches)
	defer chatServer.Close()
	defer embedServer.Close()
	defer ttsServer.Close()
	for mask := 0; mask < 8; mask++ {
		c := config.Config{ChatBaseURL: chatServer.URL, ChatModel: "test-chat", EmbedBaseURL: embedServer.URL, EmbedModel: "text-embedding-v4", EmbeddingDim: 4, TTSBaseURL: ttsServer.URL, TTSModel: "test-tts", TTSVoices: []string{"echo"}}
		if mask&1 != 0 {
			c.ChatAPIKey = "local-chat"
		}
		if mask&2 != 0 {
			c.EmbedAPIKey = "local-embed"
		}
		if mask&4 != 0 {
			c.TTSAPIKey = "local-tts"
		}
		client := New(c)
		answer, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "演示"}}, ChatOptions{})
		if err != nil || (answer == "真实对话模拟") != (mask&1 != 0) {
			t.Fatal("对话真实/Fake 行为不正确")
		}
		usage := &tokenUsage{}
		vectors, err := client.Embed(context.WithValue(context.Background(), usageKey{}, usage), []string{"向量"})
		if err != nil || len(vectors) != 1 || len(vectors[0]) != 4 {
			t.Fatal("独立向量请求失败")
		}
		if mask&2 != 0 && usage.prompt != 9 {
			t.Fatal("百炼 total_tokens 未计入输入用量")
		}
		data, err := client.TTS(context.Background(), "文稿", "")
		if mask&4 != 0 {
			if err != nil || string(data) != "模拟音频" {
				t.Fatal("独立语音请求失败")
			}
		} else if err != ErrNotConfigured {
			t.Fatal("Fake 语音应保持仅文稿行为")
		}
	}
	if chats.Load() != 4 || embeds.Load() != 4 || speeches.Load() != 4 {
		t.Fatal("未启用能力仍然请求了上游")
	}
	var output bytes.Buffer
	c := config.Config{LLMBaseURL: "https://legacy.example/v1", LLMAPIKey: "local-legacy-secret", LLMChatModel: "default"}.Resolved()
	if c.ImageBaseURL != c.LLMBaseURL || c.ImageAPIKey != c.LLMAPIKey {
		t.Fatal("图像能力未继承旧配置")
	}
	LogCapabilities(slog.New(slog.NewTextHandler(&output, nil)), c)
	if strings.Contains(output.String(), "local-legacy-secret") || strings.Count(output.String(), "模型能力配置") != 4 {
		t.Fatal("能力日志泄露密钥或缺少逐项输出")
	}
}
