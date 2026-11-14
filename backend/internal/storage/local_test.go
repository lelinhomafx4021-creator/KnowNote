package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestFilesAndCleanup(t *testing.T) {
	dir := t.TempDir()
	s, err := New(context.Background(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	user, source, notebook := uuid.NewString(), uuid.NewString(), uuid.NewString()
	path, size, err := s.Save(context.Background(), user, source, "../../资料.md", strings.NewReader("知识管理"))
	if err != nil || size != 12 || path != filepath.Join("uploads", user, source, "资料.md") {
		t.Fatalf("文件路径或大小错误：%v", err)
	}
	file, err := s.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(file)
	file.Close()
	if string(data) != "知识管理" {
		t.Fatal("保存内容错误")
	}
	if err := s.DeleteNotebookFiles(context.Background(), user, notebook, []string{filepath.Join(dir, path)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("笔记本文件未清理")
	}
	if err := s.DeleteSourceFiles(context.Background(), user, source); err != nil {
		t.Fatal("重复清理应该安全")
	}
	if err := s.DeleteNotebookFiles(context.Background(), uuid.NewString(), notebook, []string{path}); err == nil {
		t.Fatal("不应清理其他用户文件")
	}
	if err := s.remove([]string{"."}); err == nil {
		t.Fatal("不应删除根目录")
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "file"), []byte("目录外的测试文件"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open("escape/file"); err == nil {
		t.Fatal("不应访问根目录外的符号链接")
	}
	for _, name := range []string{"", "..", "a\x00.md"} {
		if _, err := Filename(name); err == nil {
			t.Fatal("应拒绝非法文件名")
		}
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestUploadLimitAndCancellation(t *testing.T) {
	s, err := New(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, source := uuid.NewString(), uuid.NewString()
	if _, _, err := s.Save(context.Background(), user, source, "large.pdf", io.LimitReader(zeros{}, MaxUploadSize+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("超出 200MB 未被拒绝：%v", err)
	}
	if _, err := s.root.Stat(filepath.Join("uploads", user, source)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("超限上传遗留文件")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := s.Save(ctx, user, source, "cancel.md", strings.NewReader("文本")); !errors.Is(err, context.Canceled) {
		t.Fatal("取消上传应终止")
	}
}

func TestCleanupRestart(t *testing.T) {
	dir := t.TempDir()
	user, source := uuid.NewString(), uuid.NewString()
	path := filepath.Join("uploads", user, source)
	if err := os.MkdirAll(filepath.Join(dir, path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".cleanup"), 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal([]string{path})
	if err := os.WriteFile(filepath.Join(dir, ".cleanup", uuid.NewString()+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(context.Background(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.retry()
	if _, err := os.Stat(filepath.Join(dir, path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("重启后未恢复文件清理任务")
	}
}
