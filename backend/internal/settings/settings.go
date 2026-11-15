// Package settings 管理有限的数据库配置覆盖，不读取本地配置文件。
package settings

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"knownote/internal/config"
	"knownote/internal/llm"
	"knownote/internal/model"
	"knownote/internal/storage"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const Clear = "__CLEAR__"

var Keys = []string{
	"CHAT_BASE_URL", "CHAT_API_KEY", "CHAT_MODEL", "CHAT_MODELS",
	"EMBED_BASE_URL", "EMBED_API_KEY", "EMBED_MODEL",
	"STORAGE_DRIVER", "OSS_REGION", "OSS_ENDPOINT", "OSS_BUCKET", "OSS_ACCESS_KEY_ID", "OSS_ACCESS_KEY_SECRET", "OSS_PREFIX",
	"QUOTA_DAILY_CHAT_REQUESTS", "QUOTA_DAILY_TOKENS",
}

type Item struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Source string `json:"source"`
}
type View struct {
	Chat            []Item `json:"chat"`
	Embed           []Item `json:"embed"`
	Storage         []Item `json:"storage"`
	Quota           []Item `json:"quota"`
	ReindexRequired bool   `json:"reindexRequired"`
	Message         string `json:"message"`
}
type TestResult struct {
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latencyMs"`
	Message   string `json:"message"`
}
type Manager struct {
	DB              *gorm.DB
	Base            config.Config
	Provider        *llm.Provider
	Logger          *slog.Logger
	mu              sync.Mutex
	aead            cipher.AEAD
	reindexRequired bool
	// VectorBusy 由单进程重建器提供；防止重建期间切换向量空间。
	VectorBusy func() bool
	// OnStorageChange 在存储配置保存成功后热切换运行期存储实例。
	OnStorageChange func(context.Context, config.Config) error
}

func New(database *gorm.DB, base config.Config, logger *slog.Logger) (*Manager, error) {
	if logger == nil {
		logger = slog.Default()
	}
	material := base.SettingsEncryptionKey
	if material == "" {
		material = base.JWTSecret
	}
	key := sha256.Sum256([]byte(material))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	m := &Manager{DB: database, Base: base.Resolved(), Logger: logger, aead: aead}
	c, _, err := m.resolve(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	m.Provider = llm.NewProvider(c)
	return m, nil
}
func (m *Manager) Encrypt(value string) (string, error) {
	nonce := make([]byte, m.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("加密配置失败")
	}
	return base64.StdEncoding.EncodeToString(m.aead.Seal(nonce, nonce, []byte(value), []byte("knownote-settings-v1"))), nil
}
func (m *Manager) Decrypt(value string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(data) < m.aead.NonceSize() {
		return "", fmt.Errorf("解密配置失败")
	}
	plain, err := m.aead.Open(nil, data[:m.aead.NonceSize()], data[m.aead.NonceSize():], []byte("knownote-settings-v1"))
	if err != nil {
		return "", fmt.Errorf("解密配置失败")
	}
	return string(plain), nil
}
func Mask(value string) string {
	r := []rune(value)
	if len(r) == 0 {
		return ""
	}
	if len(r) <= 7 {
		return "****"
	}
	return string(r[:3]) + "****" + string(r[len(r)-4:])
}
func secret(key string) bool {
	return key == "CHAT_API_KEY" || key == "EMBED_API_KEY" || key == "OSS_ACCESS_KEY_ID" || key == "OSS_ACCESS_KEY_SECRET"
}
func values(c config.Config) map[string]string {
	return map[string]string{
		"CHAT_BASE_URL":             c.ChatBaseURL,
		"CHAT_API_KEY":              c.ChatAPIKey,
		"CHAT_MODEL":                c.ChatModel,
		"CHAT_MODELS":               strings.Join(c.ChatModels, ","),
		"EMBED_BASE_URL":            c.EmbedBaseURL,
		"EMBED_API_KEY":             c.EmbedAPIKey,
		"EMBED_MODEL":               c.EmbedModel,
		"STORAGE_DRIVER":            c.StorageDriver,
		"OSS_REGION":                c.OSSRegion,
		"OSS_ENDPOINT":              c.OSSEndpoint,
		"OSS_BUCKET":                c.OSSBucket,
		"OSS_ACCESS_KEY_ID":         c.OSSAccessKeyID,
		"OSS_ACCESS_KEY_SECRET":     c.OSSAccessKeySecret,
		"OSS_PREFIX":                c.OSSPrefix,
		"QUOTA_DAILY_CHAT_REQUESTS": strconv.Itoa(c.QuotaDailyChatRequests),
		"QUOTA_DAILY_TOKENS":        strconv.Itoa(c.QuotaDailyTokens),
	}
}
func apply(c config.Config, v map[string]string) config.Config {
	c.ChatBaseURL, c.ChatAPIKey, c.ChatModel = v["CHAT_BASE_URL"], v["CHAT_API_KEY"], v["CHAT_MODEL"]
	c.ChatModels = nil
	for _, id := range strings.Split(v["CHAT_MODELS"], ",") {
		if id = strings.TrimSpace(id); id != "" {
			c.ChatModels = append(c.ChatModels, id)
		}
	}
	c.EmbedBaseURL, c.EmbedAPIKey, c.EmbedModel = v["EMBED_BASE_URL"], v["EMBED_API_KEY"], v["EMBED_MODEL"]
	c.StorageDriver = strings.ToLower(strings.TrimSpace(v["STORAGE_DRIVER"]))
	c.OSSRegion = strings.TrimSpace(v["OSS_REGION"])
	c.OSSEndpoint = strings.TrimSpace(v["OSS_ENDPOINT"])
	c.OSSBucket = strings.TrimSpace(v["OSS_BUCKET"])
	c.OSSAccessKeyID = strings.TrimSpace(v["OSS_ACCESS_KEY_ID"])
	c.OSSAccessKeySecret = strings.TrimSpace(v["OSS_ACCESS_KEY_SECRET"])
	c.OSSPrefix = strings.TrimSpace(v["OSS_PREFIX"])
	c.QuotaDailyChatRequests, _ = strconv.Atoi(v["QUOTA_DAILY_CHAT_REQUESTS"])
	c.QuotaDailyTokens, _ = strconv.Atoi(v["QUOTA_DAILY_TOKENS"])
	return c.Resolved()
}

// resolve 空字符串忽略；清除覆盖值后直接使用启动环境快照。
func (m *Manager) resolve(ctx context.Context, patch map[string]string) (config.Config, map[string]string, error) {
	var rows []model.SystemSetting
	if err := m.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return config.Config{}, nil, fmt.Errorf("读取系统配置失败，请稍后重试")
	}
	v := values(m.Base)
	sources := map[string]string{}
	for _, key := range Keys {
		sources[key] = "default"
		if m.Base.SettingSources[key] == "env" {
			sources[key] = "env"
		}
	}
	for _, row := range rows {
		if _, ok := v[row.Key]; !ok {
			continue
		}
		value := row.Value
		if secret(row.Key) {
			var err error
			value, err = m.Decrypt(value)
			if err != nil {
				m.Logger.Warn("模型密钥解密失败，已回退到环境配置", "配置项", row.Key)
				continue
			}
		}
		v[row.Key] = value
		sources[row.Key] = "database"
	}
	base := values(m.Base)
	for key, value := range patch {
		if _, ok := v[key]; !ok {
			return config.Config{}, nil, fmt.Errorf("包含不支持的配置项")
		}
		if value == "" {
			continue
		}
		if value == Clear {
			v[key] = base[key]
			sources[key] = m.Base.SettingSources[key]
			if sources[key] != "env" {
				sources[key] = "default"
			}
			continue
		}
		v[key] = strings.TrimSpace(value)
		sources[key] = "database"
	}
	// 未单独配置模型列表时，列表随默认模型变化。
	if sources["CHAT_MODELS"] != "database" && m.Base.SettingSources["CHAT_MODELS"] != "env" && v["CHAT_MODELS"] == m.Base.ChatModel {
		v["CHAT_MODELS"] = v["CHAT_MODEL"]
	}
	c := apply(m.Base, v)
	return c, sources, validate(c)
}
func validate(c config.Config) error {
	for _, v := range []string{c.ChatBaseURL, c.EmbedBaseURL} {
		u, err := url.Parse(v)
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("模型地址必须为 HTTP 地址，不能包含凭据、查询参数或片段")
		}
	}
	for _, v := range append([]string{c.ChatModel, c.EmbedModel}, c.ChatModels...) {
		if len(v) == 0 || len([]rune(v)) > 200 || strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("模型名需要 1 到 200 个字符且不能换行")
		}
	}
	if c.QuotaDailyChatRequests < 0 || c.QuotaDailyTokens < 0 {
		return fmt.Errorf("每日额度必须为非负整数")
	}
	if err := c.ValidateStorage(false); err != nil {
		return err
	}
	if c.OSSEndpoint != "" {
		if strings.ContainsAny(c.OSSEndpoint, " \t\r\n?#") {
			return fmt.Errorf("OSS Endpoint 不能包含空白、查询参数或片段")
		}
		if strings.Contains(c.OSSEndpoint, "://") {
			u, err := url.Parse(c.OSSEndpoint)
			if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return fmt.Errorf("OSS Endpoint 格式无效")
			}
		}
	}
	if strings.Contains(c.OSSPrefix, "..") || strings.ContainsAny(c.OSSPrefix, "\\\x00") {
		return fmt.Errorf("OSS 前缀格式无效")
	}
	return nil
}
func validatePatch(patch map[string]string) error {
	for key, value := range patch {
		known := false
		for _, k := range Keys {
			if k == key {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("包含不支持的配置项")
		}
		if len(value) > 4096 || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("配置值过长或包含换行")
		}
		if value == "" || value == Clear {
			continue
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("配置值不能只含空白")
		}
		if strings.HasPrefix(key, "QUOTA_") {
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return fmt.Errorf("每日额度必须为非负整数")
			}
		}
	}
	return nil
}
func affected(patch map[string]string, capability string) (chat, embed, store bool) {
	for key, v := range patch {
		if v == "" {
			continue
		}
		chat = chat || strings.HasPrefix(key, "CHAT_")
		embed = embed || strings.HasPrefix(key, "EMBED_")
		store = store || key == "STORAGE_DRIVER" || strings.HasPrefix(key, "OSS_")
	}
	switch capability {
	case "chat":
		return true, false, false
	case "embed":
		return false, true, false
	case "storage":
		return false, false, true
	}
	return
}
func storageConfigChanged(a, b config.Config) bool {
	return a.StorageDriver != b.StorageDriver ||
		a.OSSRegion != b.OSSRegion ||
		a.OSSEndpoint != b.OSSEndpoint ||
		a.OSSBucket != b.OSSBucket ||
		a.OSSAccessKeyID != b.OSSAccessKeyID ||
		a.OSSAccessKeySecret != b.OSSAccessKeySecret ||
		a.OSSPrefix != b.OSSPrefix
}
func probe(ctx context.Context, c config.Config, client llm.Client, chat, embed, store bool) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if chat {
		if _, err := client.Chat(ctx, []llm.Message{{Role: "user", Content: "你好"}}, llm.ChatOptions{Model: c.ChatModel, MaxTokens: 8}); err != nil {
			return fmt.Errorf("对话连接测试失败，请检查地址、密钥和模型")
		}
	}
	if embed {
		v, err := client.Embed(ctx, []string{"连接测试"})
		if err != nil || len(v) != 1 || len(v[0]) != c.EmbeddingDim {
			return fmt.Errorf("向量连接测试失败，请检查地址、密钥、模型和固定向量维度（%d）", c.EmbeddingDim)
		}
	}
	if store {
		if err := storage.ProbeOSS(ctx, c); err != nil {
			return err
		}
	}
	return nil
}
func (m *Manager) view(c config.Config, sources map[string]string) View {
	v := View{Chat: []Item{}, Embed: []Item{}, Storage: []Item{}, Quota: []Item{}, ReindexRequired: m.reindexRequired}
	for _, key := range Keys {
		value := values(c)[key]
		if secret(key) {
			value = Mask(value)
		}
		item := Item{key, value, sources[key]}
		if strings.HasPrefix(key, "CHAT_") {
			v.Chat = append(v.Chat, item)
		} else if strings.HasPrefix(key, "EMBED_") {
			v.Embed = append(v.Embed, item)
		} else if key == "STORAGE_DRIVER" || strings.HasPrefix(key, "OSS_") {
			v.Storage = append(v.Storage, item)
		} else {
			v.Quota = append(v.Quota, item)
		}
	}
	if v.ReindexRequired {
		v.Message = "向量模型已更换，旧资料需要重建向量才能被准确检索"
	}
	return v
}
func (m *Manager) Get(ctx context.Context) (View, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, sources, err := m.resolve(ctx, nil)
	if err != nil {
		return View{}, err
	}
	return m.view(c, sources), nil
}
func (m *Manager) Test(ctx context.Context, patch map[string]string, capability string) (TestResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := validatePatch(patch); err != nil {
		return TestResult{}, err
	}
	if capability != "" && capability != "chat" && capability != "embed" && capability != "storage" {
		return TestResult{}, fmt.Errorf("测试能力只能为对话、向量或存储")
	}
	c, _, err := m.resolve(ctx, patch)
	if err != nil {
		return TestResult{}, err
	}
	chat, embed, store := affected(patch, capability)
	if !chat && !embed && !store {
		chat, embed = true, true
	}
	start := time.Now()
	err = probe(ctx, c, llm.New(c), chat, embed, store)
	result := TestResult{OK: err == nil, LatencyMS: time.Since(start).Milliseconds(), Message: "连接测试通过"}
	if err != nil {
		result.Message = err.Error()
	}
	return result, nil
}
func (m *Manager) Save(ctx context.Context, userID string, patch map[string]string) (View, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := validatePatch(patch); err != nil {
		return View{}, err
	}
	c, sources, err := m.resolve(ctx, patch)
	if err != nil {
		return View{}, err
	}
	old := m.Provider.Current().Config
	chat, embed, store := affected(patch, "")
	if embed && m.VectorBusy != nil && m.VectorBusy() {
		return View{}, fmt.Errorf("正在重建向量，请完成后再修改向量配置")
	}
	client := llm.New(c)
	if err := probe(ctx, c, client, chat, embed, store && c.StorageDriver == "oss"); err != nil {
		return View{}, err
	}
	changed := []string{}
	err = m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range patch {
			if value == "" {
				continue
			}
			changed = append(changed, key)
			if value == Clear {
				if err := tx.Where("key = ?", key).Delete(&model.SystemSetting{}).Error; err != nil {
					return err
				}
				continue
			}
			value = strings.TrimSpace(value)
			if secret(key) {
				var err error
				value, err = m.Encrypt(value)
				if err != nil {
					return err
				}
			}
			row := model.SystemSetting{Key: key, Value: value, IsSecret: secret(key), UpdatedBy: &userID, UpdatedAt: time.Now()}
			if err := tx.Omit("User").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value", "is_secret", "updated_by", "updated_at"})}).Create(&row).Error; err != nil {
				return err
			}
		}
		if len(changed) == 0 {
			return nil
		}
		sort.Strings(changed)
		return Audit(tx, userID, "修改配置", "system_settings", "更新配置项："+strings.Join(changed, "、"))
	})
	if err != nil {
		return View{}, fmt.Errorf("保存配置失败，请稍后重试")
	}
	m.Provider.Replace(client, c)
	if m.OnStorageChange != nil && storageConfigChanged(old, c) {
		if err := m.OnStorageChange(ctx, c); err != nil {
			return View{}, fmt.Errorf("存储配置已写入数据库，但热切换失败：%w", err)
		}
	}
	if old.EmbedModel != c.EmbedModel || old.EmbedBaseURL != c.EmbedBaseURL {
		m.reindexRequired = true
	}
	return m.view(c, sources), nil
}

// WithSnapshot 与配置保存串行地启动重建，之后向量配置在重建结束前不可修改。
func (m *Manager) WithSnapshot(fn func(llm.Snapshot) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(m.Provider.Current())
}
func (m *Manager) ReindexDone() { m.mu.Lock(); defer m.mu.Unlock(); m.reindexRequired = false }
func Audit(tx *gorm.DB, userID, action, target, detail string) error {
	return tx.Omit("User").Create(&model.AuditLog{ID: uuid.NewString(), UserID: &userID, Action: action, Target: target, Detail: detail, CreatedAt: time.Now()}).Error
}
