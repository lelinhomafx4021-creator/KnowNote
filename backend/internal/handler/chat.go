package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/service/chat"
)

// EncodeSSE 保证 JSON 内换行被转义，每条 data 后有空行；空引用仍编码为 []。
func EncodeSSE(writer io.Writer, event chat.Event) error {
	var value any = event
	if event.Type == "citations" {
		citations := event.Citations
		if citations == nil {
			citations = []model.Citation{}
		}
		value = struct {
			Type      string           `json:"type"`
			Citations []model.Citation `json:"citations"`
		}{event.Type, citations}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "data: %s\n\n", data)
	return err
}

type messageView struct {
	ID        string           `json:"id"`
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Citations []model.Citation `json:"citations,omitempty"`
	Feedback  *string          `json:"feedback,omitempty"`
	CreatedAt time.Time        `json:"createdAt"`
}

func ChatRoutes(core *service.Service, chats *chat.Service) Registrar {
	h := &Handler{Service: core, Logger: core.Logger}
	return func(api *gin.RouterGroup) {
		api.GET("/notebooks/:id/messages", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			messages, err := chats.Messages(c.Request.Context(), middleware.UserID(c), id)
			views := make([]messageView, 0, len(messages))
			for _, message := range messages {
				views = append(views, messageView{message.ID, message.Role, message.Content, message.Citations, message.Feedback, message.CreatedAt})
			}
			h.result(c, views, err)
		})
		api.DELETE("/notebooks/:id/messages", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			h.result(c, nil, chats.Clear(c.Request.Context(), middleware.UserID(c), id))
		})
		api.POST("/messages/:id/feedback", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			var input struct {
				Value json.RawMessage `json:"value"`
			}
			if !BindJSON(c, &input) {
				return
			}
			var value *string
			if len(input.Value) == 0 || json.Unmarshal(input.Value, &value) != nil || (value != nil && *value != "up" && *value != "down") {
				invalid(c, "反馈必须为 up、down 或 null")
				return
			}
			h.result(c, nil, chats.Feedback(c.Request.Context(), middleware.UserID(c), id, value))
		})
		api.GET("/notebooks/:id/guide", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			guide, err := chats.Guide(c.Request.Context(), middleware.UserID(c), id)
			h.result(c, guide, err)
		})
		api.POST("/notebooks/:id/chat", func(c *gin.Context) {
			id, ok := ParamID(c)
			if !ok {
				return
			}
			var input struct {
				Question  string    `json:"question"`
				SourceIDs *[]string `json:"sourceIds"`
			}
			if !BindJSON(c, &input) {
				return
			}
			input.Question = strings.TrimSpace(input.Question)
			if !textValid(input.Question, 1, 10000) || input.SourceIDs == nil || len(*input.SourceIDs) > 200 {
				invalid(c, "问题须为 1～10000 字，sourceIds 须为至多 200 项的数组")
				return
			}
			for i, value := range *input.SourceIDs {
				parsed, err := uuid.Parse(value)
				if err != nil || parsed == uuid.Nil {
					invalid(c, "来源 ID 必须是有效的 UUID")
					return
				}
				(*input.SourceIDs)[i] = parsed.String()
			}
			request, err := chats.Prepare(c.Request.Context(), middleware.UserID(c), id, input.Question, *input.SourceIDs)
			if err != nil {
				h.result(c, nil, err)
				return
			}
			c.Header("Content-Type", "text/event-stream; charset=utf-8")
			c.Header("Cache-Control", "no-cache")
			c.Header("X-Accel-Buffering", "no")
			chats.Stream(c.Request.Context(), request, func(event chat.Event) error {
				if err := c.Request.Context().Err(); err != nil {
					return err
				}
				if err := EncodeSSE(c.Writer, event); err != nil {
					return err
				}
				c.Writer.Flush()
				return nil
			})
		})
	}
}
