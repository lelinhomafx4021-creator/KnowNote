package llm

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"knownote/internal/config"
)

type ChatModel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}
type ModelCatalog struct {
	Chat  []ChatModel `json:"chat"`
	Embed struct {
		Model     string `json:"model"`
		Dimension int    `json:"dimension"`
		Provider  string `json:"provider"`
	} `json:"embed"`
	TTS struct {
		Enabled bool `json:"enabled"`
	} `json:"tts"`
	Image struct {
		Enabled  bool   `json:"enabled"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	} `json:"image"`
}

func ProviderDomain(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
func Catalog(c config.Config) ModelCatalog {
	c = c.Resolved()
	result := ModelCatalog{Chat: []ChatModel{}}
	for _, id := range c.ChatModels {
		name := id
		if id == "deepseek-flash" {
			name = "DeepSeek Flash"
		}
		if id == "deepseek-v4-pro" {
			name = "DeepSeek V4 Pro"
		}
		provider := ProviderDomain(c.ChatBaseURL)
		if c.ChatAPIKey == "" {
			provider = "fake"
		}
		result.Chat = append(result.Chat, ChatModel{id, name, provider})
	}
	result.Embed.Model, result.Embed.Dimension, result.Embed.Provider = c.EmbedModel, c.EmbeddingDim, ProviderDomain(c.EmbedBaseURL)
	if c.EmbedAPIKey == "" {
		result.Embed.Provider = "fake"
	}
	result.TTS.Enabled = c.TTSAPIKey != "" && c.TTSModel != ""
	result.Image.Enabled, result.Image.Provider, result.Image.Model = c.ImageProvider != "", c.ImageProvider, c.ImageModel
	return result
}
func LogCapabilities(logger *slog.Logger, c config.Config) {
	c = c.Resolved()
	for _, entry := range []struct {
		ability, base, model string
		real                 bool
	}{{"对话", c.ChatBaseURL, c.ChatModel, c.ChatAPIKey != ""}, {"向量", c.EmbedBaseURL, c.EmbedModel, c.EmbedAPIKey != ""}, {"语音", c.TTSBaseURL, c.TTSModel, c.TTSAPIKey != "" && c.TTSModel != ""}, {"图像", c.ImageBaseURL, c.ImageModel, c.ImageProvider != "" && c.ImageAPIKey != ""}} {
		mode := "Fake"
		if entry.real {
			mode = "真实"
		}
		if entry.ability == "图像" && c.ImageProvider == "" {
			mode = "关闭"
		}
		logger.Info("模型能力配置", "能力", entry.ability, "提供方域名", ProviderDomain(entry.base), "模型", entry.model, "模式", mode)
	}
}

// ProbeEmbedding 仅在配置真实向量能力后调用；不归属用户，也不计入用户额度。
func ProbeEmbedding(ctx context.Context, client Client, c config.Config) error {
	c = c.Resolved()
	if c.EmbedAPIKey == "" {
		return nil
	}
	vectors, err := client.Embed(ctx, []string{"知页向量维度探测"})
	if err != nil {
		return fmt.Errorf("启动向量探测失败：%s；请检查 EMBED_MODEL、EMBEDDING_DIM 与服务地址，修改维度后停止服务并运行 scripts/reindex.sh", err.Error())
	}
	if len(vectors) != 1 || len(vectors[0]) != c.EmbeddingDim {
		return fmt.Errorf("启动向量探测维度不一致，要求 %d 维；停止服务并运行 scripts/reindex.sh 重建向量", c.EmbeddingDim)
	}
	return nil
}
func AllowedModel(catalog ModelCatalog, id string) bool {
	for _, item := range catalog.Chat {
		if strings.TrimSpace(id) == item.ID {
			return true
		}
	}
	return false
}
