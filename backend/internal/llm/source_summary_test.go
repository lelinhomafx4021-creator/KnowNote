package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSourceSummary(t *testing.T) {
	for _, text := range []string{"短", strings.Repeat("知识管理和资料检索。", 100), "PDF document semantic search learning notes"} {
		raw, err := NewFake(1536).Chat(context.Background(), SourceSummaryMessages(text), ChatOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, candidate := range []string{raw, "```json\n" + raw + "\n```"} {
			result, err := ParseSourceSummary(candidate)
			if err != nil || len(result.Topics) < 3 || len(result.Topics) > 6 {
				t.Fatalf("摘要降级或 Fake 输出无效：%v", err)
			}
		}
	}
	valid := FallbackSourceSummary("知识检索与文本处理")
	for _, name := range []string{"过短", "过长", "非中文", "主题不足", "重复主题", "主题过多"} {
		t.Run(name, func(t *testing.T) {
			input := valid
			switch name {
			case "过短":
				input.Summary = "短摘要"
			case "过长":
				input.Summary = strings.Repeat("长", 201)
			case "非中文":
				input.Summary = strings.Repeat("a", 130)
			case "主题不足":
				input.Topics = []string{"一", "二"}
			case "重复主题":
				input.Topics = []string{"一", "一", "一"}
			case "主题过多":
				input.Topics = []string{"一", "二", "三", "四", "五", "六", "七"}
			}
			raw, _ := json.Marshal(input)
			if _, err := ParseSourceSummary(string(raw)); err == nil {
				t.Fatal("应拒绝不合规的结构化结果")
			}
		})
	}
	for _, raw := range []string{"无效", `{"summary":1}`, `{} {}`, `null`} {
		if _, err := ParseSourceSummary(raw); err == nil {
			t.Fatal("应拒绝无效 JSON")
		}
	}
}

func TestSummaryPromptLimit(t *testing.T) {
	messages := SourceSummaryMessages(strings.Repeat("字", 12001))
	if len([]rune(messages[1].Content)) != 12000 {
		t.Fatal("摘要输入未按字符截断")
	}
}
