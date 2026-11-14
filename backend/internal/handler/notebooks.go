package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"knownote/internal/middleware"
	"strings"
)

func (h *Handler) Notebooks(c *gin.Context) {
	tag := c.Query("tag")
	if tag != "" {
		parsed, err := uuid.Parse(tag)
		if err != nil || parsed == uuid.Nil {
			invalid(c, "标签 ID 必须是有效的 UUID")
			return
		}
		tag = parsed.String()
	}
	data, err := h.Service.Notebooks(c.Request.Context(), middleware.UserID(c), tag)
	h.result(c, data, err)
}

func (h *Handler) Notebook(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	data, err := h.Service.Notebook(c.Request.Context(), middleware.UserID(c), id)
	h.result(c, data, err)
}

func (h *Handler) CreateNotebook(c *gin.Context) {
	var input struct {
		Title string `json:"title"`
	}
	if !BindJSON(c, &input) {
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	if !textValid(input.Title, 0, 200) {
		invalid(c, "标题不能超过 200 个字符")
		return
	}
	data, err := h.Service.CreateNotebook(c.Request.Context(), middleware.UserID(c), input.Title)
	h.result(c, data, err)
}

func (h *Handler) UpdateNotebook(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	var input struct {
		Title       *string `json:"title"`
		Emoji       *string `json:"emoji"`
		Color       *int    `json:"color"`
		Description *string `json:"description"`
	}
	if !BindJSON(c, &input) {
		return
	}
	updates := make(map[string]any)
	if input.Title != nil {
		value := strings.TrimSpace(*input.Title)
		if !textValid(value, 1, 200) {
			invalid(c, "标题需要 1 到 200 个字符")
			return
		}
		updates["title"] = value
	}
	if input.Emoji != nil {
		value := strings.TrimSpace(*input.Emoji)
		if !textValid(value, 1, 32) {
			invalid(c, "图标需要 1 到 32 个字符")
			return
		}
		updates["emoji"] = value
	}
	if input.Color != nil {
		if *input.Color < 0 || *input.Color > 7 {
			invalid(c, "颜色必须在 0 到 7 之间")
			return
		}
		updates["color"] = *input.Color
	}
	if input.Description != nil {
		if !textValid(*input.Description, 0, 10000) {
			invalid(c, "描述不能超过 10000 个字符")
			return
		}
		updates["description"] = *input.Description
	}
	data, err := h.Service.UpdateNotebook(c.Request.Context(), middleware.UserID(c), id, updates)
	h.result(c, data, err)
}

func (h *Handler) DeleteNotebook(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	err := h.Service.DeleteNotebook(c.Request.Context(), middleware.UserID(c), id)
	h.result(c, nil, err)
}
