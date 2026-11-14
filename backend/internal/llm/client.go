package llm

import (
	"context"
	"errors"

	"knownote/internal/config"
)

var ErrNotConfigured = errors.New("未配置语音合成模型")

const EmbedBatchSize = 64

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatOptions struct {
	Model       string
	Temperature float32
	MaxTokens   int
}

type Client interface {
	Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error)
	ChatStream(ctx context.Context, messages []Message, opts ChatOptions, onDelta func(string) error) (string, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	TTS(ctx context.Context, text, voice string) ([]byte, error)
	Name() string
}

func New(c config.Config) Client {
	if c.LLMAPIKey == "" {
		return NewFake(c.EmbeddingDim)
	}
	return NewOpenAI(c)
}
