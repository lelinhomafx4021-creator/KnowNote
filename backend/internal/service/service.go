package service

import (
	"errors"
	"log/slog"
	"net/http"

	"gorm.io/gorm"
	"knownote/internal/middleware"
	"knownote/pkg/response"
)

type Error struct {
	Status  int
	Code    int
	Message string
}

func (e *Error) Error() string { return e.Message }

var ErrNotFound = &Error{http.StatusNotFound, response.CodeNotFound, "资源不存在"}
var ErrUnauthorized = &Error{http.StatusUnauthorized, response.CodeUnauthorized, "邮箱或密码错误"}
var ErrEmailConflict = &Error{http.StatusConflict, response.CodeConflict, "该邮箱已注册"}

func databaseError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

type Service struct {
	DB           *gorm.DB
	Tokens       *middleware.Tokens
	DefaultModel string
	Files        NotebookFileCleaner
	Logger       *slog.Logger
}

func New(database *gorm.DB, tokens *middleware.Tokens, defaultModel string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{DB: database, Tokens: tokens, DefaultModel: defaultModel, Logger: logger}
}
