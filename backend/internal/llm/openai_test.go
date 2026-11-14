package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"knownote/internal/config"
)

func TestOpenAICompatibleRequests(t *testing.T) {
	var chats atomic.Int32
	var batches []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer local-test-placeholder" {
			t.Error("模型请求缺少鉴权")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/chat/completions":
			var request struct {
				Stream bool   `json:"stream"`
				Model  string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			attempt := chats.Add(1)
			if request.Stream {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"你好\"}}]}\n\ndata: [DONE]\n\n")
			} else if attempt == 1 {
				w.WriteHeader(429)
				fmt.Fprint(w, `{"error":{"message":"请求过多","type":"rate_limit"}}`)
			} else {
				if request.Model != "custom-chat" {
					t.Error("没有采用单次请求的模型设置")
				}
				fmt.Fprint(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"中文回答"}}]}`)
			}
		case "/v1/embeddings":
			var request struct {
				Input      []string `json:"input"`
				Dimensions int      `json:"dimensions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Dimensions != 4 || len(request.Input) > 64 {
				t.Error("向量请求的维度或批次错误")
			}
			batches = append(batches, len(request.Input))
			data := make([]map[string]any, 0, len(request.Input))
			for i := len(request.Input) - 1; i >= 0; i-- {
				data = append(data, map[string]any{"index": i, "embedding": []float32{float32(i + 1), 1, 0, 0}})
			}
			if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
				t.Error(err)
			}
		case "/v1/audio/speech":
			var request struct {
				Voice string `json:"voice"`
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Voice != "echo" || request.Model != "local-tts" {
				t.Error("语音配置错误")
			}
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("本地模拟音频"))
		default:
			t.Error("请求没有使用配置的 BaseURL")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := NewOpenAI(config.Config{LLMBaseURL: server.URL + "/v1", LLMAPIKey: "local-test-placeholder", LLMChatModel: "default-chat", LLMEmbedModel: "text-embedding-3-small", EmbeddingDim: 4, LLMTTSModel: "local-tts", LLMTTSVoices: []string{"echo"}})
	answer, err := client.Chat(context.Background(), []Message{{Role: "user", Content: "测试"}}, ChatOptions{Model: "custom-chat"})
	if err != nil || answer != "中文回答" || chats.Load() != 2 {
		t.Fatalf("限流后未正确重试: %v", err)
	}
	var deltas strings.Builder
	answer, err = client.ChatStream(context.Background(), []Message{{Role: "user", Content: "测试"}}, ChatOptions{}, func(s string) error { deltas.WriteString(s); return nil })
	if err != nil || answer != "你好" || deltas.String() != answer {
		t.Fatalf("兼容流式响应错误: %v", err)
	}
	vectors, err := client.Embed(context.Background(), make([]string, 65))
	if err != nil || !reflect.DeepEqual(batches, []int{64, 1}) || len(vectors) != 65 || vectors[0][0] != 1 || vectors[63][0] != 64 {
		t.Fatalf("分批或恢复向量顺序失败: %v", err)
	}
	data, err := client.TTS(context.Background(), "测试", "")
	if err != nil || string(data) != "本地模拟音频" {
		t.Fatalf("读取音频响应失败: %v", err)
	}
	client.ttsModel = ""
	if _, err := client.TTS(context.Background(), "测试", ""); !errors.Is(err, ErrNotConfigured) {
		t.Fatal("未配置 TTS 时应返回约定错误")
	}
}

func TestOpenAIRetryBoundsAndPartialStream(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		var request struct {
			Stream bool `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		if request.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"已生成\"}}]}\n\ndata: 非法内容\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":{"message":"暂不可用"}}`)
	}))
	defer server.Close()
	client := NewOpenAI(config.Config{LLMAPIKey: "local-test-placeholder", LLMBaseURL: server.URL, LLMChatModel: "local-chat"})
	if _, err := client.Chat(context.Background(), nil, ChatOptions{}); err == nil || attempts.Load() != 3 {
		t.Fatal("服务异常时应最多请求三次")
	}
	before := attempts.Load()
	partial, err := client.ChatStream(context.Background(), nil, ChatOptions{}, func(string) error { return nil })
	if err == nil || partial != "已生成" || attempts.Load() != before+1 {
		t.Fatal("流式错误应返回部分内容且不重新请求")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Chat(ctx, nil, ChatOptions{}); !errors.Is(err, context.Canceled) || attempts.Load() != before+1 {
		t.Fatal("取消后不应发送请求")
	}
}
