package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

const MaxAudioSize int64 = 64 << 20

func AudioPath(userID, notebookID string) string {
	return filepath.Join("audio", userID, notebookID+".mp3")
}

// SaveAudio 先写同目录临时文件再原子替换；调用方须持有笔记本行锁。
func (s *Local) SaveAudio(ctx context.Context, userID, notebookID string, input io.Reader) (string, error) {
	if !validID(userID) || !validID(notebookID) {
		return "", fmt.Errorf("音频归属无效")
	}
	path := AudioPath(userID, notebookID)
	if err := s.root.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	temp := path + "." + uuid.NewString() + ".tmp"
	file, err := s.root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer s.root.Remove(temp)
	size, err := io.Copy(file, io.LimitReader(contextReader{ctx, input}, MaxAudioSize+1))
	closeErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if size == 0 || size > MaxAudioSize {
		return "", fmt.Errorf("音频文件大小无效")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := s.root.Rename(temp, path); err != nil {
		return "", err
	}
	return path, nil
}
