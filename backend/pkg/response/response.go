package response

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

const (
	CodeInvalid      = 40001
	CodeUnauthorized = 40101
	CodeForbidden    = 40301
	CodeNotFound     = 40401
	CodeConflict     = 40901
	CodeInternal     = 50001
)

type Envelope struct {
	Code    int    `json:"code"`
	Data    any    `json:"data"`
	Message string `json:"message"`
}

func OK(c *gin.Context, data any) { c.JSON(http.StatusOK, Envelope{Data: data, Message: "ok"}) }

func Fail(c *gin.Context, status, code int, message string) {
	c.AbortWithStatusJSON(status, Envelope{Code: code, Data: nil, Message: message})
}
