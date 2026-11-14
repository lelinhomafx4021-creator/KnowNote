package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"knownote/internal/middleware"
	"knownote/internal/service"
	"knownote/internal/storage"
)

func SourceRoutes(s *service.Service) Registrar {
	h := &Handler{Service: s, Logger: s.Logger}
	return func(api *gin.RouterGroup) {
		api.GET("/notebooks/:id/sources", h.Sources)
		api.POST("/notebooks/:id/sources/upload", h.UploadSource)
		api.POST("/notebooks/:id/sources", h.CreateSource)
		api.GET("/sources/:id", h.Source)
		api.GET("/sources/:id/preview", func(c *gin.Context) { h.sourceFile(c, true) })
		api.GET("/sources/:id/file", func(c *gin.Context) { h.sourceFile(c, false) })
		api.PATCH("/sources/:id", h.UpdateSource)
		api.DELETE("/sources/:id", h.DeleteSource)
	}
}

func (h *Handler) Sources(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	data, err := h.Service.Sources(c.Request.Context(), middleware.UserID(c), id)
	h.result(c, data, err)
}

func (h *Handler) Source(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	data, err := h.Service.Source(c.Request.Context(), middleware.UserID(c), id)
	h.result(c, data, err)
}

func (h *Handler) UploadSource(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	userID := middleware.UserID(c)
	if _, err := h.Service.Notebook(c.Request.Context(), userID, id); err != nil {
		h.result(c, nil, err)
		return
	}
	// 为 multipart 边界预留 1MB，文件本身仍严格限制 200MB。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, storage.MaxUploadSize+(1<<20))
	err := c.Request.ParseMultipartForm(8 << 20)
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		invalid(c, "上传失败，请使用 multipart 的 file 字段，文件不能超过 200MB")
		return
	}
	form := c.Request.MultipartForm
	if form == nil || len(form.File) != 1 || len(form.File["file"]) != 1 {
		invalid(c, "请通过 file 字段上传一个文件")
		return
	}
	header := form.File["file"][0]
	if header.Size > storage.MaxUploadSize {
		invalid(c, "文件不能超过 200MB")
		return
	}
	file, err := header.Open()
	if err != nil {
		invalid(c, "读取上传文件失败")
		return
	}
	defer file.Close()
	data, err := h.Service.UploadSource(c.Request.Context(), userID, id, header.Filename, file)
	h.result(c, data, err)
}

func (h *Handler) CreateSource(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	var input service.CreateSourceInput
	if !BindJSON(c, &input) {
		return
	}
	data, err := h.Service.CreateSource(c.Request.Context(), middleware.UserID(c), id, input)
	h.result(c, data, err)
}

func (h *Handler) UpdateSource(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	var input struct {
		Title *string `json:"title"`
	}
	if !BindJSON(c, &input) {
		return
	}
	if input.Title == nil {
		invalid(c, "请提供来源标题")
		return
	}
	data, err := h.Service.UpdateSource(c.Request.Context(), middleware.UserID(c), id, *input.Title)
	h.result(c, data, err)
}

func (h *Handler) DeleteSource(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	err := h.Service.DeleteSource(c.Request.Context(), middleware.UserID(c), id)
	h.result(c, nil, err)
}
