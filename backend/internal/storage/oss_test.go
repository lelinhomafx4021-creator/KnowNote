package storage

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/google/uuid"
	"knownote/internal/config"
)

func TestOSSDriver(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	signedRequests := 0
	listCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if strings.HasPrefix(r.Header.Get("Authorization"), "OSS4-HMAC-SHA256 ") {
			signedRequests++
		} else {
			t.Error("OSS 请求未使用 SDK 签名")
		}
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		if r.URL.Query().Get("list-type") == "2" {
			listCalls++
			prefix := r.URL.Query().Get("prefix")
			var keys []string
			for key := range objects {
				if strings.HasPrefix(key, prefix) {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			// 模拟每页一个对象，删除本页后仍可继续以 token 取下一页。
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<ListBucketResult><Name>bucket</Name><IsTruncated>`)
			fmt.Fprint(w, len(keys) > 1)
			fmt.Fprint(w, `</IsTruncated>`)
			if len(keys) > 1 {
				fmt.Fprint(w, `<NextContinuationToken>`+url.QueryEscape(keys[0])+`</NextContinuationToken>`)
			}
			if len(keys) > 0 {
				fmt.Fprint(w, `<Contents><Key>`)
				xml.EscapeText(w, []byte(keys[0]))
				fmt.Fprint(w, `</Key><Size>`+strconv.Itoa(len(objects[keys[0]]))+`</Size></Contents>`)
			}
			fmt.Fprint(w, `</ListBucketResult>`)
			return
		}
		switch r.Method {
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			objects[key] = data
			w.Header().Set("ETag", `"fixture"`)
			w.WriteHeader(200)
		case http.MethodGet, http.MethodHead:
			data, ok := objects[key]
			if !ok {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(404)
				fmt.Fprint(w, `<Error><Code>NoSuchKey</Code><Message>不存在</Message></Error>`)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			if r.Method == http.MethodGet {
				w.Write(data)
			}
		case http.MethodDelete:
			delete(objects, key)
			w.WriteHeader(204)
		default:
			t.Error("未知 OSS 方法")
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	cfg := oss.LoadDefaultConfig().WithRegion("cn-hangzhou").WithEndpoint(server.URL).WithUsePathStyle(true).WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test-id", "test-secret"))
	store := &OSS{client: oss.NewClient(cfg), bucket: "bucket", prefix: "knownote/"}
	user, source, book := uuid.NewString(), uuid.NewString(), uuid.NewString()
	key := OriginalKey(store, user, source, "pdf")
	ctx := context.Background()
	size, err := store.Put(ctx, key, strings.NewReader("%PDF-测试"), "application/pdf")
	if err != nil || size != 11 {
		t.Fatalf("Put：%d %v", size, err)
	}
	stream, err := store.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(stream)
	stream.Close()
	if string(data) != "%PDF-测试" {
		t.Fatal("Get 内容错误")
	}
	info, err := store.Stat(ctx, key)
	if err != nil || info.Size != 11 || info.ContentType != "application/pdf" {
		t.Fatalf("Stat 错误：%+v %v", info, err)
	}
	file, cleanup, err := Materialize(ctx, store, key, "pdf")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := file.Name()
	cleanup()
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("临时原件未删除")
	}
	signed, err := store.SignedURL(WithResponseHeaders(ctx, "inline", "application/pdf"), key, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(signed)
	if err != nil || u.Query().Get("x-oss-expires") != "600" || u.Query().Get("x-oss-signature") == "" || u.Query().Get("response-content-disposition") != "inline" {
		t.Fatal("签名 URL 缺少有效期、响应头或签名")
	}
	preview := SourcePrefix(store, user, source) + "preview.pdf"
	if _, err := store.Put(ctx, preview, strings.NewReader("preview"), "application/pdf"); err != nil {
		t.Fatal(err)
	}
	audio, err := store.SaveAudio(ctx, user, book, strings.NewReader("mp3"))
	if err != nil || audio != store.Key(AudioPath(user, book)) {
		t.Fatal(err)
	}
	if err := store.DeleteSourceFiles(ctx, user, source); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stat(ctx, key); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("删除来源后原件残留")
	}
	if err := store.DeleteNotebookFiles(ctx, user, book, []string{audio}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, audio); err != nil {
		t.Fatal("重复删除应安全")
	}
	if err := store.DeleteNotebookFiles(ctx, uuid.NewString(), book, []string{key}); err == nil {
		t.Fatal("越权清理未拒绝")
	}
	if _, err := store.Open(ctx, "../outside"); err == nil {
		t.Fatal("越界对象键未拒绝")
	}
	if len(objects) != 0 || signedRequests < 8 || listCalls < 2 {
		t.Fatalf("对象清理、签名或分页未覆盖：%d %d %d", len(objects), signedRequests, listCalls)
	}
}
func TestObjectCleanupRestart(t *testing.T) {
	dir := t.TempDir()
	var fail = true
	var count int
	execute := func(context.Context, cleanupOperation) error {
		count++
		if fail {
			return fmt.Errorf("模拟断网")
		}
		return nil
	}
	q, err := newObjectCleanup(context.Background(), dir, nil, execute)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.schedule(context.Background(), []cleanupOperation{{Key: "测试对象"}}); err == nil {
		t.Fatal("应记录清理失败")
	}
	q.close()
	fail = false
	q, err = newObjectCleanup(context.Background(), dir, nil, execute)
	if err != nil {
		t.Fatal(err)
	}
	defer q.close()
	q.retry(context.Background())
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 || count < 2 {
		t.Fatal("重启未恢复清理日志")
	}
}
func TestRealOSS(t *testing.T) {
	if os.Getenv("T7_REAL_OSS") != "1" || os.Getenv("OSS_ACCESS_KEY_ID") == "" || os.Getenv("OSS_ACCESS_KEY_SECRET") == "" {
		t.Skip("未启用真实 OSS 验证或未配置凭据")
	}
	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = t.TempDir()
	store, err := NewOSS(context.Background(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	key := store.Key("uploads/" + uuid.NewString() + "/" + uuid.NewString() + "/original.txt")
	defer store.Delete(context.Background(), key)
	if _, err := store.Put(ctx, key, strings.NewReader("知页 OSS 测试"), "text/plain"); err != nil {
		t.Fatal("真实 OSS 上传失败")
	}
	stream, err := store.Open(ctx, key)
	if err != nil {
		t.Fatal("真实 OSS 读取失败")
	}
	stream.Close()
}
