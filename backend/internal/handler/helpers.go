package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"knownote/internal/service"
	"knownote/pkg/response"
)

// BindJSON 限制普通 JSON 请求为 1MB，拒绝未知字段、空值和尾随内容。
// T2 的 multipart 上传应使用单独的大小限制。
func BindJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil || len(raw) == 0 || raw[0] != '{' {
		invalid(c, "请求体必须是有效的 JSON 对象")
		return false
	}
	// RawMessage 用于确认顶层类型，再对对象进行严格字段校验。
	strict := json.NewDecoder(strings.NewReader(string(raw)))
	strict.DisallowUnknownFields()
	if err := strict.Decode(target); err != nil {
		invalid(c, "请求包含未知字段或错误类型")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		invalid(c, "请求体只能包含一个 JSON 对象")
		return false
	}
	if err := binding.Validator.ValidateStruct(target); err != nil {
		invalid(c, "请求参数不符合要求")
		return false
	}
	return true
}

func ParamID(c *gin.Context) (string, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		invalid(c, "资源 ID 必须是有效的 UUID")
		return "", false
	}
	return id.String(), true
}

func invalid(c *gin.Context, message string) {
	response.Fail(c, http.StatusBadRequest, response.CodeInvalid, message)
}

func textValid(value string, min, max int) bool {
	count := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && count >= min && count <= max
}

func (h *Handler) result(c *gin.Context, data any, err error) {
	if err == nil {
		response.OK(c, data)
		return
	}
	var business *service.Error
	if errors.As(err, &business) {
		response.Fail(c, business.Status, business.Code, business.Message)
		return
	}
	// 不向客户端或日志输出底层错误，防止数据库参数和凭据泄露。
	h.Logger.Error("业务处理失败", "method", c.Request.Method, "path", c.Request.URL.Path)
	response.Fail(c, http.StatusInternalServerError, response.CodeInternal, "服务暂时不可用")
}
