package studio

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"knownote/internal/model"
)

func TestParseMindmap(t *testing.T) {
	for _, raw := range []string{
		`{"label":"根","children":[{"label":"中文分支"}]}`,
		"\ufeff```json\n{\"label\":\"根\",\"children\":[{\"label\":\"中文分支\",},],}\n```",
		"以下是结果：\n{\"label\":\"根\",\"children\":[{\"label\":\"字符串中的,}与\\\"引号\"}]}\n说明结束。",
	} {
		node, err := ParseMindmap(raw)
		if err != nil {
			t.Fatal(err)
		}
		if node.Label != "根" || len(node.Children) != 1 || node.ID == node.Children[0].ID {
			t.Fatal("节点内容或编号错误")
		}
		if _, err := uuid.Parse(node.ID); err != nil {
			t.Fatal("未补充UUID")
		}
	}
	for _, raw := range []string{
		`null`, `[]`, `{"label":""}`, `{"label":null}`, `{"label":20}`, `{"label":"根","children":[null]}`, `{"label":"根","children":{}}`,
		`{"label":"根"}{"label":"其他根"}`, `{"label":"根","children":[{"label":"二","children":[{"label":"三","children":[{"label":"四","children":[{"label":"五"}]}]}]}]}`,
		`{"label":"` + strings.Repeat("中", 21) + `"}`, `{"label":"根","children":[` + strings.Repeat(`{"label":"子"},`, 8) + `{"label":"子"}]}`,
	} {
		if _, err := ParseMindmap(raw); err == nil {
			t.Fatalf("应拒绝无效结构：%s", raw)
		}
	}
	root := model.MindMapNode{Label: strings.Repeat("汉", 20)}
	for i := 0; i < 8; i++ {
		root.Children = append(root.Children, model.MindMapNode{Label: "二", Children: []model.MindMapNode{{Label: "三", Children: []model.MindMapNode{{Label: "四"}}}}})
	}
	data, _ := json.Marshal(root)
	if _, err := ParseMindmap(string(data)); err != nil {
		t.Fatal("边界合法结构被拒绝", err)
	}
}

func TestParseTranscript(t *testing.T) {
	valid := "小知：" + strings.Repeat("知", 650) + "\n\n小页: " + strings.Repeat("页", 650)
	turns, err := parseTranscript(valid)
	if err != nil || len(turns) != 2 || turns[1].Speaker != "小页" {
		t.Fatal("双主持脚本解析失败", err)
	}
	for _, raw := range []string{"小知：内容", "小知：\n小页：内容", "前言\n" + valid, "小知：" + strings.Repeat("知", 1900) + "\n小页：内容"} {
		if _, err := parseTranscript(raw); err == nil {
			t.Fatal("无效脚本未拒绝")
		}
	}
}
