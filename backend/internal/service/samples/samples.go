package samples

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/service/chat"
	"knownote/internal/storage"
)

type Manifest struct {
	Slug        string       `json:"slug"`
	Title       string       `json:"title"`
	Emoji       string       `json:"emoji"`
	Description string       `json:"description"`
	Tone        int          `json:"tone"`
	Order       int          `json:"order"`
	Sources     []SourceFile `json:"sources"`
	hash        string
}
type SourceFile struct {
	File    string `json:"file"`
	Title   string `json:"title"`
	content []byte
}
type View struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	Emoji           string `json:"emoji"`
	Description     string `json:"description"`
	Tone            int    `json:"tone"`
	SourceCount     int    `json:"sourceCount"`
	Ready           bool   `json:"ready"`
	AddedNotebookID string `json:"addedNotebookId,omitempty"`
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Load 将清单和正文一起计算哈希；嵌入二进制后不依赖运行目录。
func Load(content fs.FS) ([]Manifest, error) {
	names, err := fs.Glob(content, "*/manifest.json")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("没有找到示例清单")
	}
	result := make([]Manifest, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		raw, err := fs.ReadFile(content, name)
		if err != nil {
			return nil, err
		}
		var m Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("示例清单格式错误：%s", name)
		}
		if !slugPattern.MatchString(m.Slug) || m.Slug != path.Dir(name) || seen[m.Slug] || m.Title == "" || m.Emoji == "" || m.Description == "" || m.Tone < 0 || m.Tone > 7 || len(m.Sources) < 2 || len(m.Sources) > 3 {
			return nil, fmt.Errorf("示例清单字段无效：%s", name)
		}
		seen[m.Slug] = true
		hash := sha256.New()
		hash.Write(raw)
		files := map[string]bool{}
		for i := range m.Sources {
			source := &m.Sources[i]
			if path.Base(source.File) != source.File || path.Ext(source.File) != ".md" || source.Title == "" || files[source.File] {
				return nil, fmt.Errorf("示例来源字段无效：%s", name)
			}
			files[source.File] = true
			source.content, err = fs.ReadFile(content, path.Join(m.Slug, source.File))
			if err != nil {
				return nil, err
			}
			hash.Write([]byte{0})
			hash.Write([]byte(source.File))
			hash.Write([]byte{0})
			hash.Write(source.content)
		}
		m.hash = hex.EncodeToString(hash.Sum(nil))
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Order == result[j].Order {
			return result[i].Slug < result[j].Slug
		}
		return result[i].Order < result[j].Order
	})
	return result, nil
}

type Manager struct {
	base      *service.Service
	chats     *chat.Service
	manifests []Manifest
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func New(ctx context.Context, base *service.Service, chats *chat.Service, content fs.FS) (*Manager, error) {
	manifests, err := Load(content)
	if err != nil {
		return nil, err
	}
	// 保留邮箱，避免启动前注册同名账号；固定标识和数据库 CHECK 双重约束。
	err = base.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user := model.User{Base: model.Base{ID: model.SystemUserID}, Email: model.SystemEmail, Nickname: "知页官方示例", Role: "user", Disabled: true, IsSystem: true, PasswordHash: ""}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoUpdates: clause.AssignmentColumns([]string{"email", "nickname", "role", "disabled", "is_system", "password_hash"})}).Create(&user).Error; err != nil {
			return err
		}
		settings := model.UserSettings{UserID: model.SystemUserID, Model: base.CurrentModel(), AnswerStyle: "concise", Language: "zh-CN"}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"model"})}).Create(&settings).Error
	})
	if err != nil {
		return nil, err
	}
	return &Manager{base: base, chats: chats, manifests: manifests}, nil
}

func (m *Manager) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		pending := map[string]Manifest{}
		for _, manifest := range m.manifests {
			if err := m.prepare(ctx, manifest); err != nil {
				if ctx.Err() == nil {
					m.base.Logger.Error("示例模板初始化失败", "slug", manifest.Slug)
				}
				continue
			}
			pending[manifest.Slug] = manifest
		}
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		for len(pending) > 0 {
			for slug, manifest := range pending {
				done, err := m.finish(ctx, manifest)
				if done || err != nil {
					delete(pending, slug)
				}
				if done && slug == StarterSlug {
					m.seedEmptyUsers(ctx)
				}
				if err != nil && ctx.Err() == nil {
					m.base.Logger.Warn("示例准备失败，下次启动可恢复", "slug", slug)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
func (m *Manager) Close() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}

func (m *Manager) cleanup(ids []string, userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, id := range ids {
		if err := m.base.Storage.DeleteSourceFiles(ctx, userID, id); err != nil {
			m.base.Logger.Warn("示例文件清理将自动重试", "sourceID", id)
		}
	}
}

func (m *Manager) prepare(ctx context.Context, manifest Manifest) error {
	var created, removed []string
	err := m.base.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,12))", manifest.Slug).Error; err != nil {
			return err
		}
		var notebook model.Notebook
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND sample_slug = ? AND is_template = true", model.SystemUserID, manifest.Slug).First(&notebook).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && notebook.SampleHash == manifest.hash {
			// 正常重启不重新处理；中断时继续 processing，失败来源在下次启动重试。
			return tx.Model(&model.Source{}).Where("user_id = ? AND notebook_id = ? AND status = 'error'", model.SystemUserID, notebook.ID).Updates(map[string]any{"status": "processing", "error": "", "progress": 0}).Error
		}
		m.base.Logger.Info("开始准备示例模板", "slug", manifest.Slug, "sources", len(manifest.Sources))
		notebook.UserID, notebook.Title, notebook.Emoji, notebook.Color, notebook.Description = model.SystemUserID, manifest.Title, manifest.Emoji, manifest.Tone, manifest.Description
		notebook.SampleSlug, notebook.SampleHash, notebook.IsTemplate, notebook.SampleReady = manifest.Slug, manifest.hash, true, false
		if err := tx.Omit(clause.Associations).Save(&notebook).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Source{}).Where("user_id = ? AND notebook_id = ?", model.SystemUserID, notebook.ID).Pluck("id", &removed).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND notebook_id = ?", model.SystemUserID, notebook.ID).Delete(&model.Source{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND notebook_id = ?", model.SystemUserID, notebook.ID).Delete(&model.NotebookGuide{}).Error; err != nil {
			return err
		}
		for _, file := range manifest.Sources {
			source := model.Source{Base: model.Base{ID: uuid.NewString()}, NotebookOwner: model.NotebookOwner{UserID: model.SystemUserID, NotebookID: notebook.ID}, Title: file.Title, Type: "markdown", OriginalExt: "md", Status: "processing", Topics: []string{}, Sections: []model.SourceSection{}}
			created = append(created, source.ID)
			source.ObjectKey = storage.OriginalKey(m.base.Storage, source.UserID, source.ID, "md")
			source.FilePath = source.ObjectKey
			size, err := m.base.Storage.Put(ctx, source.ObjectKey, bytes.NewReader(file.content), "text/markdown; charset=utf-8")
			if err != nil {
				return err
			}
			source.Size = size
			if err := tx.Omit(clause.Associations).Create(&source).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		m.cleanup(created, model.SystemUserID)
		return err
	}
	m.cleanup(removed, model.SystemUserID)
	m.base.Ingest.Notify()
	return nil
}

func (m *Manager) finish(ctx context.Context, manifest Manifest) (bool, error) {
	var notebook model.Notebook
	if err := m.base.DB.WithContext(ctx).Where("user_id = ? AND sample_slug = ? AND is_template = true", model.SystemUserID, manifest.Slug).First(&notebook).Error; err != nil {
		return false, err
	}
	var sources []model.Source
	if err := m.base.DB.WithContext(ctx).Select("id", "status").Where("user_id = ? AND notebook_id = ?", model.SystemUserID, notebook.ID).Find(&sources).Error; err != nil {
		return false, err
	}
	if len(sources) != len(manifest.Sources) {
		return false, fmt.Errorf("模板来源数量不完整")
	}
	for _, source := range sources {
		if source.Status == "error" {
			return false, fmt.Errorf("示例来源处理失败")
		}
		if source.Status != "ready" {
			return false, nil
		}
	}
	if notebook.SampleReady {
		return true, nil
	}
	m.base.Logger.Info("示例来源已就绪，生成概要", "slug", manifest.Slug)
	if _, err := m.chats.Guide(ctx, model.SystemUserID, notebook.ID); err != nil {
		return false, err
	}
	if err := m.base.DB.WithContext(ctx).Model(&model.Notebook{}).Where("user_id = ? AND id = ? AND sample_hash = ?", model.SystemUserID, notebook.ID, manifest.hash).Update("sample_ready", true).Error; err != nil {
		return false, err
	}
	m.base.Logger.Info("示例模板已就绪", "slug", manifest.Slug)
	return true, nil
}

func (m *Manager) List(ctx context.Context, userID string) ([]View, error) {
	var templates, added []model.Notebook
	if err := m.base.DB.WithContext(ctx).Where("user_id = ? AND is_template = true", model.SystemUserID).Find(&templates).Error; err != nil {
		return nil, err
	}
	if err := m.base.DB.WithContext(ctx).Where("user_id = ? AND sample_slug <> '' AND is_template = false", userID).Find(&added).Error; err != nil {
		return nil, err
	}
	bySlug := map[string]model.Notebook{}
	ids := map[string]string{}
	for _, n := range templates {
		bySlug[n.SampleSlug] = n
	}
	for _, n := range added {
		ids[n.SampleSlug] = n.ID
	}
	result := make([]View, 0, len(m.manifests))
	for _, manifest := range m.manifests {
		n := bySlug[manifest.Slug]
		result = append(result, View{manifest.Slug, manifest.Title, manifest.Emoji, manifest.Description, manifest.Tone, len(manifest.Sources), n.SampleReady && n.SampleHash == manifest.hash, ids[manifest.Slug]})
	}
	return result, nil
}

var ErrPreparing = &service.Error{Status: 409, Code: 40901, Message: "示例正在准备中，请稍后再试"}

func (m *Manager) Use(ctx context.Context, userID, slug string) (service.NotebookView, error) {
	var manifest *Manifest
	for i := range m.manifests {
		if m.manifests[i].Slug == slug {
			manifest = &m.manifests[i]
			break
		}
	}
	if manifest == nil {
		return service.NotebookView{}, service.ErrNotFound
	}
	var id string
	var created []string
	err := m.base.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,12))", userID+":"+slug).Error; err != nil {
			return err
		}
		var existing model.Notebook
		err := tx.Where("user_id = ? AND sample_slug = ? AND is_template = false", userID, slug).First(&existing).Error
		if err == nil {
			id = existing.ID
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var template model.Notebook
		err = tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("user_id = ? AND sample_slug = ? AND is_template = true", model.SystemUserID, slug).First(&template).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPreparing
		}
		if err != nil {
			return err
		}
		if !template.SampleReady || template.SampleHash != manifest.hash {
			return ErrPreparing
		}
		var sources []model.Source
		if err := tx.Where("user_id = ? AND notebook_id = ?", model.SystemUserID, template.ID).Order("created_at,id").Find(&sources).Error; err != nil {
			return err
		}
		if len(sources) != len(manifest.Sources) {
			return ErrPreparing
		}
		for _, source := range sources {
			if source.Status != "ready" {
				return ErrPreparing
			}
		}
		var guide model.NotebookGuide
		if err := tx.Where("user_id = ? AND notebook_id = ? AND stale = false", model.SystemUserID, template.ID).First(&guide).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPreparing
			}
			return err
		}
		nb := model.Notebook{UserID: userID, Title: template.Title, Emoji: template.Emoji, Color: template.Color, Description: template.Description, SampleSlug: slug}
		if err := tx.Omit(clause.Associations).Create(&nb).Error; err != nil {
			return err
		}
		id = nb.ID
		for _, original := range sources {
			source := original
			source.Base = model.Base{ID: uuid.NewString()}
			source.NotebookOwner = model.NotebookOwner{UserID: userID, NotebookID: nb.ID}
			created = append(created, source.ID)
			oldKey := original.ObjectKey
			if oldKey == "" {
				oldKey = original.FilePath
			}
			source.FilePath, source.ObjectKey, source.PreviewKey = "", "", ""
			if oldKey != "" {
				source.ObjectKey = storage.OriginalKey(m.base.Storage, userID, source.ID, source.OriginalExt)
				if err := m.copyObject(ctx, oldKey, source.ObjectKey, storage.ContentType(source.OriginalExt)); err != nil {
					return err
				}
				source.FilePath = source.ObjectKey
			}
			if original.PreviewKey != "" {
				if original.PreviewKey == oldKey {
					source.PreviewKey = source.ObjectKey
				} else {
					source.PreviewKey = storage.SourcePrefix(m.base.Storage, userID, source.ID) + "preview.pdf"
					if err := m.copyObject(ctx, original.PreviewKey, source.PreviewKey, "application/pdf"); err != nil {
						return err
					}
				}
			}
			if err := tx.Omit(clause.Associations).Create(&source).Error; err != nil {
				return err
			}
			// INSERT SELECT 在数据库内原样复制 vector，避免模型调用和浮点转换。
			if err := tx.Exec(`INSERT INTO chunks (id,created_at,updated_at,user_id,notebook_id,source_id,page,location_label,section_index,chunk_index,content,token_count,embedding)
    SELECT gen_random_uuid(),NOW(),NOW(),?,?,?,page,location_label,section_index,chunk_index,content,token_count,embedding FROM chunks WHERE user_id=? AND notebook_id=? AND source_id=?`, userID, nb.ID, source.ID, model.SystemUserID, template.ID, original.ID).Error; err != nil {
				return err
			}
		}
		guide.Base = model.Base{}
		guide.NotebookOwner = model.NotebookOwner{UserID: userID, NotebookID: nb.ID}
		if err := tx.Omit(clause.Associations).Create(&guide).Error; err != nil {
			return err
		}
		// GORM 的 true 默认值会覆盖 false 零值，需要显式恢复有效缓存。
		return tx.Model(&model.NotebookGuide{}).Where("user_id = ? AND notebook_id = ?", userID, nb.ID).Update("stale", false).Error
	})
	if err != nil {
		m.cleanup(created, userID)
		return service.NotebookView{}, err
	}
	return m.base.Notebook(ctx, userID, id)
}
// StarterSlug 为每个新用户自动初始化的官方示例笔记本标识（对应《知页使用指南》）。
const StarterSlug = "knownote-guide"

// EnsureStarter 为尚未拥有笔记本的账号自动克隆初始《知页使用指南》模板，并保留幂等性。
func (m *Manager) EnsureStarter(ctx context.Context, userID string) error {
	var count int64
	if err := m.base.DB.WithContext(ctx).Model(&model.Notebook{}).Where("user_id = ? AND is_template = false", userID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := m.Use(ctx, userID, StarterSlug)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrPreparing) || time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}

// SeedStarterForUser 供注册回调使用，初始化失败时记录警告但不阻断注册主流程。
func (m *Manager) SeedStarterForUser(ctx context.Context, userID string) {
	if err := m.EnsureStarter(ctx, userID); err != nil {
		m.base.Logger.Warn("新用户初始示例笔记本准备未完成", "userID", userID, "error", err)
	}
}

// seedEmptyUsers 在服务启动后为当前尚无任何笔记本的非系统账号补充初始指南笔记本。
func (m *Manager) seedEmptyUsers(ctx context.Context) {
	var userIDs []string
	if err := m.base.DB.WithContext(ctx).Model(&model.User{}).
		Where("is_system = false AND disabled = false AND NOT EXISTS (SELECT 1 FROM notebooks n WHERE n.user_id = users.id AND n.is_template = false)").
		Pluck("id", &userIDs).Error; err != nil {
		return
	}
	for _, userID := range userIDs {
		m.SeedStarterForUser(ctx, userID)
	}
}

func (m *Manager) copyObject(ctx context.Context, from, to, contentType string) error {
	reader, err := m.base.Storage.Open(ctx, from)
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = m.base.Storage.Put(ctx, to, reader, contentType)
	return err
}
