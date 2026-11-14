package service

import (
	"errors"
	"log/slog"
	"net/http"

	"gorm.io/gorm"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/storage"
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
	Provider     *llm.Provider
	Admin        *AdminService
	AdminEmails  []string
	Models       llm.ModelCatalog
	Usage        *llm.Guard
	DB           *gorm.DB
	Tokens       *middleware.Tokens
	DefaultModel string
	Files        NotebookFileCleaner
	Logger       *slog.Logger
	Storage      storage.Store
	Ingest       interface{ Notify() }
}

func New(database *gorm.DB, tokens *middleware.Tokens, defaultModel string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{DB: database, Tokens: tokens, DefaultModel: defaultModel, Models: llm.ModelCatalog{Chat: []llm.ChatModel{{ID: defaultModel, Name: defaultModel, Provider: "fake"}}}, Logger: logger}
}

func (s *Service) ModelCatalog() llm.ModelCatalog {
	if s.Provider != nil {
		return llm.Catalog(s.Provider.Current().Config)
	}
	return s.Models
}
func (s *Service) CurrentModel() string {
	if s.Provider != nil {
		return s.Provider.Current().Config.ChatModel
	}
	return s.DefaultModel
}
