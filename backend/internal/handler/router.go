package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"knownote/internal/middleware"
	"knownote/internal/service"
	"knownote/pkg/response"
)

type Handler struct {
	Service *service.Service
	Logger  *slog.Logger
}

// Registrar 为 T2 来源、T3 对话和概要、T4 笔记和工作室提供受保护路由组。
// 注册器通过闭包持有各任务自己的 handler 与依赖。
type Registrar func(api *gin.RouterGroup)

func NewRouter(s *service.Service, origins []string, registrars ...Registrar) *gin.Engine {
	h := &Handler{Service: s, Logger: s.Logger}
	router := gin.New()
	router.Use(middleware.RequestLog(h.Logger), middleware.Recover(h.Logger), middleware.CORS(origins))
	router.NoRoute(func(c *gin.Context) { response.Fail(c, http.StatusNotFound, response.CodeNotFound, "接口不存在") })
	router.HandleMethodNotAllowed = true
	router.NoMethod(func(c *gin.Context) {
		response.Fail(c, http.StatusMethodNotAllowed, response.CodeInvalid, "请求方法不受支持")
	})
	api := router.Group("/api")
	api.GET("/health", h.Health)
	auth := api.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	protected := api.Group("", middleware.Auth(s.Tokens))
	protected.GET("/me", h.Me)
	protected.GET("/me/login-logs", h.LoginLogs)
	protected.GET("/tags", h.Tags)
	protected.POST("/tags", h.SaveTag)
	protected.PATCH("/tags/:id", h.SaveTag)
	protected.DELETE("/tags/:id", h.DeleteTag)
	protected.PUT("/notebooks/:id/tags", h.SetNotebookTags)
	protected.GET("/models", func(c *gin.Context) { response.OK(c, s.Models) })
	protected.GET("/me/usage", func(c *gin.Context) {
		if s.Usage == nil {
			response.OK(c, gin.H{})
			return
		}
		data, err := s.Usage.Usage(c.Request.Context(), middleware.UserID(c))
		h.result(c, data, err)
	})
	protected.GET("/me/settings", h.Settings)
	protected.PATCH("/me/settings", h.UpdateSettings)
	protected.GET("/notebooks", h.Notebooks)
	protected.POST("/notebooks", h.CreateNotebook)
	protected.GET("/notebooks/:id", h.Notebook)
	protected.PATCH("/notebooks/:id", h.UpdateNotebook)
	protected.DELETE("/notebooks/:id", h.DeleteNotebook)
	// T2 / T3 / T4 在 main.go 按任务顺序注入注册器，自动继承 JWT 鉴权。
	for _, register := range registrars {
		register(protected)
	}
	return router
}

func (h *Handler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	sqlDB, err := h.Service.DB.DB()
	if err == nil {
		err = sqlDB.PingContext(ctx)
	}
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeInternal, "数据库暂时不可用")
		return
	}
	response.OK(c, gin.H{"status": "ok", "db": "ok"})
}
