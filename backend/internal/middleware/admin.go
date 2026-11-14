package middleware

import (
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"knownote/internal/model"
	"knownote/pkg/response"
	"net/http"
)

// ActiveUser 每次查库，停用与角色撤销对现有 token 立即生效。
func ActiveUser(database *gorm.DB, admin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var u model.User
		err := database.WithContext(c.Request.Context()).Select("id", "role", "disabled").Where("id = ?", UserID(c)).First(&u).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 401, 40101, "登录凭证无效或已过期，请重新登录")
			return
		}
		if err != nil {
			response.Fail(c, 500, 50001, "读取账号状态失败，请稍后重试")
			return
		}
		if u.Disabled {
			response.Fail(c, http.StatusForbidden, 40301, "账号已被停用")
			return
		}
		if admin && u.Role != "admin" {
			response.Fail(c, http.StatusForbidden, 40301, "仅管理员可访问")
			return
		}
		c.Next()
	}
}
