package llm

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"knownote/internal/config"
	"knownote/internal/model"
)

type userKey struct{}

func WithUser(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userKey{}, id)
}

type usageKey struct{}
type tokenUsage struct {
	prompt, completion int
	exact              bool
}

func observeUsage(ctx context.Context, prompt, completion int) {
	if value, ok := ctx.Value(usageKey{}).(*tokenUsage); ok && prompt+completion > 0 {
		value.prompt += prompt
		value.completion += completion
		value.exact = true
	}
}

type LimitError struct{ Message string }

func (e *LimitError) Error() string { return e.Message }

var ErrQuota = &LimitError{"今日额度已用完，明天再来"}
var ErrRate = &LimitError{"提问太频繁，请一分钟后再试"}
var ErrASRRate = &LimitError{"语音输入太频繁，请一分钟后再试"}
var ErrBusy = &LimitError{"当前使用人数较多，请稍后重试"}

// PublicError 只透传本模块自有的中文错误，其他异常统一脱敏。
func PublicError(err error) string {
	var limit *LimitError
	var upstream *upstreamError
	if errors.As(err, &limit) || errors.As(err, &upstream) {
		return err.Error()
	}
	return "模型生成失败，请稍后重试"
}

type UsageView struct {
	Date              string `json:"date"`
	ChatRequests      int64  `json:"chatRequests"`
	Tokens            int64  `json:"tokens"`
	ReservedTokens    int64  `json:"reservedTokens"`
	ChatRequestsLimit int    `json:"chatRequestsLimit"`
	TokensLimit       int    `json:"tokensLimit"`
}
type Guard struct {
	Client   Client
	Provider *Provider
	Limits   func() config.Config
	DB       *gorm.DB
	Config   config.Config
	slots    chan struct{}
}

func NewGuard(database *gorm.DB, client Client, c config.Config) *Guard {
	c = c.Resolved()
	g := &Guard{Client: client, DB: database, Config: c, slots: make(chan struct{}, max(1, c.LLMMaxConcurrency))}
	g.Provider, _ = client.(*Provider)
	if g.Provider != nil {
		g.Limits = func() config.Config { return g.Provider.Current().Config }
	}
	return g
}

// snapshot 在一次能力调用开始时固定客户端与额度，已建立的流不受替换影响。
func (g *Guard) snapshot() *Guard {
	if g.Provider == nil {
		return g
	}
	s := *g
	current := g.Provider.Current()
	s.Client, s.Config, s.Provider = current.Client, current.Config, nil
	return &s
}
func (g *Guard) Name() string { return g.Client.Name() }

var quotaZone = time.FixedZone("Asia/Shanghai", 8*60*60)

func dayStart(now time.Time) time.Time {
	local := now.In(quotaZone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, quotaZone)
}
func (g *Guard) Usage(ctx context.Context, userID string) (UsageView, error) {
	g = g.snapshot()
	start := dayStart(time.Now())
	var view UsageView
	err := g.DB.WithContext(ctx).Model(&model.LLMUsage{}).Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, start, start.AddDate(0, 0, 1)).Select("COALESCE(SUM(CASE WHEN capability IN ('chat','asr') THEN 1 ELSE 0 END),0) AS chat_requests, COALESCE(SUM(prompt_tokens + completion_tokens),0) AS tokens, COALESCE(SUM(reserved_tokens),0) AS reserved_tokens").Scan(&view).Error
	view.Date = start.Format("2006-01-02")
	limits := g.Config
	if g.Limits != nil {
		limits = g.Limits()
	}
	view.ChatRequestsLimit, view.TokensLimit = limits.QuotaDailyChatRequests, limits.QuotaDailyTokens
	return view, err
}
func (g *Guard) begin(ctx context.Context, ability, modelName string, reserve int) (model.LLMUsage, error) {
	userID, _ := ctx.Value(userKey{}).(string)
	if userID == "" {
		return model.LLMUsage{}, fmt.Errorf("模型调用缺少用户归属")
	}
	// 模板准备和重建仍占用全局并发槽，但不计入用户额度或用量。
	if userID == model.SystemUserID {
		var count int64
		err := g.DB.WithContext(ctx).Model(&model.User{}).Where("id = ? AND is_system = true", userID).Count(&count).Error
		if err != nil {
			return model.LLMUsage{}, err
		}
		if count == 1 {
			return model.LLMUsage{}, nil
		}
	}
	item := model.LLMUsage{UserID: userID, Capability: ability, Model: modelName, Pending: true, ReservedTokens: reserve}
	err := g.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 原值更新取得用户行写锁，串行检查与预留；兼容 PostgreSQL 与 SQLite。
		locked := tx.Model(&model.User{}).Where("id = ?", userID).UpdateColumn("updated_at", gorm.Expr("updated_at"))
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected != 1 {
			return fmt.Errorf("模型调用用户不存在")
		}
		// 异常退出的调用按预留值计费；常规调用均在 90 秒内结束。
		if err := tx.Model(&model.LLMUsage{}).Where("user_id = ? AND pending = true AND created_at < ?", userID, time.Now().Add(-5*time.Minute)).Updates(map[string]any{"pending": false, "prompt_tokens": gorm.Expr("reserved_tokens"), "reserved_tokens": 0}).Error; err != nil {
			return err
		}
		scoped := *g
		scoped.DB = tx
		used, err := scoped.Usage(ctx, userID)
		if err != nil {
			return err
		}
		if ((ability == "chat" || ability == "asr") && used.ChatRequestsLimit > 0 && used.ChatRequests >= int64(used.ChatRequestsLimit)) || (used.TokensLimit > 0 && used.Tokens+used.ReservedTokens+int64(reserve) > int64(used.TokensLimit)) {
			return ErrQuota
		}
		if ability == "chat" || ability == "asr" {
			var count int64
			if err := tx.Model(&model.LLMUsage{}).Where("user_id = ? AND capability = ? AND created_at > ?", userID, ability, time.Now().Add(-time.Minute)).Count(&count).Error; err != nil {
				return err
			}
			if ability == "asr" && count >= 10 {
				return ErrASRRate
			}
			if ability == "chat" && count >= 20 {
				return ErrRate
			}
		}
		return tx.Omit("User").Create(&item).Error
	})
	return item, err
}
func (g *Guard) invoke(ctx context.Context, ability, modelName string, prompt, reserve int, call func(context.Context) (int, error)) (err error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case g.slots <- struct{}{}:
	default:
		return ErrBusy
	}
	defer func() { <-g.slots }()
	item, err := g.begin(ctx, ability, modelName, reserve)
	if err != nil {
		return err
	}
	started := time.Now()
	usage := &tokenUsage{}
	defer func() {
		if item.ID == "" {
			return
		}
		recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		updates := map[string]any{"prompt_tokens": usage.prompt, "completion_tokens": usage.completion, "duration_ms": time.Since(started).Milliseconds(), "success": err == nil, "pending": false, "reserved_tokens": 0}
		if saveErr := g.DB.WithContext(recordCtx).Model(&model.LLMUsage{}).Where("user_id = ? AND id = ?", item.UserID, item.ID).Updates(updates).Error; saveErr != nil {
			err = fmt.Errorf("用量记录失败，请稍后重试")
		}
	}()
	completion, err := call(context.WithValue(ctx, usageKey{}, usage))
	if !usage.exact {
		usage.prompt, usage.completion = prompt, completion
	}
	return err
}
func countMessages(messages []Message) int {
	count := 0
	for _, m := range messages {
		count += utf8.RuneCountInString(m.Content) + 4
	}
	return count
}
func (g *Guard) options(opts ChatOptions) ChatOptions {
	if !AllowedModel(Catalog(g.Config), opts.Model) {
		opts.Model = g.Config.ChatModel
	}
	return opts
}

// reserveTokens 只用于额度预占估算；不设置 MaxTokens 时请求不带上限，由模型自行决定输出长度。
func reserveTokens(opts ChatOptions) int {
	if opts.MaxTokens > 0 {
		return opts.MaxTokens
	}
	return 4096
}
func (g *Guard) Chat(ctx context.Context, messages []Message, opts ChatOptions) (out string, err error) {
	g = g.snapshot()
	opts = g.options(opts)
	prompt := countMessages(messages)
	err = g.invoke(ctx, "chat", opts.Model, prompt, prompt+reserveTokens(opts), func(ctx context.Context) (int, error) {
		var e error
		out, e = g.Client.Chat(ctx, messages, opts)
		return utf8.RuneCountInString(out), e
	})
	return
}
func (g *Guard) ChatStream(ctx context.Context, messages []Message, opts ChatOptions, delta func(string) error) (out string, err error) {
	g = g.snapshot()
	opts = g.options(opts)
	prompt := countMessages(messages)
	err = g.invoke(ctx, "chat", opts.Model, prompt, prompt+reserveTokens(opts), func(ctx context.Context) (int, error) {
		var e error
		out, e = g.Client.ChatStream(ctx, messages, opts, delta)
		return utf8.RuneCountInString(out), e
	})
	return
}
func (g *Guard) Embed(ctx context.Context, texts []string) (out [][]float32, err error) {
	g = g.snapshot()
	prompt := 0
	for _, text := range texts {
		prompt += utf8.RuneCountInString(text)
	}
	err = g.invoke(ctx, "embed", g.Config.EmbedModel, prompt, prompt, func(ctx context.Context) (int, error) { var e error; out, e = g.Client.Embed(ctx, texts); return 0, e })
	return
}
func (g *Guard) TTS(ctx context.Context, text, voice string) (out []byte, err error) {
	g = g.snapshot()
	prompt := utf8.RuneCountInString(text)
	err = g.invoke(ctx, "tts", g.Config.TTSModel, prompt, prompt, func(ctx context.Context) (int, error) {
		var e error
		out, e = g.Client.TTS(ctx, text, voice)
		return 0, e
	})
	return
}

func (g *Guard) TTSEnabled() bool { return TTSEnabled(g.Client) }
