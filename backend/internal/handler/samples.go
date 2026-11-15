package handler

import (
	"github.com/gin-gonic/gin"
	"knownote/internal/middleware"
	"knownote/internal/service"
	"knownote/internal/service/samples"
)

func SampleRoutes(base *service.Service, manager *samples.Manager) Registrar {
	h := &Handler{Service: base, Logger: base.Logger}
	return func(api *gin.RouterGroup) {
		api.GET("/samples", func(c *gin.Context) {
			data, err := manager.List(c.Request.Context(), middleware.UserID(c))
			h.result(c, data, err)
		})
		api.POST("/samples/:slug/use", func(c *gin.Context) {
			data, err := manager.Use(c.Request.Context(), middleware.UserID(c), c.Param("slug"))
			h.result(c, data, err)
		})
	}
}
