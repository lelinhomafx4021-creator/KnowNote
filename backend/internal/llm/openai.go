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
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"
	"knownote/internal/config"
)

type OpenAI struct {
	client      *openai.Client
	embedClient *openai.Client
	ttsClient   *openai.Client
	chatModel   string
	embedModel  string
	dimension   int
	ttsModel    string
	voices      []string
}

var _ Client = (*OpenAI)(nil)

const requestTimeout = 90 * time.Second

func NewOpenAI(c config.Config) *OpenAI {
	c = c.Resolved()
	makeClient := func(base, key string) *openai.Client {
		settings := openai.DefaultConfig(key)
		settings.BaseURL = strings.TrimRight(base, "/")
		settings.HTTPClient = &http.Client{Timeout: requestTimeout}
		return openai.NewClientWithConfig(settings)
	}
	return &OpenAI{client: makeClient(c.ChatBaseURL, c.ChatAPIKey), embedClient: makeClient(c.EmbedBaseURL, c.EmbedAPIKey), ttsClient: makeClient(c.TTSBaseURL, c.TTSAPIKey), chatModel: c.ChatModel, embedModel: c.EmbedModel, dimension: c.EmbeddingDim, ttsModel: c.TTSModel, voices: c.TTSVoices}
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
	request := openai.ChatCompletionRequest{Model: model, Messages: input, Temperature: opts.Temperature, MaxTokens: opts.MaxTokens}
	if opts.JSON {
		request.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}
	}
	return request
}

func (o *OpenAI) Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	request := o.chatRequest(messages, opts)
	call := func() (openai.ChatCompletionResponse, error) { return o.client.CreateChatCompletion(ctx, request) }
	result, err := retry(ctx, call)
	// 部分兼容端不支持 JSON 模式，仅在参数不支持时降级为原有文本 JSON 解析。
	if opts.JSON && (statusCode(err) == 400 || statusCode(err) == 422) {
		request.ResponseFormat = nil
		result, err = retry(ctx, call)
	}
	if err != nil {
		return "", modelError(err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("模型服务返回了空回答")
	}
	observeUsage(ctx, result.Usage.PromptTokens, result.Usage.CompletionTokens)
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
	request.StreamOptions = &openai.StreamOptions{IncludeUsage: true}
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
		if chunk.Usage != nil {
			observeUsage(ctx, chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens)
		}
		// reasoning_content 仅为推理过程，绝不传给前端或保存为正式回答。
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
	batchSize := EmbedBatchSize
	// 百炼 text-embedding-v 系列的行数上限低于 OpenAI，统一取兼容的十条。
	if strings.HasPrefix(o.embedModel, "text-embedding-v") {
		batchSize = min(batchSize, 10)
	}
	for start := 0; start < len(texts); start += batchSize {
		batch := texts[start:min(start+batchSize, len(texts))]
		request := openai.EmbeddingRequestStrings{Input: batch, Model: openai.EmbeddingModel(o.embedModel)}
		// 其他兼容模型不一定接受 dimensions 参数，以返回维度校验为准。
		if strings.HasPrefix(o.embedModel, "text-embedding-3-") || o.embedModel == "text-embedding-v3" || o.embedModel == "text-embedding-v4" || strings.HasPrefix(o.embedModel, "qwen3.7-text-embedding") {
			request.Dimensions = o.dimension
		}
		result, err := retry(ctx, func() (openai.EmbeddingResponse, error) { return o.embedClient.CreateEmbeddings(ctx, request) })
		if err != nil {
			return nil, modelError(err)
		}
		promptTokens := result.Usage.PromptTokens
		if promptTokens == 0 {
			promptTokens = result.Usage.TotalTokens
		}
		if promptTokens == 0 {
			for _, text := range batch {
				promptTokens += utf8.RuneCountInString(text)
			}
		}
		observeUsage(ctx, promptTokens, 0)
		if len(result.Data) != len(batch) {
			return nil, fmt.Errorf("模型返回的向量数量不匹配")
		}
		ordered := make([][]float32, len(batch))
		for _, item := range result.Data {
			if item.Index < 0 || item.Index >= len(batch) || ordered[item.Index] != nil {
				return nil, fmt.Errorf("模型返回的向量索引或维度错误")
			}
			if len(item.Embedding) != o.dimension {
				return nil, fmt.Errorf("向量服务返回 %d 维，EMBEDDING_DIM=%d；请确认模型支持 dimensions 参数，停止服务后运行 scripts/reindex.sh 重建向量", len(item.Embedding), o.dimension)
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
		response, err := o.ttsClient.CreateSpeech(ctx, openai.CreateSpeechRequest{Model: openai.SpeechModel(o.ttsModel), Input: text, Voice: openai.SpeechVoice(voice), ResponseFormat: openai.SpeechResponseFormatMp3})
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
	var asrErr *asrHTTPError
	if errors.As(err, &asrErr) {
		return asrErr.status
	}
	var imageErr *imageHTTPError
	if errors.As(err, &imageErr) {
		return imageErr.status
	}
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
		if status == 429 {
			return "模型服务繁忙，请稍后重试"
		}
		if status >= 500 {
			return "模型服务暂时不可用，请稍后重试"
		}
		return fmt.Sprintf("模型服务请求失败（HTTP %d），请联系管理员检查配置", status)
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
