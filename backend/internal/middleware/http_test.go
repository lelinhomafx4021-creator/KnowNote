package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSRecoveryAndPrivateLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	router := gin.New()
	router.Use(RequestLog(logger), Recover(logger), CORS([]string{"http://localhost:5173"}))
	router.GET("/panic", func(*gin.Context) { panic("请求中的敏感信息") })
	for _, test := range []struct {
		method, origin string
		status         int
	}{
		{"GET", "", 500}, {"OPTIONS", "http://localhost:5173", 204}, {"GET", "https://untrusted.example", 403},
	} {
		request := httptest.NewRequest(test.method, "/panic?token=private-log-marker", nil)
		request.Header.Set("Origin", test.origin)
		writer := httptest.NewRecorder()
		router.ServeHTTP(writer, request)
		if writer.Code != test.status {
			t.Fatalf("状态码为 %d，预期 %d", writer.Code, test.status)
		}
		if test.method == http.MethodOptions && writer.Header().Get("Access-Control-Allow-Origin") != test.origin {
			t.Fatal("预检响应缺少允许来源")
		}
	}
	if strings.Contains(logs.String(), "private-log-marker") || strings.Contains(logs.String(), "请求中的敏感信息") {
		t.Fatal("日志泄露了敏感内容")
	}
}
