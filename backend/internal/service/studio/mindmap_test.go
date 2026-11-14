package studio

import (
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
		`null`, `[]`, `{"label":""}`, `{"label":null}`, `{"label":20}`, `{"label":"根","children":{}}`, `{"label":"根"}{"label":"其他根"}`,
	} {
		if _, err := ParseMindmap(raw); err == nil {
			t.Fatalf("应拒绝无效结构：%s", raw)
		}
	}
	// 不再限制层数、分支数与标签长度：空子节点跳过，超长标签截短。
	for raw, check := range map[string]func(*model.MindMapNode) bool{
		`{"label":"根","children":[null,{"label":""},{"label":"子"}]}`: func(n *model.MindMapNode) bool { return len(n.Children) == 1 },
		`{"label":"根","children":[{"label":"二","children":[{"label":"三","children":[{"label":"四","children":[{"label":"五"}]}]}]}]}`: func(n *model.MindMapNode) bool {
			return n.Children[0].Children[0].Children[0].Children[0].Label == "五"
		},
		`{"label":"根","children":[` + strings.Repeat(`{"label":"子"},`, 11) + `{"label":"子"}]}`: func(n *model.MindMapNode) bool { return len(n.Children) == 12 },
		`{"label":"` + strings.Repeat("中", 45) + `"}`:                                          func(n *model.MindMapNode) bool { return len([]rune(n.Label)) == 41 },
	} {
		node, err := ParseMindmap(raw)
		if err != nil || !check(node) {
			t.Fatalf("应宽松接受：%s（%v）", raw, err)
		}
	}
}

func TestParseTranscript(t *testing.T) {
	valid := "小知：" + strings.Repeat("知", 650) + "\n\n小页: " + strings.Repeat("页", 650)
	turns, err := parseTranscript(valid)
	if err != nil || len(turns) != 2 || turns[1].Speaker != "小页" {
		t.Fatal("双主持脚本解析失败", err)
	}
	// 不限制篇幅和主持人数量；没有前缀的开头归给小知，空发言跳过。
	for raw, want := range map[string]int{"小知：内容": 1, "小知：\n小页：内容": 1, "前言\n" + valid: 3, "小知：" + strings.Repeat("知", 1900) + "\n小页：内容": 2} {
		turns, err := parseTranscript(raw)
		if err != nil || len(turns) != want {
			t.Fatalf("脚本应被接受：%q 得到 %d 段（%v）", raw[:min(len(raw), 20)], len(turns), err)
		}
	}
	if _, err := parseTranscript("  \n "); err == nil {
		t.Fatal("空脚本应拒绝")
	}
}
