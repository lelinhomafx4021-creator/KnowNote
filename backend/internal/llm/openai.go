package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"knownote/internal/config"
)

type OpenAI struct {
	client     *openai.Client
	chatModel  string
	embedModel string
	dimension  int
	ttsModel   string
	voices     []string
}

var _ Client = (*OpenAI)(nil)

const requestTimeout = 90 * time.Second

func NewOpenAI(c config.Config) *OpenAI {
	settings := openai.DefaultConfig(c.LLMAPIKey)
	settings.BaseURL = strings.TrimRight(c.LLMBaseURL, "/")
	settings.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	return &OpenAI{client: openai.NewClientWithConfig(settings), chatModel: c.LLMChatModel, embedModel: c.LLMEmbedModel, dimension: c.EmbeddingDim, ttsModel: c.LLMTTSModel, voices: c.LLMTTSVoices}
}

func (*OpenAI) Name() string { return "openai-compatible" }

func (o *OpenAI) chatRequest(messages []Message, opts ChatOptions) openai.ChatCompletionRequest {
	model := opts.Model
	if model == "" {
		model = o.chatModel
	}
	input := make([]openai.ChatCompletionMessage, len(messages))
	for i, message := range messages {
		input[i] = openai.ChatCompletionMessage{Role: message.Role, Content: message.Content}
	}
	return openai.ChatCompletionRequest{Model: model, Messages: input, Temperature: opts.Temperature, MaxTokens: opts.MaxTokens}
}

func (o *OpenAI) Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	result, err := retry(ctx, func() (openai.ChatCompletionResponse, error) {
		return o.client.CreateChatCompletion(ctx, o.chatRequest(messages, opts))
	})
	if err != nil {
		return "", modelError(err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("模型服务返回了空回答")
	}
	return result.Choices[0].Message.Content, nil
}

func (o *OpenAI) ChatStream(ctx context.Context, messages []Message, opts ChatOptions, onDelta func(string) error) (string, error) {
	if onDelta == nil {
		return "", fmt.Errorf("流式输出回调不能为空")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request := o.chatRequest(messages, opts)
	request.Stream = true
	stream, err := retry(ctx, func() (*openai.ChatCompletionStream, error) { return o.client.CreateChatCompletionStream(ctx, request) })
	if err != nil {
		return "", modelError(err)
	}
	defer stream.Close()
	var output strings.Builder
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return output.String(), nil
		}
		// 流建立后不重试，避免重复输出；将已生成部分交给调用方保存。
		if err != nil {
			return output.String(), modelError(err)
		}
		for _, choice := range chunk.Choices {
			if choice.Index != 0 || choice.Delta.Content == "" {
				continue
			}
			output.WriteString(choice.Delta.Content)
			if err := onDelta(choice.Delta.Content); err != nil {
				return output.String(), err
			}
		}
	}
}

func (o *OpenAI) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += EmbedBatchSize {
		batch := texts[start:min(start+EmbedBatchSize, len(texts))]
		request := openai.EmbeddingRequestStrings{Input: batch, Model: openai.EmbeddingModel(o.embedModel)}
		// 其他兼容模型不一定接受 dimensions 参数，以返回维度校验为准。
		if strings.HasPrefix(o.embedModel, "text-embedding-3-") {
			request.Dimensions = o.dimension
		}
		result, err := retry(ctx, func() (openai.EmbeddingResponse, error) { return o.client.CreateEmbeddings(ctx, request) })
		if err != nil {
			return nil, modelError(err)
		}
		if len(result.Data) != len(batch) {
			return nil, fmt.Errorf("模型返回的向量数量不匹配")
		}
		ordered := make([][]float32, len(batch))
		for _, item := range result.Data {
			if item.Index < 0 || item.Index >= len(batch) || ordered[item.Index] != nil || len(item.Embedding) != o.dimension {
				return nil, fmt.Errorf("模型返回的向量索引或维度错误")
			}
			var norm float64
			for _, value := range item.Embedding {
				if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
					return nil, fmt.Errorf("模型返回的向量包含无效数值")
				}
				norm += float64(value) * float64(value)
			}
			if norm == 0 {
				return nil, fmt.Errorf("模型返回的向量不能为空向量")
			}
			ordered[item.Index] = item.Embedding
		}
		vectors = append(vectors, ordered...)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return vectors, nil
}

func (o *OpenAI) TTS(ctx context.Context, text, voice string) ([]byte, error) {
	if o.ttsModel == "" {
		return nil, ErrNotConfigured
	}
	if voice == "" && len(o.voices) > 0 {
		voice = o.voices[0]
	}
	if voice == "" {
		return nil, fmt.Errorf("请配置语音音色")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	data, err := retry(ctx, func() ([]byte, error) {
		response, err := o.client.CreateSpeech(ctx, openai.CreateSpeechRequest{Model: openai.SpeechModel(o.ttsModel), Input: text, Voice: openai.SpeechVoice(voice), ResponseFormat: openai.SpeechResponseFormatMp3})
		if err != nil {
			return nil, err
		}
		defer response.Close()
		const maxAudioBytes = 32 << 20
		data, err := io.ReadAll(io.LimitReader(response, maxAudioBytes+1))
		if err != nil {
			return nil, err
		}
		if len(data) == 0 || len(data) > maxAudioBytes {
			return nil, fmt.Errorf("模型返回的音频大小无效")
		}
		return data, nil
	})
	if err != nil {
		return nil, modelError(err)
	}
	return data, nil
}

// retry 最多请求三次，仅重试网络故障、408、429 与 5xx，等待可被取消。
func retry[T any](ctx context.Context, call func() (T, error)) (T, error) {
	var zero T
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		result, err := call()
		if err == nil {
			return result, nil
		}
		if attempt == 2 || !retryable(err) {
			return zero, err
		}
		timer := time.NewTimer(time.Duration(1<<attempt) * 200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
	return zero, fmt.Errorf("模型重试次数已耗尽")
}

func retryable(err error) bool {
	status := statusCode(err)
	if status != 0 {
		return status == 408 || status == 429 || (status >= 500 && status <= 599)
	}
	var network net.Error
	return errors.As(err, &network) || errors.Is(err, io.ErrUnexpectedEOF)
}

func statusCode(err error) int {
	var api *openai.APIError
	if errors.As(err, &api) {
		return api.HTTPStatusCode
	}
	var request *openai.RequestError
	if errors.As(err, &request) {
		return request.HTTPStatusCode
	}
	return 0
}

type upstreamError struct{ cause error }

func (e *upstreamError) Error() string {
	if status := statusCode(e.cause); status != 0 {
		return fmt.Sprintf("模型服务请求失败（HTTP %d）", status)
	}
	return "模型服务请求失败，请检查网络与配置"
}
func (e *upstreamError) Unwrap() error { return e.cause }
func modelError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &upstreamError{cause: err}
}
