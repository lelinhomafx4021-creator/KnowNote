package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"knownote/internal/model"
	"knownote/internal/parser"
	"knownote/internal/storage"
	"knownote/pkg/response"
)

type SourceView struct {
	ID          string                `json:"id"`
	NotebookID  string                `json:"notebookId"`
	Type        string                `json:"type"`
	Title       string                `json:"title"`
	Status      string                `json:"status"`
	Progress    int                   `json:"progress"`
	Size        int64                 `json:"size"`
	URL         string                `json:"url,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Topics      []string              `json:"topics"`
	Content     string                `json:"content,omitempty"`
	WordCount   int                   `json:"wordCount"`
	Error       string                `json:"error,omitempty"`
	PageCount   int                   `json:"pageCount,omitempty"`
	Sections    []model.SourceSection `json:"sections"`
	Previewable bool                  `json:"previewable"`
	OriginalExt string                `json:"originalExt,omitempty"`
	CreatedAt   time.Time             `json:"createdAt"`
}

func sourceView(source model.Source) SourceView {
	content := []rune(source.Content)
	content = content[:min(3000, len(content))]
	topics := source.Topics
	if topics == nil {
		topics = []string{}
	}
	sections := source.Sections
	if sections == nil {
		sections = []model.SourceSection{}
	}
	return SourceView{PageCount: source.PageCount, Sections: sections, Previewable: source.Previewable, OriginalExt: source.OriginalExt, ID: source.ID, NotebookID: source.NotebookID, Type: source.Type, Title: source.Title, Status: source.Status,
		Progress: source.Progress, Size: source.Size, URL: source.URL, Summary: source.Summary, Topics: topics, Content: string(content),
		WordCount: source.WordCount, Error: source.Error, CreatedAt: source.CreatedAt}
}

func invalidSource(message string) error {
	return &Error{http.StatusBadRequest, response.CodeInvalid, message}
}

func lockNotebook(tx *gorm.DB, userID, notebookID string) error {
	var notebook model.Notebook
	return databaseError(tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND id = ?", userID, notebookID).First(&notebook).Error)
}

// MarkSourcesChanged 必须在来源变更事务内调用；同时刷新列表排序与概要缓存。
func MarkSourcesChanged(tx *gorm.DB, userID, notebookID string) error {
	if err := tx.Model(&model.Notebook{}).Where("user_id = ? AND id = ?", userID, notebookID).Update("updated_at", time.Now()).Error; err != nil {
		return err
	}
	return tx.Model(&model.NotebookGuide{}).Where("user_id = ? AND notebook_id = ?", userID, notebookID).Update("stale", true).Error
}

func (s *Service) Sources(ctx context.Context, userID, notebookID string) ([]SourceView, error) {
	if _, err := s.Notebook(ctx, userID, notebookID); err != nil {
		return nil, err
	}
	var sources []model.Source
	err := s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ?", userID, notebookID).Order("created_at ASC, id ASC").Find(&sources).Error
	views := make([]SourceView, 0, len(sources))
	for _, source := range sources {
		views = append(views, sourceView(source))
	}
	return views, err
}

func (s *Service) Source(ctx context.Context, userID, id string) (SourceView, error) {
	var source model.Source
	err := s.DB.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).First(&source).Error
	return sourceView(source), databaseError(err)
}

func UploadType(name string) (string, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "pdf", nil
	case ".doc", ".docx":
		return "docx", nil
	case ".ppt", ".pptx":
		return "pptx", nil
	case ".xls", ".xlsx":
		return "xlsx", nil
	case ".csv":
		return "csv", nil
	case ".txt":
		return "text", nil
	case ".md", ".markdown":
		return "markdown", nil
	case ".mp3", ".wav", ".m4a":
		return "audio", nil
	default:
		return "", invalidSource("不支持此文件格式，请上传 PDF、Word、PPT、Excel、CSV、TXT、Markdown 或音频文件")
	}
}

func (s *Service) UploadSource(ctx context.Context, userID, notebookID, name string, input io.Reader) (SourceView, error) {
	name, err := storage.Filename(name)
	if err != nil {
		return SourceView{}, invalidSource(err.Error())
	}
	kind, err := UploadType(name)
	if err != nil {
		return SourceView{}, err
	}
	source := model.Source{Base: model.Base{ID: uuid.NewString()}, NotebookOwner: model.NotebookOwner{UserID: userID, NotebookID: notebookID},
		OriginalExt: strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."), Type: kind, Title: name, Status: "processing", Topics: []string{}}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockNotebook(tx, userID, notebookID); err != nil {
			return err
		}
		key := storage.OriginalKey(s.Storage, userID, source.ID, source.OriginalExt)
		size, err := s.Storage.Put(ctx, key, input, storage.ContentType(source.OriginalExt))
		if err != nil {
			if errors.Is(err, storage.ErrTooLarge) {
				return invalidSource(err.Error())
			}
			return err
		}
		source.FilePath, source.ObjectKey, source.Size = key, key, size
		if err := tx.Create(&source).Error; err != nil {
			return err
		}
		return MarkSourcesChanged(tx, userID, notebookID)
	})
	if err != nil {
		if source.FilePath != "" {
			if err := s.Storage.DeleteSourceFiles(context.Background(), userID, source.ID); err != nil {
				s.Logger.Warn("上传回滚后的文件清理将自动重试")
			}
		}
		return SourceView{}, err
	}
	s.Ingest.Notify()
	return sourceView(source), nil
}

type CreateSourceInput struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Text  string `json:"text"`
}

func (s *Service) CreateSource(ctx context.Context, userID, notebookID string, input CreateSourceInput) (SourceView, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.URL = strings.TrimSpace(input.URL)
	if !utf8.ValidString(input.Title) || utf8.RuneCountInString(input.Title) > 500 || strings.ContainsRune(input.Title, '\x00') {
		return SourceView{}, invalidSource("标题不能超过 500 个字符")
	}
	switch input.Type {
	case "text":
		if !utf8.ValidString(input.Text) {
			return SourceView{}, invalidSource("来源文本必须为有效的 UTF-8")
		}
		// PostgreSQL 文本列不能保存 NUL，入队前先清理，后台仍执行统一清洗。
		input.Text = parser.Clean(input.Text)
		if input.Text == "" {
			return SourceView{}, invalidSource("请提供非空的来源文本")
		}
		if input.URL != "" {
			return SourceView{}, invalidSource("文本来源不接受链接")
		}
		if input.Title == "" {
			runes := []rune(strings.Join(strings.Fields(input.Text), " "))
			input.Title = string(runes[:min(30, len(runes))])
		}
	case "web", "youtube":
		if err := parser.ValidateURL(input.URL); err != nil {
			return SourceView{}, invalidSource(err.Error())
		}
		if input.Text != "" {
			return SourceView{}, invalidSource("链接来源不接受文本，请另建文本来源")
		}
		if input.Title == "" {
			runes := []rune(input.URL)
			input.Title = string(runes[:min(500, len(runes))])
		}
	default:
		return SourceView{}, invalidSource("来源类型必须是 web、youtube 或 text")
	}
	source := model.Source{NotebookOwner: model.NotebookOwner{UserID: userID, NotebookID: notebookID}, Type: input.Type,
		Title: input.Title, URL: input.URL, Content: input.Text, Status: "processing", Topics: []string{}}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockNotebook(tx, userID, notebookID); err != nil {
			return err
		}
		if err := tx.Create(&source).Error; err != nil {
			return err
		}
		return MarkSourcesChanged(tx, userID, notebookID)
	})
	if err != nil {
		return SourceView{}, err
	}
	s.Ingest.Notify()
	return sourceView(source), nil
}

func (s *Service) UpdateSource(ctx context.Context, userID, id, title string) (SourceView, error) {
	title = strings.TrimSpace(title)
	if !utf8.ValidString(title) || utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 500 || strings.ContainsRune(title, '\x00') {
		return SourceView{}, invalidSource("标题需要 1 到 500 个字符")
	}
	view, err := s.Source(ctx, userID, id)
	if err != nil {
		return SourceView{}, err
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockNotebook(tx, userID, view.NotebookID); err != nil {
			return err
		}
		result := tx.Model(&model.Source{}).Where("user_id = ? AND id = ?", userID, id).Update("title", title)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return MarkSourcesChanged(tx, userID, view.NotebookID)
	})
	if err != nil {
		return SourceView{}, err
	}
	return s.Source(ctx, userID, id)
}

func (s *Service) DeleteSource(ctx context.Context, userID, id string) error {
	view, err := s.Source(ctx, userID, id)
	if err != nil {
		return err
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockNotebook(tx, userID, view.NotebookID); err != nil {
			return err
		}
		result := tx.Where("user_id = ? AND id = ?", userID, id).Delete(&model.Source{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return MarkSourcesChanged(tx, userID, view.NotebookID)
	})
	if err != nil {
		return err
	}
	if err := s.Storage.DeleteSourceFiles(context.Background(), userID, id); err != nil {
		s.Logger.Warn("来源文件清理将自动重试", "sourceID", id)
	}
	return nil
}

// SourceFile 在任何存储访问或签名前先完成用户归属校验。
func (s *Service) SourceFile(ctx context.Context, userID, id string, preview bool) (key, name, contentType string, err error) {
	var source model.Source
	if err = s.DB.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).First(&source).Error; err != nil {
		err = databaseError(err)
		return
	}
	name = source.Title
	contentType = storage.ContentType(source.OriginalExt)
	key = source.ObjectKey
	if key == "" {
		key = source.FilePath
	}
	if preview {
		if !source.Previewable {
			err = ErrNotFound
			return
		}
		key = source.PreviewKey
		name = "preview.pdf"
		contentType = "application/pdf"
	}
	if key == "" {
		err = ErrNotFound
	}
	return
}
