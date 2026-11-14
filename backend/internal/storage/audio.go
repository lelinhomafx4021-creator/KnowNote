package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
)

const MaxAudioSize int64 = 64 << 20

func AudioPath(userID, notebookID string) string {
	return filepath.Join("audio", userID, notebookID+".mp3")
}

// SaveAudio 调用统一对象写入接口；调用方须持有笔记本行锁。
func (s *Local) SaveAudio(ctx context.Context, userID, notebookID string, input io.Reader) (string, error) {
	if !validID(userID) || !validID(notebookID) {
		return "", fmt.Errorf("音频归属无效")
	}
	key := AudioPath(userID, notebookID)
	_, err := s.Put(ctx, key, input, "audio/mpeg")
	return key, err
}
