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
	JSON        bool
}

type Client interface {
	Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error)
	ChatStream(ctx context.Context, messages []Message, opts ChatOptions, onDelta func(string) error) (string, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	TTS(ctx context.Context, text, voice string) ([]byte, error)
	Name() string
}

func New(c config.Config) Client {
	c = c.Resolved()
	fake := NewFake(c.EmbeddingDim)
	real := NewOpenAI(c)
	var chat, embed, tts Client = fake, fake, fake
	if c.ChatAPIKey != "" {
		chat = real
	}
	if c.EmbedAPIKey != "" {
		embed = real
	}
	if c.TTSAPIKey != "" && c.TTSModel != "" {
		tts = real
	}
	var asr Transcriber = &FakeTranscriber{}
	if c.ASRBaseURL != "" && c.ASRAPIKey != "" {
		asr = newASR(c)
	}
	return &Capabilities{chat: chat, embed: embed, tts: tts, asr: asr, config: c}
}

type Capabilities struct {
	chat, embed, tts Client
	asr              Transcriber
	config           config.Config
}

func (c *Capabilities) Name() string {
	if c.chat.Name() == "fake" && c.embed.Name() == "fake" && c.tts.Name() == "fake" && !c.ASREnabled() {
		return "fake"
	}
	return "multi-provider"
}
func (c *Capabilities) Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	opts.Model = c.model(opts.Model)
	return c.chat.Chat(ctx, messages, opts)
}
func (c *Capabilities) ChatStream(ctx context.Context, messages []Message, opts ChatOptions, delta func(string) error) (string, error) {
	opts.Model = c.model(opts.Model)
	return c.chat.ChatStream(ctx, messages, opts, delta)
}
func (c *Capabilities) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return c.embed.Embed(ctx, texts)
}
func (c *Capabilities) TTS(ctx context.Context, text, voice string) ([]byte, error) {
	return c.tts.TTS(ctx, text, voice)
}
func (c *Capabilities) model(value string) string {
	for _, id := range c.config.ChatModels {
		if value == id {
			return value
		}
	}
	return c.config.ChatModel
}

func (c *Capabilities) TTSEnabled() bool { return c.tts.Name() != "fake" }
func TTSEnabled(client Client) bool {
	if value, ok := client.(interface{ TTSEnabled() bool }); ok {
		return value.TTSEnabled()
	}
	return client.Name() != "fake"
}
