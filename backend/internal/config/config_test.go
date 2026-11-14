package config

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func cleanEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_ENV", "PORT", "DATABASE_URL", "JWT_SECRET", "DATA_DIR", "LLM_BASE_URL", "LLM_API_KEY", "LLM_CHAT_MODEL", "LLM_EMBED_MODEL", "EMBEDDING_DIM", "LLM_TTS_MODEL", "LLM_TTS_VOICES", "CORS_ORIGINS"} {
		t.Setenv(key, "")
	}
}

func TestDefaultsAndTemporarySecret(t *testing.T) {
	cleanEnvironment(t)
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	c, err := Load(logger)
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 8080 || c.DatabaseURL != "postgres://knownote:knownote@localhost:5432/knownote?sslmode=disable" || c.DataDir != "./data" || c.EmbeddingDim != 1536 || c.LLMChatModel != "gpt-4o-mini" || c.LLMEmbedModel != "text-embedding-3-small" || c.LLMBaseURL != "https://api.openai.com/v1" || c.LLMAPIKey != "" || c.LLMTTSModel != "" {
		t.Fatal("默认配置不符合约定")
	}
	if len(c.JWTSecret) != 64 || !strings.Contains(output.String(), "临时开发密钥") || strings.Contains(output.String(), c.JWTSecret) {
		t.Fatal("临时密钥或警告不符合约定")
	}
	second, err := Load(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil || second.JWTSecret == c.JWTSecret {
		t.Fatal("临时密钥应使用随机值")
	}
	if strings.Join(c.LLMTTSVoices, ",") != "alloy,echo" || strings.Join(c.CORSOrigins, ",") != "http://localhost:5173" {
		t.Fatal("列表默认值错误")
	}
}

func TestOverridesAndValidation(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("PORT", "8081")
	t.Setenv("EMBEDDING_DIM", "512")
	t.Setenv("JWT_SECRET", "测试专用密钥")
	t.Setenv("LLM_TTS_VOICES", " alloy, echo, ,nova ")
	c, err := Load(nil)
	if err != nil || c.Port != 8081 || c.EmbeddingDim != 512 || c.JWTSecret != "测试专用密钥" || len(c.LLMTTSVoices) != 3 {
		t.Fatal("环境变量覆盖失败")
	}
	for _, test := range []struct{ key, value string }{
		{"PORT", "abc"}, {"PORT", "65536"}, {"EMBEDDING_DIM", "0"}, {"EMBEDDING_DIM", "2001"},
		{"APP_ENV", "unknown"}, {"LLM_BASE_URL", "file:///tmp/example"}, {"CORS_ORIGINS", "http://localhost:5173/path"},
	} {
		t.Run(test.key+test.value, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			if _, err := Load(nil); err == nil {
				t.Fatal("无效配置应返回错误")
			}
		})
	}
}

func TestProductionRequiresSecret(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("APP_ENV", "production")
	if _, err := Load(nil); err == nil {
		t.Fatal("生产环境不应自动生成密钥")
	}
	t.Setenv("JWT_SECRET", "short")
	if _, err := Load(nil); err == nil {
		t.Fatal("生产环境不应接受短密钥")
	}
	t.Setenv("JWT_SECRET", strings.Repeat("a", 32))
	if _, err := Load(nil); err != nil {
		t.Fatal(err)
	}
}
