package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"knownote/internal/config"
)

type OSS struct {
	client         *oss.Client
	bucket, prefix string
	cleanup        *objectCleanup
}

func NewOSS(ctx context.Context, c config.Config, logger *slog.Logger) (*OSS, error) {
	cfg := oss.LoadDefaultConfig().WithRegion(c.OSSRegion).WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.OSSAccessKeyID, c.OSSAccessKeySecret))
	if c.OSSEndpoint != "" {
		cfg = cfg.WithEndpoint(c.OSSEndpoint)
	}
	s := &OSS{client: oss.NewClient(cfg), bucket: c.OSSBucket, prefix: strings.Trim(c.OSSPrefix, "/") + "/"}
	var err error
	s.cleanup, err = newObjectCleanup(ctx, filepath.Join(c.DataDir, ".oss-cleanup"), logger, func(ctx context.Context, op cleanupOperation) error {
		relative := strings.TrimPrefix(op.Key, s.prefix)
		parts := strings.Split(strings.TrimSuffix(relative, "/"), "/")
		if op.Prefix {
			if len(parts) != 3 || parts[0] != "uploads" || !validID(parts[1]) || !validID(parts[2]) {
				return fmt.Errorf("OSS 清理日志归属无效")
			}
			return s.DeletePrefix(ctx, op.Key)
		}
		if len(parts) != 3 || parts[0] != "audio" || !validID(parts[1]) || !validID(strings.TrimSuffix(parts[2], ".mp3")) {
			return fmt.Errorf("OSS 清理日志归属无效")
		}
		return s.Delete(ctx, op.Key)
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}
func (s *OSS) Driver() string        { return "oss" }
func (s *OSS) Key(key string) string { return s.prefix + key }
func (s *OSS) Close() error {
	if s.cleanup != nil {
		return s.cleanup.close()
	}
	return nil
}
func (s *OSS) check(key string) error {
	if !validKey(key) || !strings.HasPrefix(key, s.prefix) {
		return fmt.Errorf("对象键不属于当前存储范围")
	}
	return nil
}
func ossError(err error) error {
	var serviceErr *oss.ServiceError
	if errors.As(err, &serviceErr) && serviceErr.StatusCode == 404 {
		return os.ErrNotExist
	}
	if err != nil {
		return fmt.Errorf("OSS 文件操作失败")
	}
	return nil
}
func (s *OSS) Put(ctx context.Context, key string, input io.Reader, contentType string) (int64, error) {
	if err := s.check(key); err != nil {
		return 0, err
	}
	// SDK 签名与重试需要可回放的请求体，暂存到磁盘，避免大上传占满内存。
	file, err := os.CreateTemp("", "knownote-oss-*")
	if err != nil {
		return 0, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	size, err := io.Copy(file, io.LimitReader(contextReader{ctx, input}, MaxUploadSize+1))
	if err != nil {
		return 0, err
	}
	if size > MaxUploadSize {
		return 0, ErrTooLarge
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	disposition := "inline"
	if !strings.HasSuffix(key, "/preview.pdf") && !strings.HasSuffix(key, ".mp3") {
		disposition = mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(key)})
	}
	_, err = s.client.PutObject(ctx, &oss.PutObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(key), Body: file, ContentType: oss.Ptr(contentType), ContentDisposition: oss.Ptr(disposition)})
	return size, ossError(err)
}
func (s *OSS) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := s.check(key); err != nil {
		return nil, err
	}
	result, err := s.client.GetObject(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(key)})
	if err != nil {
		return nil, ossError(err)
	}
	return result.Body, nil
}
func (s *OSS) Delete(ctx context.Context, key string) error {
	if err := s.check(key); err != nil {
		return err
	}
	_, err := s.client.DeleteObject(ctx, &oss.DeleteObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(key)})
	return ossError(err)
}
func (s *OSS) DeletePrefix(ctx context.Context, prefix string) error {
	if !strings.HasSuffix(prefix, "/") {
		return fmt.Errorf("清理前缀必须以斜杠结尾")
	}
	if err := s.check(strings.TrimSuffix(prefix, "/")); err != nil {
		return err
	}
	var token *string
	for {
		result, err := s.client.ListObjectsV2(ctx, &oss.ListObjectsV2Request{Bucket: oss.Ptr(s.bucket), Prefix: oss.Ptr(prefix), ContinuationToken: token})
		if err != nil {
			return ossError(err)
		}
		for _, object := range result.Contents {
			if object.Key == nil || !strings.HasPrefix(*object.Key, prefix) {
				return fmt.Errorf("OSS 返回的对象键超出清理范围")
			}
			if err := s.Delete(ctx, *object.Key); err != nil {
				return err
			}
		}
		if !result.IsTruncated {
			return nil
		}
		if result.NextContinuationToken == nil || (token != nil && *token == *result.NextContinuationToken) {
			return fmt.Errorf("OSS 分页响应无效")
		}
		token = result.NextContinuationToken
	}
}
func (s *OSS) Stat(ctx context.Context, key string) (Info, error) {
	if err := s.check(key); err != nil {
		return Info{}, err
	}
	result, err := s.client.HeadObject(ctx, &oss.HeadObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(key)})
	if err != nil {
		return Info{}, ossError(err)
	}
	info := Info{Size: result.ContentLength}
	if result.ContentType != nil {
		info.ContentType = *result.ContentType
	}
	if result.LastModified != nil {
		info.ModTime = *result.LastModified
	}
	return info, nil
}
func (s *OSS) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if err := s.check(key); err != nil {
		return "", err
	}
	request := &oss.GetObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(key)}
	if headers, ok := ctx.Value(responseHeadersKey{}).(responseHeaders); ok {
		request.ResponseContentDisposition = oss.Ptr(headers.Disposition)
		request.ResponseContentType = oss.Ptr(headers.ContentType)
	}
	result, err := s.client.Presign(ctx, request, oss.PresignExpires(ttl))
	if err != nil {
		return "", ossError(err)
	}
	return result.URL, nil
}
func (s *OSS) DeleteSourceFiles(ctx context.Context, userID, sourceID string) error {
	if !validID(userID) || !validID(sourceID) {
		return fmt.Errorf("文件归属无效")
	}
	return s.schedule(ctx, []cleanupOperation{{Key: SourcePrefix(s, userID, sourceID), Prefix: true}})
}
func (s *OSS) DeleteNotebookFiles(ctx context.Context, userID, notebookID string, keys []string) error {
	if !validID(userID) || !validID(notebookID) {
		return fmt.Errorf("文件归属无效")
	}
	audio := s.Key(AudioPath(userID, notebookID))
	prefixes := map[string]bool{}
	for _, key := range keys {
		if key == audio {
			continue
		}
		relative := strings.TrimPrefix(key, s.prefix)
		parts := strings.Split(relative, "/")
		if len(parts) != 4 || parts[0] != "uploads" || parts[1] != userID || !validID(parts[2]) || s.check(key) != nil {
			return fmt.Errorf("文件路径不属于当前用户的存储范围")
		}
		prefixes[SourcePrefix(s, userID, parts[2])] = true
	}
	operations := []cleanupOperation{{Key: audio}}
	for prefix := range prefixes {
		operations = append(operations, cleanupOperation{Key: prefix, Prefix: true})
	}
	return s.schedule(ctx, operations)
}
func (s *OSS) SaveAudio(ctx context.Context, userID, notebookID string, input io.Reader) (string, error) {
	if !validID(userID) || !validID(notebookID) {
		return "", fmt.Errorf("音频归属无效")
	}
	key := s.Key(AudioPath(userID, notebookID))
	_, err := s.Put(ctx, key, input, "audio/mpeg")
	return key, err
}

func (s *OSS) schedule(ctx context.Context, operations []cleanupOperation) error {
	if s.cleanup != nil {
		return s.cleanup.schedule(ctx, operations)
	}
	for _, op := range operations {
		var err error
		if op.Prefix {
			err = s.DeletePrefix(ctx, op.Key)
		} else {
			err = s.Delete(ctx, op.Key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
