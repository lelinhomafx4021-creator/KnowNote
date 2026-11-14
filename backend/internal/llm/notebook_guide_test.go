package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNotebookGuide(t *testing.T) {
	for _, sources := range [][]GuideSource{
		{{Title: "短资料", Summary: "机器学习"}},
		{{Title: "机器学习", Summary: strings.Repeat("模型需要训练和验证。", 30)}, {Title: "模型评估", Summary: "评估应使用独立数据集。"}},
	} {
		raw, err := NewFake(32).Chat(context.Background(), GuideMessages(sources), ChatOptions{})
		if err != nil {
			t.Fatal(err)
		}
		guide, err := ParseGuide(raw)
		if err != nil || !strings.Contains(guide.Summary, "机器学习") && sources[0].Title == "短资料" {
			t.Fatalf("Fake 概要无效：%v，%s", err, raw)
		}
		if _, err := ParseGuide("```json\n" + raw + "\n```"); err != nil {
			t.Fatal(err)
		}
	}
	valid, _ := json.Marshal(FallbackGuide([]GuideSource{{Title: "资料", Summary: "内容"}}))
	for _, raw := range []string{"错误 JSON", `{}`, string(valid) + `{}`, `{"summary":"过短","suggestedQuestions":["一","二","三"]}`, strings.Replace(string(valid), "这些资料提供了哪些事实或例子来支持主要观点？", "如何梳理资料中关键概念的联系与适用条件？", 1)} {
		if _, err := ParseGuide(raw); err == nil {
			t.Fatal("错误概要应拒绝")
		}
	}
	if guide := FallbackGuide(nil); guide.Summary != "" || guide.SuggestedQuestions == nil || len(guide.SuggestedQuestions) != 0 {
		t.Fatal("空资料概要不正确")
	}
}

func TestFakeUsesCurrentContext(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "只依据当前资料"},
		{Role: "system", Content: "KNOWNOTE_RAG_CONTEXT\n[2] 来源：本轮资料\n本轮正文"},
		{Role: "assistant", Content: "历史引用 [99] 历史正文"},
		{Role: "user", Content: "解释一下 [88]"},
	}
	answer, err := NewFake(32).Chat(context.Background(), messages, ChatOptions{})
	// 问题本身可能包含编号，检查仅有本轮编号的资料要点。
	if err != nil || !strings.Contains(answer, "本轮正文") || strings.Contains(answer, "历史正文") || strings.Contains(answer, "[99]") {
		t.Fatal("Fake 误把历史当成本轮引用")
	}
}
