package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name         string
		status, code int
		data         any
		message      string
	}{
		{"成功", 200, 0, map[string]string{"status": "ok"}, "ok"},
		{"删除", 200, 0, nil, "ok"},
		{"错误", 404, CodeNotFound, nil, "资源不存在"},
	} {
		t.Run(test.name, func(t *testing.T) {
			writer := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(writer)
			if test.status == http.StatusOK {
				OK(ctx, test.data)
			} else {
				Fail(ctx, test.status, test.code, test.message)
			}
			var body Envelope
			if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if writer.Code != test.status || body.Code != test.code || body.Message != test.message || (test.data == nil && body.Data != nil) {
				t.Fatal("统一响应不符合约定")
			}
			if test.code != 0 && !ctx.IsAborted() {
				t.Fatal("错误响应应终止处理")
			}
		})
	}
}
