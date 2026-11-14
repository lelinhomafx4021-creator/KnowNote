package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"knownote/internal/service/chat"
)

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("连接已断开") }

func TestEncodeSSE(t *testing.T) {
	for _, event := range []chat.Event{
		{Type: "citations"},
		{Type: "delta", Text: "中文\n换行\r\n包含\"引号\" [2]"},
		{Type: "done", MessageID: "消息编号"},
		{Type: "error", Message: "模型暂时不可用"},
	} {
		var output bytes.Buffer
		if err := EncodeSSE(&output, event); err != nil {
			t.Fatal(err)
		}
		raw := output.String()
		if !strings.HasPrefix(raw, "data: ") || !strings.HasSuffix(raw, "\n\n") || strings.Count(raw, "\n") != 2 {
			t.Fatal("SSE 事件边界不符合契约")
		}
		var decoded chat.Event
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(raw, "data: "))), &decoded); err != nil || decoded.Type != event.Type || decoded.Text != event.Text || decoded.MessageID != event.MessageID || decoded.Message != event.Message {
			t.Fatal("SSE JSON 未正确保留字段")
		}
		if event.Type == "citations" && !strings.Contains(raw, `"citations":[]`) {
			t.Fatal("空引用没有返回空数组")
		}
	}
	if EncodeSSE(failedWriter{}, chat.Event{Type: "done"}) == nil {
		t.Fatal("未返回连接写入错误")
	}
}
