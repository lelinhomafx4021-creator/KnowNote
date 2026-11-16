package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"knownote/internal/handler"
	"knownote/internal/middleware"
	"knownote/internal/service"
	"knownote/internal/storage"
)

func avatarAPI(t *testing.T, files storage.Store) *testAPI {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database := sourceDatabase(t)
	core := service.New(database, middleware.NewTokens(strings.Repeat("a", 32)), "fake", slog.New(slog.NewTextHandler(io.Discard, nil)))
	core.Storage = files
	return &testAPI{t: t, database: database, router: handler.NewRouter(core, nil)}
}

func avatarPNG(t *testing.T, fill color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, fill)
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func avatarUpload(t *testing.T, a *testAPI, token, field, name string, data []byte, status int) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile(field, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/me/avatar", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, request)
	if w.Code != status {
		t.Fatalf("头像上传状态码 %d，预期 %d：%s", w.Code, status, w.Body.String())
	}
	return w
}

func avatarGet(t *testing.T, a *testAPI, path, token string, status int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, request)
	if w.Code != status {
		t.Fatalf("头像读取状态码 %d，预期 %d：%s", w.Code, status, w.Body.String())
	}
	return w
}

func TestT16AvatarLocalLifecycle(t *testing.T) {
	files, err := storage.New(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = files.Close() })
	a := avatarAPI(t, files)
	owner, other := a.register(), a.register()
	path := "/api/files/avatar/" + owner.User.ID
	avatarGet(t, a, path, owner.Token, 404)
	red := avatarPNG(t, color.RGBA{R: 255, A: 255})
	w := avatarUpload(t, a, owner.Token, "avatar", "用户自选文件.png", red, 200)
	var response struct {
		Code int              `json:"code"`
		Data service.UserView `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Code != 0 || response.Data.AvatarURL != path {
		t.Fatalf("上传响应不符合契约：%v %s", err, w.Body.String())
	}
	me := decode[service.UserView](t, a.call("GET", "/api/me", owner.Token, "", 200))
	if me.AvatarURL != path {
		t.Fatal("GET /me 没有返回持久化的相对头像地址")
	}
	w = avatarGet(t, a, path+"?token="+owner.Token, "", 200)
	if w.Header().Get("Content-Type") != "image/jpeg" || w.Header().Get("Cache-Control") != "private, max-age=300" {
		t.Fatalf("头像响应头错误：%v", w.Header())
	}
	img, err := jpeg.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil || img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("头像不是 256×256 JPEG：%v", err)
	}
	avatarGet(t, a, path, other.Token, 404)
	avatarGet(t, a, "/api/files/avatar/"+other.User.ID, owner.Token, 404)
	blue := avatarPNG(t, color.RGBA{B: 255, A: 255})
	avatarUpload(t, a, owner.Token, "avatar", "改名的图片.txt", blue, 200)
	w = avatarGet(t, a, path+"?v=2", owner.Token, 200)
	img, err = jpeg.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	r, _, b, _ := img.At(128, 128).RGBA()
	if r > 5*257 || b < 250*257 {
		t.Fatal("覆盖上传没有替换原头像")
	}
	removed := decode[service.UserView](t, a.call("DELETE", "/api/me/avatar", owner.Token, "", 200))
	if removed.AvatarURL != "" || decode[service.UserView](t, a.call("GET", "/api/me", owner.Token, "", 200)).AvatarURL != "" {
		t.Fatal("删除后头像字段没有清空")
	}
	avatarGet(t, a, path, owner.Token, 404)
	if _, err := files.Stat(context.Background(), "avatars/"+owner.User.ID+"/avatar.jpg"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除没有移除存储对象：%v", err)
	}
	a.call("DELETE", "/api/me/avatar", owner.Token, "", 200)
}

func TestT16AvatarValidation(t *testing.T) {
	files := &avatarMemoryStore{objects: make(map[string][]byte)}
	a := avatarAPI(t, files)
	owner := a.register()
	pngData := avatarPNG(t, color.White)
	for _, tc := range []struct {
		name, field, message string
		data                 []byte
	}{
		{"伪造.jpg", "avatar", "请上传 PNG 或 JPEG 图片", []byte("文本不是图片")},
		{"损坏.png", "avatar", "请上传 PNG 或 JPEG 图片", pngData[:len(pngData)-15]},
		{"图片.webp", "avatar", "请上传 PNG 或 JPEG 图片", []byte("RIFF0000WEBPVP8 ")},
		{"图片.png", "file", "请选择头像图片", pngData},
		{"超限.png", "avatar", "头像不能超过 2MB", make([]byte, service.MaxAvatarSize+1)},
		{"请求超限.png", "avatar", "头像不能超过 2MB", make([]byte, service.MaxAvatarSize+(128<<10))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := avatarUpload(t, a, owner.Token, tc.field, tc.name, tc.data, 400)
			var response struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Code != 40001 || response.Message != tc.message {
				t.Fatalf("头像校验错误响应不符：%v %s", err, w.Body.String())
			}
		})
	}
	if files.puts != 0 {
		t.Fatal("无效上传不应进入存储层")
	}
}

// avatarMemoryStore 没有本地目录，验证业务只通过统一对象接口操作。
type avatarMemoryStore struct {
	storage.Store
	objects                map[string][]byte
	puts, deletes          int
	putKey, deleteKey      string
	contentType, signedKey string
	ttl                    time.Duration
	putErr, deleteErr      error
}

func (s *avatarMemoryStore) Key(key string) string { return "测试前缀/" + key }
func (s *avatarMemoryStore) Driver() string        { return "oss" }
func (s *avatarMemoryStore) Put(_ context.Context, key string, input io.Reader, contentType string) (int64, error) {
	s.puts++
	s.putKey, s.contentType = key, contentType
	if s.putErr != nil {
		return 0, s.putErr
	}
	data, err := io.ReadAll(input)
	s.objects[key] = data
	return int64(len(data)), err
}
func (s *avatarMemoryStore) Delete(_ context.Context, key string) error {
	s.deletes++
	s.deleteKey = key
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.objects, key)
	return nil
}
func (s *avatarMemoryStore) Stat(_ context.Context, key string) (storage.Info, error) {
	data, ok := s.objects[key]
	if !ok {
		return storage.Info{}, os.ErrNotExist
	}
	return storage.Info{Size: int64(len(data)), ContentType: "image/jpeg"}, nil
}
func (s *avatarMemoryStore) SignedURL(_ context.Context, key string, ttl time.Duration) (string, error) {
	s.signedKey, s.ttl = key, ttl
	return "https://avatar.example.test/signed.jpg", nil
}

func TestT16AvatarUnifiedStoreAndRedirect(t *testing.T) {
	files := &avatarMemoryStore{objects: make(map[string][]byte)}
	a := avatarAPI(t, files)
	owner := a.register()
	path := "/api/files/avatar/" + owner.User.ID
	data := avatarPNG(t, color.White)
	files.putErr = errors.New("模拟存储写入失败")
	avatarUpload(t, a, owner.Token, "avatar", "图片.png", data, 500)
	if decode[service.UserView](t, a.call("GET", "/api/me", owner.Token, "", 200)).AvatarURL != "" {
		t.Fatal("存储失败仍更新了头像字段")
	}
	files.putErr = nil
	avatarUpload(t, a, owner.Token, "avatar", "图片.png", data, 200)
	avatarUpload(t, a, owner.Token, "avatar", "覆盖.png", data, 200)
	key := "测试前缀/avatars/" + owner.User.ID + "/avatar.jpg"
	if files.puts != 3 || files.putKey != key || files.contentType != "image/jpeg" || len(files.objects) != 1 {
		t.Fatalf("未通过 Put 覆盖同一对象：%+v", files)
	}
	if _, err := jpeg.Decode(bytes.NewReader(files.objects[key])); err != nil {
		t.Fatalf("存储内容没有规范化为 JPEG：%v", err)
	}
	w := avatarGet(t, a, path+"?token="+owner.Token, "", 302)
	if w.Header().Get("Location") != "https://avatar.example.test/signed.jpg" || w.Header().Get("Cache-Control") != "private, max-age=300" || files.signedKey != key || files.ttl != 10*time.Minute {
		t.Fatal("OSS 头像跳转、缓存或签名有效期不符")
	}
	files.deleteErr = errors.New("模拟存储删除失败")
	a.call("DELETE", "/api/me/avatar", owner.Token, "", 500)
	if decode[service.UserView](t, a.call("GET", "/api/me", owner.Token, "", 200)).AvatarURL != path {
		t.Fatal("存储删除失败仍清空了头像字段")
	}
	files.deleteErr = nil
	a.call("DELETE", "/api/me/avatar", owner.Token, "", 200)
	if files.deletes != 2 || files.deleteKey != key || len(files.objects) != 0 {
		t.Fatal("没有通过 Delete 删除统一对象键")
	}
	avatarGet(t, a, path, owner.Token, 404)
	avatarUpload(t, a, owner.Token, "avatar", "图片.png", data, 200)
	delete(files.objects, key)
	avatarGet(t, a, path, owner.Token, 404)
}

func TestT16AvatarRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core := service.New(nil, middleware.NewTokens(strings.Repeat("a", 32)), "fake", slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := &testAPI{t: t, router: handler.NewRouter(core, nil)}
	for _, method := range []string{"POST", "DELETE"} {
		a.call(method, "/api/me/avatar", "", "", 401)
	}
	avatarGet(t, a, "/api/files/avatar/00000000-0000-4000-8000-000000000016", "", 401)
	avatarGet(t, a, "/api/files/avatar/00000000-0000-4000-8000-000000000016?token=无效凭证", "", 401)
}
