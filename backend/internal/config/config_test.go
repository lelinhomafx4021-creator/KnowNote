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
	for _, key := range []string{"ASR_BASE_URL", "ASR_API_KEY", "ASR_MODEL"} {
		t.Setenv(key, "")
	}
	for _, key := range []string{"ADMIN_EMAILS", "ADMIN_PASSWORD", "SETTINGS_ENCRYPTION_KEY", "CHAT_BASE_URL", "CHAT_API_KEY", "CHAT_MODEL", "CHAT_MODELS", "EMBED_BASE_URL", "EMBED_API_KEY", "EMBED_MODEL", "TTS_BASE_URL", "TTS_API_KEY", "TTS_MODEL", "TTS_VOICES", "IMAGE_PROVIDER", "IMAGE_BASE_URL", "IMAGE_API_KEY", "IMAGE_MODEL", "QUOTA_DAILY_CHAT_REQUESTS", "QUOTA_DAILY_TOKENS", "LLM_MAX_CONCURRENCY", "APP_ENV", "PORT", "DATABASE_URL", "JWT_SECRET", "DATA_DIR", "LLM_BASE_URL", "LLM_API_KEY", "LLM_CHAT_MODEL", "LLM_EMBED_MODEL", "EMBEDDING_DIM", "LLM_TTS_MODEL", "LLM_TTS_VOICES", "CORS_ORIGINS", "SOFFICE_PATH", "STORAGE_DRIVER", "OSS_REGION", "OSS_ENDPOINT", "OSS_BUCKET", "OSS_ACCESS_KEY_ID", "OSS_ACCESS_KEY_SECRET", "OSS_PREFIX"} {
		t.Setenv(key, "")
	}
}

func TestASRConfiguration(t *testing.T) {
	for _, test := range []struct {
		name, embedURL, asrURL, asrKey, model, wantURL, wantKey string
	}{
		{"百炼回退", "https://dashscope.aliyuncs.com/compatible-mode/v1", "", "", "", "https://dashscope.aliyuncs.com/compatible-mode/v1", "向量测试凭据"},
		{"业务空间回退", "https://space.cn-beijing.maas.aliyuncs.com/compatible-mode/v1", "", "", "", "https://space.cn-beijing.maas.aliyuncs.com/compatible-mode/v1", "向量测试凭据"},
		{"非百炼不回退", "https://embed.example/v1", "", "", "", "", ""},
		{"路径不参与域名判断", "https://embed.example/aliyuncs.com", "", "", "", "", ""},
		{"显式配置优先", "https://dashscope.aliyuncs.com/v1", "https://asr.example/v1", "识别测试凭据", "自定义识别模型", "https://asr.example/v1", "识别测试凭据"},
		{"仅地址回退", "https://dashscope.aliyuncs.com/v1", "", "识别测试凭据", "", "https://dashscope.aliyuncs.com/v1", "识别测试凭据"},
		{"仅密钥回退", "https://dashscope.aliyuncs.com/v1", "https://asr.example/v1", "", "", "https://asr.example/v1", "向量测试凭据"},
		{"非百炼只配密钥", "https://embed.example/v1", "", "识别测试凭据", "", "", "识别测试凭据"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cleanEnvironment(t)
			for key, value := range map[string]string{"EMBED_BASE_URL": test.embedURL, "EMBED_API_KEY": "向量测试凭据", "ASR_BASE_URL": test.asrURL, "ASR_API_KEY": test.asrKey, "ASR_MODEL": test.model, "JWT_SECRET": "配置测试"} {
				t.Setenv(key, value)
			}
			c, err := Load(nil)
			if err != nil {
				t.Fatal(err)
			}
			if c.ASRBaseURL != test.wantURL || c.ASRAPIKey != test.wantKey {
				t.Fatal("ASR 配置回退不符合约定")
			}
			wantModel := test.model
			if wantModel == "" {
				wantModel = "qwen3-asr-flash"
			}
			if c.ASRModel != wantModel {
				t.Fatal("ASR 模型默认值或覆盖错误")
			}
		})
	}
	for _, value := range []string{"file:///audio", "https://user:password@asr.example/v1", "https://asr.example/v1?key=test"} {
		t.Run("地址校验"+value, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv("ASR_BASE_URL", value)
			if _, err := Load(nil); err == nil {
				t.Fatal("无效语音识别地址未被拒绝")
			}
		})
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

func TestStorageConfiguration(t *testing.T) {
	cleanEnvironment(t)
	c, err := Load(nil)
	if err != nil || c.StorageDriver != "local" || c.OSSPrefix != "knownote/" {
		t.Fatal("存储默认配置不符")
	}
	t.Setenv("STORAGE_DRIVER", "oss")
	if _, err := Load(nil); err == nil {
		t.Fatal("未配置 OSS 凭据必须拒绝启动")
	}
	for key, value := range map[string]string{"OSS_REGION": "cn-hangzhou", "OSS_BUCKET": "test-bucket", "OSS_ACCESS_KEY_ID": "test-id", "OSS_ACCESS_KEY_SECRET": "test-secret", "OSS_ENDPOINT": "https://oss-cn-hangzhou.aliyuncs.com", "SOFFICE_PATH": "/test/soffice"} {
		t.Setenv(key, value)
	}
	c, err = Load(nil)
	if err != nil || c.StorageDriver != "oss" || c.SofficePath != "/test/soffice" {
		t.Fatal("OSS 配置覆盖失败")
	}
	for _, test := range []struct{ key, value string }{{"STORAGE_DRIVER", "unknown"}, {"OSS_PREFIX", "../escape"}, {"OSS_ENDPOINT", "https://user:pass@example.com"}} {
		t.Run(test.key, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			if _, err := Load(nil); err == nil {
				t.Fatal("无效存储配置未拒绝")
			}
		})
	}
}

func TestCapabilityFallbackAndOverrides(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("LLM_API_KEY", "legacy-test-only")
	t.Setenv("LLM_BASE_URL", "https://legacy.example/v1")
	t.Setenv("LLM_CHAT_MODEL", "legacy-chat")
	t.Setenv("LLM_EMBED_MODEL", "legacy-embed")
	t.Setenv("LLM_TTS_MODEL", "legacy-tts")
	c, err := Load(nil)
	if err != nil || c.ChatAPIKey != "legacy-test-only" || c.EmbedAPIKey != c.ChatAPIKey || c.TTSAPIKey != c.ChatAPIKey || c.ChatModel != "legacy-chat" || c.EmbedModel != "legacy-embed" || c.TTSModel != "legacy-tts" || c.ChatBaseURL != c.EmbedBaseURL {
		t.Fatal("旧配置未按能力回退")
	}
	t.Setenv("CHAT_BASE_URL", "https://api.deepseek.com")
	t.Setenv("CHAT_API_KEY", "chat-test-only")
	t.Setenv("CHAT_MODEL", "deepseek-flash")
	t.Setenv("CHAT_MODELS", "deepseek-v4-pro, deepseek-flash, ,deepseek-v4-pro")
	t.Setenv("EMBED_BASE_URL", "https://embedding.example/v1")
	t.Setenv("EMBED_MODEL", "text-embedding-v4")
	t.Setenv("TTS_VOICES", "nova,echo")
	t.Setenv("QUOTA_DAILY_CHAT_REQUESTS", "0")
	c, err = Load(nil)
	if err != nil || c.ChatAPIKey != "chat-test-only" || c.EmbedAPIKey != "legacy-test-only" || c.EmbedModel != "text-embedding-v4" || len(c.ChatModels) != 2 || len(c.TTSVoices) != 2 || c.QuotaDailyChatRequests != 0 || c.QuotaDailyTokens != 200000 || c.LLMMaxConcurrency != 8 {
		t.Fatal("独立配置覆盖或默认额度错误")
	}
	for _, item := range []struct{ key, value string }{{"CHAT_BASE_URL", "https://test.example?key=forbidden"}, {"EMBED_BASE_URL", "ftp://test.example"}, {"TTS_BASE_URL", "https://name:password@test.example"}, {"IMAGE_PROVIDER", "other"}, {"QUOTA_DAILY_TOKENS", "-1"}, {"LLM_MAX_CONCURRENCY", "0"}} {
		t.Run(item.key, func(t *testing.T) {
			t.Setenv(item.key, item.value)
			if _, err := Load(nil); err == nil {
				t.Fatal("未拒绝无效能力配置")
			}
		})
	}
}

func TestDeepSeekDefaultModel(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("CHAT_BASE_URL", "https://api.deepseek.com")
	t.Setenv("CHAT_MODEL", " ")
	c, err := Load(nil)
	if err != nil || c.ChatModel != "deepseek-flash" {
		t.Fatal("DeepSeek 默认模型错误")
	}
	t.Setenv("CHAT_BASE_URL", "https://example.com/api.deepseek.com")
	c, err = Load(nil)
	if err != nil || c.ChatModel != "gpt-4o-mini" {
		t.Fatal("应按域名判断提供方")
	}
}

func TestAdminPasswordValidation(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("ADMIN_PASSWORD", "足够长的管理员口令-1234")
	if _, err := Load(nil); err == nil {
		t.Fatal("只配置口令而没有管理员邮箱应报错")
	}
	t.Setenv("ADMIN_EMAILS", "Admin@Example.test, other@example.test")
	c, err := Load(nil)
	if err != nil || c.AdminPassword != "足够长的管理员口令-1234" || c.AdminEmails[0] != "admin@example.test" {
		t.Fatalf("应读取默认管理员配置: %v", err)
	}
	t.Setenv("ADMIN_PASSWORD", "太短了")
	if _, err := Load(nil); err == nil {
		t.Fatal("过短的管理员口令应报错")
	}
}
