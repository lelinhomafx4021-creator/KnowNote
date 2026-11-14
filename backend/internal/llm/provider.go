package llm

import (
	"context"
	"knownote/internal/config"
	"sync/atomic"
)

// Provider 原子发布不可变快照。一次调用持有旧快照至结束，不关闭进行中的流。
type Provider struct{ current atomic.Pointer[Snapshot] }
type Snapshot struct {
	Client Client
	Config config.Config
}

func NewProvider(c config.Config) *Provider {
	p := &Provider{}
	p.Replace(New(c), c)
	return p
}
func (p *Provider) Replace(client Client, c config.Config) {
	c = c.Resolved()
	c.ChatModels = append([]string(nil), c.ChatModels...)
	p.current.Store(&Snapshot{client, c})
}
func (p *Provider) Current() Snapshot { return *p.current.Load() }
func (p *Provider) Name() string      { return p.Current().Client.Name() }
func (p *Provider) Chat(ctx context.Context, m []Message, o ChatOptions) (string, error) {
	return p.Current().Client.Chat(ctx, m, o)
}
func (p *Provider) ChatStream(ctx context.Context, m []Message, o ChatOptions, d func(string) error) (string, error) {
	return p.Current().Client.ChatStream(ctx, m, o, d)
}
func (p *Provider) Embed(ctx context.Context, t []string) ([][]float32, error) {
	return p.Current().Client.Embed(ctx, t)
}
func (p *Provider) TTS(ctx context.Context, t, v string) ([]byte, error) {
	return p.Current().Client.TTS(ctx, t, v)
}
func (p *Provider) TTSEnabled() bool { return TTSEnabled(p.Current().Client) }
