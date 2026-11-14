package handler

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"knownote/internal/middleware"
	"knownote/internal/service"
	"knownote/internal/storage"
)

func (h *Handler) sourceFile(c *gin.Context, preview bool) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	key, name, contentType, err := h.Service.SourceFile(c.Request.Context(), middleware.UserID(c), id, preview)
	if err != nil {
		h.result(c, nil, err)
		return
	}
	disposition := "attachment"
	if preview {
		disposition = "inline"
	}
	h.serveStored(c, h.Service.Storage, key, name, contentType, disposition)
}
func (h *Handler) serveStored(c *gin.Context, files storage.Store, key, name, contentType, disposition string) {
	ctx := storage.WithResponseHeaders(c.Request.Context(), mime.FormatMediaType(disposition, map[string]string{"filename": name}), contentType)
	info, err := files.Stat(ctx, key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = service.ErrNotFound
		}
		h.result(c, nil, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	if files.Driver() == "oss" {
		url, err := files.SignedURL(ctx, key, 10*time.Minute)
		if err != nil {
			h.result(c, nil, err)
			return
		}
		c.Redirect(http.StatusFound, url)
		return
	}
	file, err := files.Open(ctx, key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = service.ErrNotFound
		}
		h.result(c, nil, err)
		return
	}
	defer file.Close()
	seeker, ok := file.(io.ReadSeeker)
	if !ok {
		h.result(c, nil, errors.New("文件存储不支持范围读取"))
		return
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	http.ServeContent(c.Writer, c.Request, name, info.ModTime, seeker)
}
