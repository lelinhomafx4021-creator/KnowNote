package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
)

const MaxUploadSize int64 = 200 << 20

var ErrTooLarge = errors.New("文件不能超过 200MB")

// Local 用受限目录访问文件；清理日志使失败的删除能在重启后重试。
type Local struct {
	root    *os.Root
	logger  *slog.Logger
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	pending map[string][]string
}

func New(ctx context.Context, dir string, logger *slog.Logger) (*Local, error) {
	if logger == nil {
		logger = slog.Default()
	}
	var err error
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("文件存储目录无效")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("创建文件存储目录失败")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("打开文件存储目录失败")
	}
	if err := root.MkdirAll(".cleanup", 0700); err != nil {
		root.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Local{root: root, logger: logger, cancel: cancel, pending: make(map[string][]string)}
	entries, err := fs.ReadDir(root.FS(), ".cleanup")
	if err != nil {
		cancel()
		root.Close()
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := filepath.Join(".cleanup", entry.Name())
		data, err := root.ReadFile(name)
		var paths []string
		if err != nil || json.Unmarshal(data, &paths) != nil {
			cancel()
			root.Close()
			return nil, fmt.Errorf("读取文件清理记录失败")
		}
		for _, path := range paths {
			if !cleanupPath(path) {
				cancel()
				root.Close()
				return nil, fmt.Errorf("文件清理记录包含非法路径")
			}
		}
		s.pending[name] = paths
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			s.retry()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return s, nil
}

func (s *Local) Close() error { s.cancel(); s.wg.Wait(); return s.root.Close() }

func validID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed != uuid.Nil && parsed.String() == id
}

func Filename(name string) (string, error) {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "." || name == ".." || name == "" || len(name) > 240 || strings.ContainsFunc(name, unicode.IsControl) {
		return "", fmt.Errorf("文件名无效或过长")
	}
	return name, nil
}

func (s *Local) Save(ctx context.Context, userID, sourceID, name string, input io.Reader) (path string, size int64, err error) {
	if !validID(userID) || !validID(sourceID) {
		return "", 0, fmt.Errorf("文件归属无效")
	}
	name, err = Filename(name)
	if err != nil {
		return "", 0, err
	}
	dir := filepath.Join("uploads", userID, sourceID)
	if err = s.root.MkdirAll(dir, 0700); err != nil {
		return "", 0, fmt.Errorf("创建上传目录失败")
	}
	path = filepath.Join(dir, name)
	file, err := s.root.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", 0, fmt.Errorf("保存上传文件失败")
	}
	size, err = io.Copy(file, io.LimitReader(contextReader{ctx, input}, MaxUploadSize+1))
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if size > MaxUploadSize {
		err = ErrTooLarge
	}
	if err != nil {
		_ = s.remove([]string{dir})
		return "", 0, err
	}
	return path, size, nil
}

type contextReader struct {
	ctx context.Context
	io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

func (s *Local) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validKey(key) {
		return nil, fmt.Errorf("文件路径无效")
	}
	return s.root.Open(key)
}

func (s *Local) DeleteSourceFiles(_ context.Context, userID, sourceID string) error {
	if !validID(userID) || !validID(sourceID) {
		return fmt.Errorf("文件归属无效")
	}
	return s.remove([]string{filepath.Join("uploads", userID, sourceID)})
}

func (s *Local) DeleteNotebookFiles(_ context.Context, userID, notebookID string, paths []string) error {
	if !validID(userID) || !validID(notebookID) {
		return fmt.Errorf("文件归属无效")
	}
	dirs := []string{AudioPath(userID, notebookID)}
	for _, path := range paths {
		if filepath.IsAbs(path) {
			var err error
			path, err = filepath.Rel(s.root.Name(), path)
			if err != nil {
				return fmt.Errorf("文件路径无效")
			}
		}
		parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
		if len(parts) == 4 && parts[0] == "uploads" && parts[1] == userID && validID(parts[2]) {
			dirs = append(dirs, filepath.Join(parts[:3]...))
		} else if path == AudioPath(userID, notebookID) {
			dirs = append(dirs, path)
		} else {
			return fmt.Errorf("文件路径不属于当前用户的存储范围")
		}
	}
	return s.remove(dirs)
}

func cleanupPath(path string) bool {
	if !filepath.IsLocal(path) || filepath.Clean(path) != path {
		return false
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	return (len(parts) == 3 && parts[0] == "uploads" && validID(parts[1]) && validID(parts[2])) || (len(parts) >= 2 && parts[0] == "audio")
}

func (s *Local) remove(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	for _, path := range paths {
		if !cleanupPath(path) {
			return fmt.Errorf("禁止清理存储范围以外的文件")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name := filepath.Join(".cleanup", uuid.NewString()+".json")
	s.pending[name] = paths
	return s.attempt(name, paths)
}

func (s *Local) attempt(name string, paths []string) error {
	data, err := json.Marshal(paths)
	if err != nil {
		return err
	}
	if err := s.root.WriteFile(name+".tmp", data, 0600); err != nil {
		return err
	}
	if err := s.root.Rename(name+".tmp", name); err != nil {
		return err
	}
	for _, path := range paths {
		if err := s.root.RemoveAll(path); err != nil {
			return err
		}
	}
	if err := s.root.Remove(name); err != nil {
		return err
	}
	delete(s.pending, name)
	return nil
}

func (s *Local) retry() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, paths := range s.pending {
		if err := s.attempt(name, paths); err != nil {
			s.logger.Warn("文件清理失败，将自动重试")
		}
	}
}

func (s *Local) Driver() string        { return "local" }
func (s *Local) Key(key string) string { return key }
func (s *Local) Put(ctx context.Context, key string, input io.Reader, contentType string) (int64, error) {
	if !validKey(key) {
		return 0, fmt.Errorf("文件路径无效")
	}
	if err := s.root.MkdirAll(filepath.Dir(key), 0700); err != nil {
		return 0, err
	}
	temp := key + "." + uuid.NewString() + ".tmp"
	file, err := s.root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return 0, err
	}
	defer s.root.Remove(temp)
	size, err := io.Copy(file, io.LimitReader(contextReader{ctx, input}, MaxUploadSize+1))
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if size > MaxUploadSize {
		err = ErrTooLarge
	}
	if err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return size, s.root.Rename(temp, key)
}
func (s *Local) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validKey(key) {
		return fmt.Errorf("文件路径无效")
	}
	err := s.root.Remove(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func (s *Local) DeletePrefix(ctx context.Context, prefix string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.remove([]string{strings.TrimSuffix(prefix, "/")})
}
func (s *Local) Stat(ctx context.Context, key string) (Info, error) {
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	if !validKey(key) {
		return Info{}, fmt.Errorf("文件路径无效")
	}
	info, err := s.root.Stat(key)
	if err != nil {
		return Info{}, err
	}
	return Info{Size: info.Size(), ModTime: info.ModTime()}, nil
}
func (s *Local) SignedURL(context.Context, string, time.Duration) (string, error) { return "", nil }
