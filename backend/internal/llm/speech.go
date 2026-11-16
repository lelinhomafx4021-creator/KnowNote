package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"knownote/internal/config"
)

const (
	SpeechMaxSeconds = 60
	SpeechMaxBytes   = 4 << 20
	asrTimeout       = 30 * time.Second
)

var (
	ErrASRNotConfigured = errors.New("未配置语音识别服务")
	ErrNoSpeech         = errors.New("没有听清，请再说一遍")
	ErrASRTimeout       = errors.New("语音识别超时，请稍后重试")
	ErrASRUpstream      = errors.New("语音识别服务暂时不可用，请稍后重试")
	ErrASRBusy          = errors.New("语音识别服务繁忙，请稍后重试")
)

// Transcriber 是可选能力，避免扩展 Client 破坏现有调用方与测试替身。
type Transcriber interface {
	Transcribe(ctx context.Context, wav []byte, language string) (string, error)
	ASREnabled() bool
}

func ASREnabled(client Client) bool {
	value, ok := client.(Transcriber)
	return ok && value.ASREnabled()
}

func (c *Capabilities) ASREnabled() bool { return c.asr != nil && c.asr.ASREnabled() }
func (c *Capabilities) Transcribe(ctx context.Context, wav []byte, language string) (string, error) {
	return c.asr.Transcribe(ctx, wav, language)
}
func (p *Provider) ASREnabled() bool { return ASREnabled(p.Current().Client) }
func (p *Provider) Transcribe(ctx context.Context, wav []byte, language string) (string, error) {
	value, ok := p.Current().Client.(Transcriber)
	if !ok {
		return "", ErrASRNotConfigured
	}
	return value.Transcribe(ctx, wav, language)
}

// FakeTranscriber 仅供测试注入确定文本，不作为已配置的真实识别服务。
type FakeTranscriber struct{ Text string }

func (*FakeTranscriber) ASREnabled() bool { return false }
func (f *FakeTranscriber) Transcribe(ctx context.Context, _ []byte, _ string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text := strings.TrimSpace(f.Text)
	if text == "" {
		return "", ErrNoSpeech
	}
	return text, nil
}

type WAVError struct{ Message string }

func (e *WAVError) Error() string { return e.Message }

// ValidateWAV 遍历 RIFF 子块，按真实 PCM 数据长度计算时长，不信任扩展名或 MIME。
func ValidateWAV(wav []byte) (float64, error) {
	invalid := &WAVError{"音频格式不支持，请重试"}
	if len(wav) > SpeechMaxBytes {
		return 0, &WAVError{"录音文件不能超过 4MB"}
	}
	if len(wav) < 12 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || uint64(binary.LittleEndian.Uint32(wav[4:8]))+8 != uint64(len(wav)) {
		return 0, invalid
	}
	var sampleRate, dataBytes uint64
	var haveFormat, haveData bool
	for offset := 12; offset < len(wav); {
		if len(wav)-offset < 8 {
			return 0, invalid
		}
		size := uint64(binary.LittleEndian.Uint32(wav[offset+4 : offset+8]))
		start := offset + 8
		if size > uint64(len(wav)-start) {
			return 0, invalid
		}
		chunk := wav[start : start+int(size)]
		switch string(wav[offset : offset+4]) {
		case "fmt ":
			if haveFormat || size < 16 || binary.LittleEndian.Uint16(chunk[:2]) != 1 || binary.LittleEndian.Uint16(chunk[2:4]) != 1 || binary.LittleEndian.Uint16(chunk[14:16]) != 16 {
				return 0, invalid
			}
			sampleRate = uint64(binary.LittleEndian.Uint32(chunk[4:8]))
			if sampleRate == 0 || uint64(binary.LittleEndian.Uint32(chunk[8:12])) != sampleRate*2 || binary.LittleEndian.Uint16(chunk[12:14]) != 2 {
				return 0, invalid
			}
			haveFormat = true
		case "data":
			if !haveFormat || haveData || size == 0 || size%2 != 0 {
				return 0, invalid
			}
			dataBytes, haveData = size, true
		}
		// RIFF 子块按偶数字节对齐；奇数长度的未知子块同样需要填充字节。
		next := uint64(start) + size + size%2
		if next > uint64(len(wav)) {
			return 0, invalid
		}
		offset = int(next)
	}
	if !haveFormat || !haveData {
		return 0, invalid
	}
	if dataBytes > sampleRate*2*SpeechMaxSeconds {
		return 0, &WAVError{"录音不能超过 60 秒"}
	}
	return float64(dataBytes) / float64(sampleRate*2), nil
}

type asrClient struct {
	baseURL, apiKey, model string
	httpClient             *http.Client
}

func newASR(c config.Config) *asrClient {
	return &asrClient{baseURL: strings.TrimRight(c.ASRBaseURL, "/"), apiKey: c.ASRAPIKey, model: c.ASRModel,
		httpClient: &http.Client{Timeout: asrTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (*asrClient) ASREnabled() bool { return true }

type asrHTTPError struct{ status int }

func (*asrHTTPError) Error() string { return "语音识别上游请求失败" }

func (a *asrClient) Transcribe(ctx context.Context, wav []byte, language string) (string, error) {
	if _, err := ValidateWAV(wav); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, asrTimeout)
	defer cancel()
	options := map[string]any{"enable_itn": true}
	if language != "" {
		options["language"] = language
	}
	body, err := json.Marshal(map[string]any{
		"model": a.model, "stream": false, "asr_options": options,
		"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{
			"type": "input_audio", "input_audio": map[string]string{"data": "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(wav)},
		}}}},
	})
	if err != nil {
		return "", ErrASRUpstream
	}
	type asrResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens *int `json:"total_tokens"`
		} `json:"usage"`
	}
	result, err := retry(ctx, func() (asrResponse, error) {
		var result asrResponse
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return result, ErrASRUpstream
		}
		request.Header.Set("Authorization", "Bearer "+a.apiKey)
		request.Header.Set("Content-Type", "application/json")
		response, err := a.httpClient.Do(request)
		if err != nil {
			return result, err
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			// 不读取上游错误正文，避免原始报错、音频或密钥进入日志与响应。
			return result, &asrHTTPError{status: response.StatusCode}
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		if err != nil {
			return result, err
		}
		if len(data) > 1<<20 || json.Unmarshal(data, &result) != nil {
			return result, ErrASRUpstream
		}
		return result, nil
	})
	if err != nil {
		var network net.Error
		switch {
		case errors.Is(err, context.DeadlineExceeded), errors.As(err, &network) && network.Timeout():
			return "", ErrASRTimeout
		case errors.Is(err, context.Canceled):
			return "", context.Canceled
		case statusCode(err) == http.StatusTooManyRequests:
			return "", ErrASRBusy
		default:
			return "", ErrASRUpstream
		}
	}
	if total := result.Usage.TotalTokens; total != nil && *total >= 0 {
		// total_tokens 包括全部用量，不能以文本字符数或输出 Token 覆盖。
		if usage, ok := ctx.Value(usageKey{}).(*tokenUsage); ok {
			usage.prompt, usage.completion, usage.exact = *total, 0, true
		}
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return "", ErrNoSpeech
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func (g *Guard) ASREnabled() bool { return ASREnabled(g.snapshot().Client) }
func (g *Guard) Transcribe(ctx context.Context, wav []byte, language string) (out string, err error) {
	g = g.snapshot()
	client, ok := g.Client.(Transcriber)
	if !ok || !client.ASREnabled() {
		return "", ErrASRNotConfigured
	}
	seconds, err := ValidateWAV(wav)
	if err != nil {
		return "", err
	}
	estimate := int(math.Ceil(seconds * 25))
	err = g.invoke(ctx, "asr", g.Config.ASRModel, estimate, estimate, func(ctx context.Context) (int, error) {
		var callErr error
		out, callErr = client.Transcribe(ctx, wav, language)
		out = strings.TrimSpace(out)
		if callErr == nil && out == "" {
			callErr = ErrNoSpeech
		}
		return 0, callErr
	})
	return
}
