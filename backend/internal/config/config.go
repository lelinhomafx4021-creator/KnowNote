package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
)

type Config struct {
	ChatBaseURL, ChatAPIKey, ChatModel                                               string
	ChatModels                                                                       []string
	EmbedBaseURL, EmbedAPIKey, EmbedModel                                            string
	TTSBaseURL, TTSAPIKey, TTSModel                                                  string
	TTSVoices                                                                        []string
	ImageProvider, ImageBaseURL, ImageAPIKey, ImageModel                             string
	QuotaDailyChatRequests, QuotaDailyTokens, LLMMaxConcurrency                      int
	SofficePath                                                                      string
	StorageDriver                                                                    string
	OSSRegion, OSSEndpoint, OSSBucket, OSSAccessKeyID, OSSAccessKeySecret, OSSPrefix string
	Environment                                                                      string
	Port                                                                             int
	DatabaseURL                                                                      string
	JWTSecret                                                                        string
	DataDir                                                                          string
	LLMBaseURL                                                                       string
	LLMAPIKey                                                                        string
	LLMChatModel                                                                     string
	LLMEmbedModel                                                                    string
	EmbeddingDim                                                                     int
	LLMTTSModel                                                                      string
	LLMTTSVoices                                                                     []string
	CORSOrigins                                                                      []string
}

// Load 只读取进程环境变量，避免隐式读取本地密钥文件。
func Load(logger *slog.Logger) (Config, error) {
	if logger == nil {
		logger = slog.Default()
	}
	get := func(key, fallback string) string {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
		return fallback
	}
	c := Config{
		SofficePath: get("SOFFICE_PATH", ""), StorageDriver: get("STORAGE_DRIVER", "local"),
		OSSRegion: get("OSS_REGION", ""), OSSEndpoint: get("OSS_ENDPOINT", ""), OSSBucket: get("OSS_BUCKET", ""),
		OSSAccessKeyID: get("OSS_ACCESS_KEY_ID", ""), OSSAccessKeySecret: get("OSS_ACCESS_KEY_SECRET", ""), OSSPrefix: get("OSS_PREFIX", "knownote/"),
		Environment: get("APP_ENV", "development"),
		DatabaseURL: get("DATABASE_URL", "postgres://knownote:knownote@localhost:5432/knownote?sslmode=disable"),
		JWTSecret:   os.Getenv("JWT_SECRET"), DataDir: get("DATA_DIR", "./data"),
		LLMBaseURL:    get("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:     strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		LLMChatModel:  get("LLM_CHAT_MODEL", "gpt-4o-mini"),
		LLMEmbedModel: get("LLM_EMBED_MODEL", "text-embedding-3-small"),
		LLMTTSModel:   strings.TrimSpace(os.Getenv("LLM_TTS_MODEL")),
		LLMTTSVoices:  split(get("LLM_TTS_VOICES", "alloy,echo")),
		CORSOrigins:   split(get("CORS_ORIGINS", "http://localhost:5173")),
	}
	c.ChatBaseURL, c.ChatAPIKey, c.ChatModel = get("CHAT_BASE_URL", c.LLMBaseURL), get("CHAT_API_KEY", c.LLMAPIKey), get("CHAT_MODEL", c.LLMChatModel)
	chatURL, _ := url.Parse(c.ChatBaseURL)
	if chatURL != nil && chatURL.Hostname() == "api.deepseek.com" && strings.TrimSpace(os.Getenv("CHAT_MODEL")) == "" && strings.TrimSpace(os.Getenv("LLM_CHAT_MODEL")) == "" {
		c.ChatModel = "deepseek-flash"
	}
	c.ChatModels = split(get("CHAT_MODELS", c.ChatModel))
	c.EmbedBaseURL, c.EmbedAPIKey, c.EmbedModel = get("EMBED_BASE_URL", c.LLMBaseURL), get("EMBED_API_KEY", c.LLMAPIKey), get("EMBED_MODEL", c.LLMEmbedModel)
	c.TTSBaseURL, c.TTSAPIKey, c.TTSModel = get("TTS_BASE_URL", c.LLMBaseURL), get("TTS_API_KEY", c.LLMAPIKey), get("TTS_MODEL", c.LLMTTSModel)
	c.TTSVoices = split(get("TTS_VOICES", strings.Join(c.LLMTTSVoices, ",")))
	c.ImageProvider, c.ImageBaseURL, c.ImageAPIKey, c.ImageModel = get("IMAGE_PROVIDER", ""), get("IMAGE_BASE_URL", c.LLMBaseURL), get("IMAGE_API_KEY", c.LLMAPIKey), get("IMAGE_MODEL", "")
	if c.ImageProvider != "" && c.ImageProvider != "openai" && c.ImageProvider != "dashscope" {
		return Config{}, fmt.Errorf("IMAGE_PROVIDER 只支持 openai、dashscope 或留空关闭")
	}
	for _, item := range []struct {
		key      string
		target   *int
		fallback string
	}{{"QUOTA_DAILY_CHAT_REQUESTS", &c.QuotaDailyChatRequests, "200"}, {"QUOTA_DAILY_TOKENS", &c.QuotaDailyTokens, "200000"}, {"LLM_MAX_CONCURRENCY", &c.LLMMaxConcurrency, "8"}} {
		value, err := strconv.Atoi(get(item.key, item.fallback))
		if err != nil || value < 0 || (item.key == "LLM_MAX_CONCURRENCY" && value < 1) {
			return Config{}, fmt.Errorf("%s 必须为有效的非负整数，并发上限至少为 1", item.key)
		}
		*item.target = value
	}
	c = c.Resolved()
	for key, value := range map[string]string{"CHAT_BASE_URL": c.ChatBaseURL, "EMBED_BASE_URL": c.EmbedBaseURL, "TTS_BASE_URL": c.TTSBaseURL, "IMAGE_BASE_URL": c.ImageBaseURL} {
		u, err := url.Parse(value)
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
			return Config{}, fmt.Errorf("%s 必须为有效的 HTTP 地址，不能包含凭据、查询参数或片段", key)
		}
	}
	if c.StorageDriver != "local" && c.StorageDriver != "oss" {
		return Config{}, fmt.Errorf("STORAGE_DRIVER 只支持 local 或 oss")
	}
	if c.StorageDriver == "oss" && (c.OSSRegion == "" || c.OSSBucket == "" || c.OSSAccessKeyID == "" || c.OSSAccessKeySecret == "") {
		return Config{}, fmt.Errorf("OSS 存储需要配置地域、Bucket 与访问凭据")
	}
	prefix := strings.Trim(c.OSSPrefix, "/")
	if prefix == "" || path.Clean(prefix) != prefix || strings.Contains(prefix, "..") || strings.ContainsAny(prefix, "\\\x00") {
		return Config{}, fmt.Errorf("OSS_PREFIX 必须是有效的对象键前缀")
	}
	if c.OSSEndpoint != "" {
		endpoint := c.OSSEndpoint
		if !strings.Contains(endpoint, "://") {
			endpoint = "https://" + endpoint
		}
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
			return Config{}, fmt.Errorf("OSS_ENDPOINT 必须是有效的服务地址")
		}
	}
	var err error
	c.Port, err = strconv.Atoi(get("PORT", "8080"))
	if err != nil || c.Port < 1 || c.Port > 65535 {
		return Config{}, fmt.Errorf("PORT 必须是 1 到 65535 的整数")
	}
	c.EmbeddingDim, err = strconv.Atoi(get("EMBEDDING_DIM", "1536"))
	// pgvector 的 vector HNSW 索引最多支持 2000 维。
	if err != nil || c.EmbeddingDim < 1 || c.EmbeddingDim > 2000 {
		return Config{}, fmt.Errorf("EMBEDDING_DIM 必须是 1 到 2000 的整数")
	}
	if c.Environment != "development" && c.Environment != "test" && c.Environment != "production" {
		return Config{}, fmt.Errorf("APP_ENV 只支持 development、test 或 production")
	}
	u, err := url.Parse(c.LLMBaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return Config{}, fmt.Errorf("LLM_BASE_URL 必须是有效的 HTTP 地址，且不能包含凭据")
	}
	for _, origin := range c.CORSOrigins {
		if origin == "*" {
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return Config{}, fmt.Errorf("CORS_ORIGINS 必须是逗号分隔的来源地址")
		}
	}
	if strings.TrimSpace(c.JWTSecret) == "" {
		if c.Environment == "production" {
			return Config{}, fmt.Errorf("生产环境必须配置 JWT_SECRET")
		}
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return Config{}, fmt.Errorf("生成开发 JWT 密钥失败")
		}
		c.JWTSecret = hex.EncodeToString(secret)
		logger.Warn("未配置 JWT_SECRET，已生成临时开发密钥；重启后现有登录会失效")
	}
	if c.Environment == "production" && len(c.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("生产环境 JWT_SECRET 至少需要 32 字节")
	}
	return c, nil
}

// Resolved 同时兼容旧调用方直接构造的 Config；空的新字段继承旧能力配置。
func (c Config) Resolved() Config {
	fallback := func(value, old string) string {
		if value != "" {
			return value
		}
		return old
	}
	c.ChatBaseURL, c.ChatAPIKey, c.ChatModel = fallback(c.ChatBaseURL, c.LLMBaseURL), fallback(c.ChatAPIKey, c.LLMAPIKey), fallback(c.ChatModel, c.LLMChatModel)
	c.EmbedBaseURL, c.EmbedAPIKey, c.EmbedModel = fallback(c.EmbedBaseURL, c.LLMBaseURL), fallback(c.EmbedAPIKey, c.LLMAPIKey), fallback(c.EmbedModel, c.LLMEmbedModel)
	c.TTSBaseURL, c.TTSAPIKey, c.TTSModel = fallback(c.TTSBaseURL, c.LLMBaseURL), fallback(c.TTSAPIKey, c.LLMAPIKey), fallback(c.TTSModel, c.LLMTTSModel)
	c.ImageBaseURL, c.ImageAPIKey = fallback(c.ImageBaseURL, c.LLMBaseURL), fallback(c.ImageAPIKey, c.LLMAPIKey)
	if len(c.TTSVoices) == 0 {
		c.TTSVoices = c.LLMTTSVoices
	}
	models := []string{c.ChatModel}
	for _, id := range c.ChatModels {
		if id != "" && !contains(models, id) {
			models = append(models, id)
		}
	}
	c.ChatModels = models
	return c
}
func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func split(value string) []string {
	values := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
