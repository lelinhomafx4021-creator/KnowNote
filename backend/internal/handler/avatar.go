package handler

import (
	"errors"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"
	"knownote/internal/middleware"
	"knownote/internal/service"
)

func (h *Handler) UploadAvatar(c *gin.Context) {
	// 额外空间只留给 multipart 边界与字段头，文件本身仍严格限制为 2MB。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxAvatarSize+(64<<10))
	err := c.Request.ParseMultipartForm(service.MaxAvatarSize + (64 << 10))
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || errors.Is(err, multipart.ErrMessageTooLarge) {
			h.result(c, nil, service.ErrAvatarTooLarge)
		} else {
			h.result(c, nil, service.ErrAvatarFormat)
		}
		return
	}
	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		invalid(c, "请选择头像图片")
		return
	}
	defer file.Close()
	if header.Size > service.MaxAvatarSize {
		h.result(c, nil, service.ErrAvatarTooLarge)
		return
	}
	data, err := h.Service.UploadAvatar(c.Request.Context(), middleware.UserID(c), file)
	h.result(c, data, err)
}

func (h *Handler) DeleteAvatar(c *gin.Context) {
	data, err := h.Service.DeleteAvatar(c.Request.Context(), middleware.UserID(c))
	h.result(c, data, err)
}

func (h *Handler) Avatar(c *gin.Context) {
	key, err := h.Service.AvatarFile(c.Request.Context(), middleware.UserID(c), c.Param("userId"))
	if err != nil {
		h.result(c, nil, err)
		return
	}
	c.Header("Cache-Control", "private, max-age=300")
	h.serveStored(c, h.Service.Storage, key, "avatar.jpg", "image/jpeg", "inline")
}
