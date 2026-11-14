package rag

import (
	"fmt"
	"strings"
	"testing"

	"knownote/internal/model"
)

func TestBuildPrompt(t *testing.T) {
	var history []model.Message
	for i := 0; i < 8; i++ {
		history = append(history, model.Message{Role: "user", Content: fmt.Sprintf("历史问题%d", i)}, model.Message{Role: "assistant", Content: fmt.Sprintf("历史回答%d [99]", i)})
	}
	for style, word := range map[string]string{"concise": "简洁", "detailed": "详细", "learning": "学习引导"} {
		messages := BuildPrompt("当前问题", style, history, []Passage{{Citation: model.Citation{Index: 2, SourceTitle: "测试来源"}, Content: "可核对的原文"}})
		if len(messages) != 15 || messages[2].Content != "历史问题2" || messages[len(messages)-1].Content != "当前问题" {
			t.Fatal("没有按六轮保留历史并添加当前问题")
		}
		for _, rule := range []string{"只依据提供的资料回答", "每个依据资料的句子末尾用 [n] 标注", "资料不足时明确说明", "中文 Markdown", "不要编造引用编号", word} {
			if !strings.Contains(messages[0].Content, rule) {
				t.Fatalf("提示词缺少要求：%s", rule)
			}
		}
		if !strings.Contains(messages[1].Content, "[2] 来源：测试来源；位置：\n可核对的原文") || strings.Contains(messages[1].Content, "[99]") {
			t.Fatal("检索资料与历史没有隔离")
		}
	}
	if !strings.Contains(BuildPrompt("问题", "", nil, nil)[1].Content, "没有检索到足够相关的资料") {
		t.Fatal("无匹配资料时缺少提示")
	}
}
