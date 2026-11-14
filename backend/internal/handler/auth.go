package handler

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"knownote/internal/middleware"
)

type registerRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required"`
	Nickname string `json:"nickname" binding:"required"`
}

func (h *Handler) Register(c *gin.Context) {
	var input registerRequest
	if !BindJSON(c, &input) {
		return
	}
	input.Nickname = strings.TrimSpace(input.Nickname)
	if !textValid(input.Password, 6, 72) || len(input.Password) > 72 {
		invalid(c, "密码至少 6 个字符，且不超过 72 字节")
		return
	}
	if !textValid(input.Nickname, 1, 100) {
		invalid(c, "昵称需要 1 到 100 个字符")
		return
	}
	data, err := h.Service.Register(c.Request.Context(), input.Email, input.Password, input.Nickname)
	h.result(c, data, err)
}

func (h *Handler) Login(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email,max=254"`
		Password string `json:"password" binding:"required"`
	}
	if !BindJSON(c, &input) {
		return
	}
	if len(input.Password) > 72 {
		invalid(c, "密码不能超过 72 字节")
		return
	}
	data, err := h.Service.Login(c.Request.Context(), input.Email, input.Password)
	h.result(c, data, err)
}

func (h *Handler) Me(c *gin.Context) {
	data, err := h.Service.Me(c.Request.Context(), middleware.UserID(c))
	h.result(c, data, err)
}

func (h *Handler) Settings(c *gin.Context) {
	data, err := h.Service.Settings(c.Request.Context(), middleware.UserID(c))
	h.result(c, data, err)
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var input struct {
		Model        *string         `json:"model"`
		AnswerStyle  *string         `json:"answerStyle"`
		Language     *string         `json:"language"`
		StorageUsed  json.RawMessage `json:"storageUsed"`
		StorageTotal json.RawMessage `json:"storageTotal"`
	}
	if !BindJSON(c, &input) {
		return
	}
	updates := make(map[string]any)
	if input.Model != nil {
		value := strings.TrimSpace(*input.Model)
		if !textValid(value, 1, 200) {
			invalid(c, "模型名需要 1 到 200 个字符")
			return
		}
		updates["model"] = value
	}
	if input.AnswerStyle != nil {
		value := *input.AnswerStyle
		if value != "concise" && value != "detailed" && value != "learning" {
			invalid(c, "回答风格无效")
			return
		}
		updates["answer_style"] = value
	}
	if input.Language != nil {
		value := *input.Language
		if value != "zh-CN" && value != "en-US" {
			invalid(c, "语言只支持 zh-CN 或 en-US")
			return
		}
		updates["language"] = value
	}
	data, err := h.Service.UpdateSettings(c.Request.Context(), middleware.UserID(c), updates)
	h.result(c, data, err)
}
