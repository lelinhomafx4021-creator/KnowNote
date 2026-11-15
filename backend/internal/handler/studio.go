package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"knownote/internal/exporter"
	"knownote/internal/middleware"
	"knownote/internal/service"
	"knownote/internal/service/studio"
)

func StudioRoutes(core *service.Service, workshop *studio.Service) Registrar {
	h := &Handler{Service: core, Logger: core.Logger}
	return func(api *gin.RouterGroup) {
		api.GET("/notebooks/:id/notes", h.Notes)
		api.POST("/notebooks/:id/notes", h.CreateNote)
		api.PATCH("/notes/:id", h.UpdateNote)
		api.DELETE("/notes/:id", h.DeleteNote)
		api.GET("/notes/:id/export", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			format := c.Query("format")
			if format != "" {
				if _, _, err := exporter.ContentType(format); err != nil {
					invalid(c, err.Error())
					return
				}
			}
			note, err := h.Service.Note(c.Request.Context(), middleware.UserID(c), id)
			if err != nil {
				h.result(c, nil, err)
				return
			}
			data, name, mimeType, err := exporter.ExportNote(c.Request.Context(), note, format)
			h.serveExported(c, data, name, mimeType, format, err)
		})
		api.GET("/notebooks/:id/artifacts", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			data, err := workshop.Artifacts(c.Request.Context(), middleware.UserID(c), id)
			h.result(c, data, err)
		})
		api.POST("/notebooks/:id/artifacts", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			var input struct {
				Kind      string   `json:"kind"`
				Topic     string   `json:"topic"`
				Preset    string   `json:"preset"`
				SourceIDs []string `json:"sourceIds"`
			}
			if !BindJSON(c, &input) {
				return
			}
			data, err := workshop.CreateArtifactWithRequest(c.Request.Context(), middleware.UserID(c), id, studio.CreateArtifactRequest{
				Kind:      input.Kind,
				Topic:     input.Topic,
				Preset:    input.Preset,
				SourceIDs: input.SourceIDs,
			})
			h.result(c, data, err)
		})
		api.GET("/artifacts/:id/export", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			format := c.Query("format")
			if format != "" {
				if _, _, err := exporter.ContentType(format); err != nil {
					invalid(c, err.Error())
					return
				}
			}
			art, err := workshop.Artifact(c.Request.Context(), middleware.UserID(c), id)
			if err != nil {
				h.result(c, nil, err)
				return
			}
			data, name, mimeType, err := exporter.ExportArtifact(c.Request.Context(), art, format)
			h.serveExported(c, data, name, mimeType, format, err)
		})
		api.DELETE("/artifacts/:id", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			h.result(c, nil, workshop.DeleteArtifact(c.Request.Context(), middleware.UserID(c), id))
		})
		api.GET("/notebooks/:id/audio", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			data, err := workshop.Audio(c.Request.Context(), middleware.UserID(c), id)
			h.result(c, data, err)
		})
		api.POST("/notebooks/:id/audio", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			data, err := workshop.StartAudio(c.Request.Context(), middleware.UserID(c), id)
			h.result(c, data, err)
		})
		api.GET("/files/audio/:file", func(c *gin.Context) {
			file, err := workshop.AudioFile(c.Request.Context(), middleware.UserID(c), c.Param("file"))
			if err != nil {
				h.result(c, nil, err)
				return
			}
			h.serveStored(c, workshop.Files, file, c.Param("file"), "audio/mpeg", "inline")
		})
	}
}

func (h *Handler) Notes(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	data, err := h.Service.Notes(c.Request.Context(), middleware.UserID(c), id)
	h.result(c, data, err)
}

func validNoteText(c *gin.Context, title, content *string) bool {
	if title != nil {
		*title = strings.TrimSpace(*title)
		if !textValid(*title, 1, 500) || strings.ContainsRune(*title, '\x00') {
			invalid(c, "笔记标题需要1到500个字符")
			return false
		}
	}
	if content != nil && (!textValid(*content, 0, 200000) || strings.ContainsRune(*content, '\x00')) {
		invalid(c, "笔记正文不能超过200000个字符或包含空字符")
		return false
	}
	return true
}

func (h *Handler) CreateNote(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	var input struct {
		Title   *string `json:"title"`
		Content *string `json:"content"`
		Origin  string  `json:"origin"`
	}
	if !BindJSON(c, &input) {
		return
	}
	if input.Title == nil || input.Content == nil {
		invalid(c, "请提供笔记标题和正文")
		return
	}
	if !validNoteText(c, input.Title, input.Content) {
		return
	}
	data, err := h.Service.CreateNote(c.Request.Context(), middleware.UserID(c), id, *input.Title, *input.Content, input.Origin)
	h.result(c, data, err)
}

func (h *Handler) UpdateNote(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	var input struct {
		Title   *string `json:"title"`
		Content *string `json:"content"`
	}
	if !BindJSON(c, &input) || !validNoteText(c, input.Title, input.Content) {
		return
	}
	updates := map[string]any{}
	if input.Title != nil {
		updates["title"] = *input.Title
	}
	if input.Content != nil {
		updates["content"] = *input.Content
	}
	data, err := h.Service.UpdateNote(c.Request.Context(), middleware.UserID(c), id, updates)
	h.result(c, data, err)
}

func (h *Handler) DeleteNote(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	h.result(c, nil, h.Service.DeleteNote(c.Request.Context(), middleware.UserID(c), id))
}

func (h *Handler) serveExported(c *gin.Context, data []byte, name, mimeType, requestedFormat string, err error) {
	if err != nil {
		if strings.Contains(err.Error(), "LibreOffice") {
			h.result(c, nil, &service.Error{Status: http.StatusServiceUnavailable, Code: 50301, Message: err.Error()})
			return
		}
		h.result(c, nil, err)
		return
	}
	ext := exporter.FormatDOCX
	if requestedFormat != "" {
		ext = strings.ToLower(requestedFormat)
	} else if mimeType == "application/pdf" {
		ext = exporter.FormatPDF
	} else if strings.Contains(mimeType, "presentationml") {
		ext = exporter.FormatPPTX
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", exporter.Disposition(name, ext, c.Query("inline") == "1"))
	c.Data(http.StatusOK, mimeType, data)
}
