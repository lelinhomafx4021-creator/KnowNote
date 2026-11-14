package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"knownote/internal/config"
)

type Info struct {
	Size        int64
	ModTime     time.Time
	ContentType string
}

// Store 以对象键读写，不向业务层暴露本地绝对路径或 OSS 凭据。
type Store interface {
	Put(context.Context, string, io.Reader, string) (int64, error)
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	DeletePrefix(context.Context, string) error
	Stat(context.Context, string) (Info, error)
	SignedURL(context.Context, string, time.Duration) (string, error)
	Driver() string
	Key(string) string
	Close() error
	DeleteSourceFiles(context.Context, string, string) error
	DeleteNotebookFiles(context.Context, string, string, []string) error
	SaveAudio(context.Context, string, string, io.Reader) (string, error)
}

func NewConfigured(ctx context.Context, c config.Config, logger *slog.Logger) (Store, error) {
	if c.StorageDriver == "oss" {
		return NewOSS(ctx, c, logger)
	}
	return New(ctx, c.DataDir, logger)
}
func validKey(key string) bool {
	return key != "" && !strings.ContainsAny(key, "\\\x00") && !strings.HasPrefix(key, "/") && path.Clean(key) == key && key != "." && key != ".." && !strings.HasPrefix(key, "../")
}
func SourcePrefix(s Store, userID, sourceID string) string {
	return s.Key(path.Join("uploads", userID, sourceID)) + "/"
}
func OriginalKey(s Store, userID, sourceID, ext string) string {
	return SourcePrefix(s, userID, sourceID) + "original." + ext
}

// Materialize 为必须使用本地文件的解析步骤下载原件，并由调用方清理。
func Materialize(ctx context.Context, s Store, key, ext string) (*os.File, func(), error) {
	stream, err := s.Open(ctx, key)
	if err != nil {
		return nil, func() {}, err
	}
	defer stream.Close()
	dir, err := os.MkdirTemp("", "knownote-input-*")
	if err != nil {
		return nil, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if strings.ContainsAny(ext, "/\\.") {
		cleanup()
		return nil, func() {}, fmt.Errorf("文件扩展名无效")
	}
	file, err := os.OpenFile(path.Join(dir, "original."+ext), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	size, err := io.Copy(file, io.LimitReader(contextReader{ctx, stream}, MaxUploadSize+1))
	if err == nil && size > MaxUploadSize {
		err = ErrTooLarge
	}
	if err == nil {
		_, err = file.Seek(0, io.SeekStart)
	}
	if err != nil {
		file.Close()
		cleanup()
		return nil, func() {}, err
	}
	return file, func() { file.Close(); cleanup() }, nil
}

func ContentType(ext string) string {
	switch ext {
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "doc":
		return "application/msword"
	case "ppt":
		return "application/vnd.ms-powerpoint"
	case "xls":
		return "application/vnd.ms-excel"
	case "csv":
		return "text/csv; charset=utf-8"
	case "mp3":
		return "audio/mpeg"
	}
	return "application/octet-stream"
}

type responseHeadersKey struct{}
type responseHeaders struct{ Disposition, ContentType string }

func WithResponseHeaders(ctx context.Context, disposition, contentType string) context.Context {
	return context.WithValue(ctx, responseHeadersKey{}, responseHeaders{disposition, contentType})
}
