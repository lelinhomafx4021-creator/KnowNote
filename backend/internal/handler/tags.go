package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"knownote/internal/middleware"
	"regexp"
	"strings"
)

var tagColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (h *Handler) Tags(c *gin.Context) {
	data, err := h.Service.Tags(c.Request.Context(), middleware.UserID(c))
	h.result(c, data, err)
}
func (h *Handler) SaveTag(c *gin.Context) {
	var input struct {
		Name  *string `json:"name"`
		Color *string `json:"color"`
	}
	if !BindJSON(c, &input) {
		return
	}
	updates := map[string]any{}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if !textValid(value, 1, 50) {
			invalid(c, "标签名称需要 1 到 50 个字符")
			return
		}
		updates["name"] = value
	}
	if input.Color != nil {
		if !tagColor.MatchString(*input.Color) {
			invalid(c, "标签颜色需要使用 #RRGGBB 格式")
			return
		}
		updates["color"] = strings.ToUpper(*input.Color)
	}
	if c.Request.Method == "POST" {
		if input.Name == nil {
			invalid(c, "请输入标签名称")
			return
		}
		color := "#6750A4"
		if v, ok := updates["color"].(string); ok {
			color = v
		}
		data, err := h.Service.CreateTag(c.Request.Context(), middleware.UserID(c), updates["name"].(string), color)
		h.result(c, data, err)
	} else {
		id, ok := ParamID(c)
		if !ok {
			return
		}
		data, err := h.Service.UpdateTag(c.Request.Context(), middleware.UserID(c), id, updates)
		h.result(c, data, err)
	}
}
func (h *Handler) DeleteTag(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	h.result(c, nil, h.Service.DeleteTag(c.Request.Context(), middleware.UserID(c), id))
}
func (h *Handler) SetNotebookTags(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	var input struct {
		TagIDs []string `json:"tagIds"`
	}
	if !BindJSON(c, &input) {
		return
	}
	if input.TagIDs == nil || len(input.TagIDs) > 100 {
		invalid(c, "tagIds 必须是数组，且最多选择 100 个标签")
		return
	}
	for i, value := range input.TagIDs {
		parsed, err := uuid.Parse(value)
		if err != nil || parsed == uuid.Nil {
			invalid(c, "标签 ID 必须是有效的 UUID")
			return
		}
		input.TagIDs[i] = parsed.String()
	}
	data, err := h.Service.SetNotebookTags(c.Request.Context(), middleware.UserID(c), id, input.TagIDs)
	h.result(c, data, err)
}
