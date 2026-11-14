package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type cleanupOperation struct {
	Key    string `json:"key"`
	Prefix bool   `json:"prefix"`
}
type objectCleanup struct {
	root    *os.Root
	execute func(context.Context, cleanupOperation) error
	logger  *slog.Logger
	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newObjectCleanup(ctx context.Context, dir string, logger *slog.Logger, execute func(context.Context, cleanupOperation) error) (*objectCleanup, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("创建 OSS 清理日志目录失败")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("打开 OSS 清理日志目录失败")
	}
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(ctx)
	q := &objectCleanup{root: root, logger: logger, cancel: cancel, execute: execute}
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			q.retry(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return q, nil
}
func (q *objectCleanup) close() error { q.cancel(); q.wg.Wait(); return q.root.Close() }
func (q *objectCleanup) schedule(ctx context.Context, operations []cleanupOperation) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	data, err := json.Marshal(operations)
	if err != nil {
		return err
	}
	name := uuid.NewString() + ".json"
	if err := q.root.WriteFile(name+".tmp", data, 0600); err != nil {
		return err
	}
	if err := q.root.Rename(name+".tmp", name); err != nil {
		return err
	}
	return q.attempt(ctx, name, operations)
}
func (q *objectCleanup) attempt(ctx context.Context, name string, operations []cleanupOperation) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for _, operation := range operations {
		if err := q.execute(ctx, operation); err != nil {
			return err
		}
	}
	return q.root.Remove(name)
}
func (q *objectCleanup) retry(ctx context.Context) {
	q.mu.Lock()
	defer q.mu.Unlock()
	entries, err := fs.ReadDir(q.root.FS(), ".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := q.root.ReadFile(entry.Name())
		var operations []cleanupOperation
		if err != nil || json.Unmarshal(data, &operations) != nil {
			q.logger.Warn("读取 OSS 清理日志失败")
			continue
		}
		if err := q.attempt(ctx, entry.Name(), operations); err != nil && ctx.Err() == nil {
			q.logger.Warn("OSS 对象清理失败，将自动重试")
		}
	}
}
