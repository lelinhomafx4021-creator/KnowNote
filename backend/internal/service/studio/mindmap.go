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
	// 尽量接受模型给出的结构：标签过长截短、空节点跳过；只保留 10 层的安全上限，防止异常数据递归过深。
	var parse func(string, int) (model.MindMapNode, bool, error)
	parse = func(data string, depth int) (model.MindMapNode, bool, error) {
		if !strings.HasPrefix(strings.TrimSpace(data), "{") {
			return model.MindMapNode{}, false, nil
		}
		current := tree
		decoder := json.NewDecoder(strings.NewReader(data))
		if err := decoder.Decode(&current); err != nil || decoder.Decode(new(any)) != io.EOF {
			return model.MindMapNode{}, false, fmt.Errorf("思维导图 JSON 格式无效")
		}
		label := strings.TrimSpace(strings.ReplaceAll(current.Label, "\x00", ""))
		if label == "" {
			return model.MindMapNode{}, false, nil
		}
		if runes := []rune(label); len(runes) > 40 {
			label = string(runes[:40]) + "…"
		}
		node := model.MindMapNode{ID: uuid.NewString(), Label: label}
		if depth >= 10 {
			return node, true, nil
		}
		for _, child := range current.Children {
			parsed, ok, err := parse(string(child), depth+1)
			if err != nil {
				return model.MindMapNode{}, false, err
			}
			if ok {
				node.Children = append(node.Children, parsed)
			}
		}
		return node, true, nil
	}
	root, ok, err := parse(cleaned.String(), 1)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("思维导图缺少根节点")
	}
	return &root, nil
}
