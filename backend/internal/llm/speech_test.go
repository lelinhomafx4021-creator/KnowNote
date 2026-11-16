package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"knownote/internal/config"
)

func speechWAV(samples, rate int) []byte {
	wav := make([]byte, 44+samples*2)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:8], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:20], 16)
	binary.LittleEndian.PutUint16(wav[20:22], 1)
	binary.LittleEndian.PutUint16(wav[22:24], 1)
	binary.LittleEndian.PutUint32(wav[24:28], uint32(rate))
	binary.LittleEndian.PutUint32(wav[28:32], uint32(rate*2))
	binary.LittleEndian.PutUint16(wav[32:34], 2)
	binary.LittleEndian.PutUint16(wav[34:36], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:44], uint32(samples*2))
	return wav
}

func TestValidateWAV(t *testing.T) {
	for _, test := range []struct {
		name string
		wav  []byte
		edit func([]byte)
		want float64
	}{
		{name: "合法", wav: speechWAV(16000, 16000), want: 1},
		{name: "恰好六十秒", wav: speechWAV(16000*60, 16000), want: 60},
		{name: "其他采样率", wav: speechWAV(8000, 8000), want: 1},
		{name: "空文件"},
		{name: "非RIFF", wav: speechWAV(16, 16000), edit: func(w []byte) { copy(w, "NOPE") }},
		{name: "非WAVE", wav: speechWAV(16, 16000), edit: func(w []byte) { copy(w[8:], "NOPE") }},
		{name: "非PCM", wav: speechWAV(16, 16000), edit: func(w []byte) { w[20] = 3 }},
		{name: "多声道", wav: speechWAV(16, 16000), edit: func(w []byte) { w[22] = 2 }},
		{name: "非16位", wav: speechWAV(16, 16000), edit: func(w []byte) { w[34] = 8 }},
		{name: "零采样率", wav: speechWAV(16, 16000), edit: func(w []byte) { clear(w[24:28]) }},
		{name: "错误字节率", wav: speechWAV(16, 16000), edit: func(w []byte) { clear(w[28:32]) }},
		{name: "空数据", wav: speechWAV(0, 16000)},
		{name: "截断数据", wav: speechWAV(16, 16000), edit: func(w []byte) { w[40] = 255 }},
		{name: "伪造RIFF长度", wav: speechWAV(16, 16000), edit: func(w []byte) { clear(w[4:8]) }},
		{name: "超时长一帧", wav: speechWAV(16000*60+1, 16000)},
		{name: "超大小", wav: make([]byte, SpeechMaxBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.edit != nil {
				test.edit(test.wav)
			}
			seconds, err := ValidateWAV(test.wav)
			if test.want > 0 {
				if err != nil || seconds != test.want {
					t.Fatalf("时长=%v，错误=%v", seconds, err)
				}
			} else {
				var invalid *WAVError
				if !errors.As(err, &invalid) {
					t.Fatalf("无效 WAV 未拒绝：%v", err)
				}
			}
		})
	}
	// 允许头中插入带填充字节的未知块，而不是假定数据从第 44 字节开始。
	wav := speechWAV(16000, 16000)
	chunk := []byte{'J', 'U', 'N', 'K', 1, 0, 0, 0, 7, 0}
	wav = append(append(append([]byte{}, wav[:36]...), chunk...), wav[36:]...)
	binary.LittleEndian.PutUint32(wav[4:8], uint32(len(wav)-8))
	if seconds, err := ValidateWAV(wav); err != nil || seconds != 1 {
		t.Fatalf("未知 RIFF 子块处理失败：%v", err)
	}
}

func TestASRRequestAndResponse(t *testing.T) {
	wav := speechWAV(16000, 16000)
	for _, language := range []string{"", "zh", "en"} {
		t.Run("语言"+language, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Authorization") != "Bearer asr-test-only" {
					t.Error("识别请求路径、方法或请求头不符合约定")
				}
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if string(body["model"]) != `"qwen3-asr-flash"` || string(body["stream"]) != "false" || len(body) != 4 {
					t.Error("识别模型或流式设置不符合约定")
				}
				var messages []struct {
					Role    string `json:"role"`
					Content []struct {
						Type       string `json:"type"`
						InputAudio struct {
							Data string `json:"data"`
						} `json:"input_audio"`
					} `json:"content"`
				}
				if err := json.Unmarshal(body["messages"], &messages); err != nil || len(messages) != 1 || messages[0].Role != "user" || len(messages[0].Content) != 1 {
					t.Error("音频消息结构错误")
					return
				}
				content := messages[0].Content[0]
				data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(content.InputAudio.Data, "data:audio/wav;base64,"))
				if err != nil || content.Type != "input_audio" || !strings.HasPrefix(content.InputAudio.Data, "data:audio/wav;base64,") || !bytes.Equal(data, wav) {
					t.Error("input_audio Data URI 与原音频不一致")
				}
				var options map[string]any
				if err := json.Unmarshal(body["asr_options"], &options); err != nil || options["enable_itn"] != true {
					t.Error("缺少 enable_itn")
				}
				value, exists := options["language"]
				if (language == "" && exists) || (language != "" && (!exists || value != language)) {
					t.Error("language 必须只在指定时发送")
				}
				fmt.Fprint(w, `{"choices":[{"message":{"content":"  转写内容\n "}}],"usage":{"total_tokens":31,"prompt_tokens":2,"completion_tokens":3}}`)
			}))
			defer server.Close()
			client := New(config.Config{ASRBaseURL: server.URL + "/v1/", ASRAPIKey: "asr-test-only"})
			if !ASREnabled(client) {
				t.Fatal("显式配置没有启用识别能力")
			}
			usage := &tokenUsage{}
			ctx := context.WithValue(context.Background(), usageKey{}, usage)
			text, err := client.(Transcriber).Transcribe(ctx, wav, language)
			if err != nil || text != "转写内容" || !usage.exact || usage.prompt != 31 || usage.completion != 0 {
				t.Fatalf("转写或 total_tokens 解析错误：%v", err)
			}
		})
	}
}

func TestASRErrorsAndRetries(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
		want       error
		calls      int32
	}{
		{"空文本", `{"choices":[{"message":{"content":"  \n"}}]}`, 200, ErrNoSpeech, 1},
		{"没有候选", `{"choices":[]}`, 200, ErrNoSpeech, 1},
		{"非法响应", `敏感上游原始内容`, 200, ErrASRUpstream, 1},
		{"认证失败", `敏感凭据 test-only`, 401, ErrASRUpstream, 1},
		{"上游限流", `敏感凭据 test-only`, 429, ErrASRBusy, 3},
		{"上游故障", `敏感凭据 test-only`, 503, ErrASRUpstream, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			client := newASR(config.Config{ASRBaseURL: server.URL, ASRAPIKey: "test-only", ASRModel: "test"})
			_, err := client.Transcribe(context.Background(), speechWAV(16000, 16000), "")
			if !errors.Is(err, test.want) || calls.Load() != test.calls || strings.Contains(err.Error(), "敏感") || strings.Contains(err.Error(), "test-only") {
				t.Fatalf("错误=%v，请求次数=%d", err, calls.Load())
			}
		})
	}
	t.Run("超时与取消", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			<-r.Context().Done()
		}))
		defer server.Close()
		client := newASR(config.Config{ASRBaseURL: server.URL, ASRAPIKey: "test-only"})
		if client.httpClient.Timeout != 30*time.Second {
			t.Fatal("单次识别请求超时应为 30 秒")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if _, err := client.Transcribe(ctx, speechWAV(16000, 16000), ""); !errors.Is(err, ErrASRTimeout) {
			t.Fatalf("超时没有映射为中文提示：%v", err)
		}
		ctx, cancel = context.WithCancel(context.Background())
		cancel()
		if _, err := client.Transcribe(ctx, speechWAV(16000, 16000), ""); !errors.Is(err, context.Canceled) {
			t.Fatalf("取消未透传：%v", err)
		}
	})
	t.Run("重试恢复", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.WriteHeader(502)
				return
			}
			fmt.Fprint(w, `{"choices":[{"message":{"content":"恢复成功"}}]}`)
		}))
		defer server.Close()
		client := New(config.Config{ASRBaseURL: server.URL, ASRAPIKey: "test-only"})
		text, err := client.(Transcriber).Transcribe(context.Background(), speechWAV(16000, 16000), "")
		if err != nil || text != "恢复成功" || calls.Load() != 2 {
			t.Fatalf("重试未恢复：%v", err)
		}
	})
}

func TestASRFakeAndOptionalCapability(t *testing.T) {
	for _, c := range []config.Config{{}, {ASRBaseURL: "https://asr.example/v1"}, {ASRAPIKey: "test-only"}, {EmbedBaseURL: "https://embed.example/v1", EmbedAPIKey: "test-only"}} {
		if ASREnabled(New(c)) {
			t.Fatal("不完整配置或非百炼配置不得启用识别")
		}
	}
	fake := &FakeTranscriber{Text: "  确定文本  "}
	if text, err := fake.Transcribe(context.Background(), nil, ""); err != nil || text != "确定文本" || fake.ASREnabled() {
		t.Fatal("Fake 识别器行为错误")
	}
	p := NewProvider(config.Config{})
	if p.ASREnabled() {
		t.Fatal("Provider 未配置时应禁用识别")
	}
	p.Replace(New(config.Config{ASRBaseURL: "https://asr.example/v1", ASRAPIKey: "test-only"}), config.Config{})
	if !p.ASREnabled() {
		t.Fatal("Provider 未转发可选能力")
	}
}
