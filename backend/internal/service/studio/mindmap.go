package studio

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"knownote/internal/model"
)

// ParseMindmap 去除 BOM、代码围栏和对象外说明，并修复字符串之外的尾逗号。
func ParseMindmap(raw string) (*model.MindMapNode, error) {
	if len(raw) > 256<<10 || !utf8.ValidString(raw) {
		return nil, fmt.Errorf("思维导图内容无效或过大")
	}
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
	start, end := strings.IndexByte(raw, '{'), strings.LastIndexByte(raw, '}')
	if start < 0 || end < start {
		return nil, fmt.Errorf("思维导图不是 JSON 对象")
	}
	raw = raw[start : end+1]
	var cleaned strings.Builder
	quoted, escaped := false, false
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if quoted {
			cleaned.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				quoted = false
			}
			continue
		}
		if ch == '"' {
			quoted = true
		}
		if ch == ',' {
			j := i + 1
			for j < len(raw) && strings.ContainsRune(" \r\n\t", rune(raw[j])) {
				j++
			}
			if j < len(raw) && (raw[j] == ']' || raw[j] == '}') {
				continue
			}
		}
		cleaned.WriteByte(ch)
	}
	var tree struct {
		Label    string            `json:"label"`
		Children []json.RawMessage `json:"children"`
	}
	// 每层递归解析前检查深度，拒绝类型错误和非对象节点。
	var parse func(string, int) (model.MindMapNode, error)
	parse = func(data string, depth int) (model.MindMapNode, error) {
		if depth > 4 || !strings.HasPrefix(strings.TrimSpace(data), "{") {
			return model.MindMapNode{}, fmt.Errorf("思维导图深度超过4层或节点无效")
		}
		current := tree
		decoder := json.NewDecoder(strings.NewReader(data))
		if err := decoder.Decode(&current); err != nil || decoder.Decode(new(any)) != io.EOF {
			return model.MindMapNode{}, fmt.Errorf("思维导图 JSON 格式无效")
		}
		label := strings.TrimSpace(current.Label)
		if utf8.RuneCountInString(label) < 1 || utf8.RuneCountInString(label) > 20 || strings.ContainsRune(label, '\x00') || len(current.Children) > 8 {
			return model.MindMapNode{}, fmt.Errorf("思维导图标签或分支数量无效")
		}
		node := model.MindMapNode{ID: uuid.NewString(), Label: label}
		for _, child := range current.Children {
			parsed, err := parse(string(child), depth+1)
			if err != nil {
				return model.MindMapNode{}, err
			}
			node.Children = append(node.Children, parsed)
		}
		return node, nil
	}
	root, err := parse(cleaned.String(), 1)
	if err != nil {
		return nil, err
	}
	return &root, nil
}
