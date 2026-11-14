package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Environment   string
	Port          int
	DatabaseURL   string
	JWTSecret     string
	DataDir       string
	LLMBaseURL    string
	LLMAPIKey     string
	LLMChatModel  string
	LLMEmbedModel string
	EmbeddingDim  int
	LLMTTSModel   string
	LLMTTSVoices  []string
	CORSOrigins   []string
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

func split(value string) []string {
	values := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
