package handler

import (
	"github.com/gin-gonic/gin"
	"knownote/internal/middleware"
	"knownote/pkg/response"
	"strconv"
	"strings"
)

func registerAdmin(api *gin.RouterGroup, h *Handler) {
	admin := api.Group("/admin", middleware.ActiveUser(h.Service.DB, true))
	admin.GET("/settings", h.AdminSettings)
	admin.PATCH("/settings", h.SaveAdminSettings)
	admin.POST("/settings/test", h.TestAdminSettings)
	admin.GET("/users", h.AdminUsers)
	admin.PATCH("/users/:id", h.UpdateAdminUser)
	admin.GET("/usage", func(c *gin.Context) { data, err := h.Service.AdminUsage(c.Request.Context()); h.result(c, data, err) })
	admin.GET("/audit-logs", func(c *gin.Context) { data, err := h.Service.AuditLogs(c.Request.Context()); h.result(c, data, err) })
	admin.GET("/reindex", func(c *gin.Context) {
		if h.adminReady(c) {
			response.OK(c, h.Service.Admin.Progress())
		}
	})
	admin.POST("/reindex", func(c *gin.Context) {
		if h.adminReady(c) {
			err := h.Service.Admin.Start(c.Request.Context(), middleware.UserID(c))
			h.result(c, h.Service.Admin.Progress(), err)
		}
	})
}
func (h *Handler) adminReady(c *gin.Context) bool {
	if h.Service.Admin == nil {
		response.Fail(c, 503, 50001, "管理后台尚未初始化")
		return false
	}
	return true
}

type adminSettingsRequest struct {
	Values     map[string]string `json:"values"`
	Capability string            `json:"capability,omitempty"`
}

func (h *Handler) AdminSettings(c *gin.Context) {
	if !h.adminReady(c) {
		return
	}
	data, err := h.Service.Admin.Settings.Get(c.Request.Context())
	h.result(c, data, err)
}
func (h *Handler) SaveAdminSettings(c *gin.Context) {
	if !h.adminReady(c) {
		return
	}
	var input adminSettingsRequest
	if !BindJSON(c, &input) {
		return
	}
	if input.Capability != "" {
		invalid(c, "保存配置不接受测试能力参数")
		return
	}
	data, err := h.Service.Admin.Settings.Save(c.Request.Context(), middleware.UserID(c), input.Values)
	if err != nil {
		invalid(c, err.Error())
		return
	}
	response.OK(c, data)
}
func (h *Handler) TestAdminSettings(c *gin.Context) {
	if !h.adminReady(c) {
		return
	}
	var input adminSettingsRequest
	if !BindJSON(c, &input) {
		return
	}
	data, err := h.Service.Admin.Settings.Test(c.Request.Context(), input.Values, input.Capability)
	if err != nil {
		invalid(c, "测试参数无效，请检查配置项与格式")
		return
	}
	response.OK(c, data)
}
func (h *Handler) AdminUsers(c *gin.Context) {
	page, size := 1, 20
	var err error
	if v := c.Query("page"); v != "" {
		page, err = strconv.Atoi(v)
		if err != nil {
			invalid(c, "页码无效")
			return
		}
	}
	if v := c.Query("pageSize"); v != "" {
		size, err = strconv.Atoi(v)
		if err != nil {
			invalid(c, "每页数量无效")
			return
		}
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		invalid(c, "分页参数超出范围")
		return
	}
	query := strings.TrimSpace(c.Query("query"))
	if len([]rune(query)) > 254 {
		invalid(c, "搜索内容过长")
		return
	}
	data, err := h.Service.AdminUsers(c.Request.Context(), query, page, size)
	h.result(c, data, err)
}
func (h *Handler) UpdateAdminUser(c *gin.Context) {
	id, ok := ParamID(c)
	if !ok {
		return
	}
	var input struct {
		Disabled *bool   `json:"disabled"`
		Role     *string `json:"role"`
	}
	if !BindJSON(c, &input) {
		return
	}
	if input.Role != nil && *input.Role != "admin" && *input.Role != "user" {
		invalid(c, "角色只支持 user 或 admin")
		return
	}
	if input.Disabled == nil && input.Role == nil {
		invalid(c, "请选择要修改的账号状态或角色")
		return
	}
	err := h.Service.UpdateAdminUser(c.Request.Context(), middleware.UserID(c), id, input.Disabled, input.Role)
	h.result(c, nil, err)
}
