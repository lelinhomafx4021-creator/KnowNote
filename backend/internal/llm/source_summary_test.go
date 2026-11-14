package llm

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode"
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
	// 不再限制摘要长度和主题数量：长短摘要都接受，主题去重并最多保留 6 个，没有主题时自动补齐。
	accept := map[string]func(*SourceSummary){
		"过短":   func(s *SourceSummary) { s.Summary = "短摘要" },
		"过长":   func(s *SourceSummary) { s.Summary = strings.Repeat("长", 400) },
		"主题不足": func(s *SourceSummary) { s.Topics = []string{"一", "二"} },
		"重复主题": func(s *SourceSummary) { s.Topics = []string{"一", "一", "一"} },
		"主题过多": func(s *SourceSummary) { s.Topics = []string{"一", "二", "三", "四", "五", "六", "七"} },
		"没有主题": func(s *SourceSummary) { s.Topics = nil },
	}
	for name, mutate := range accept {
		t.Run(name, func(t *testing.T) {
			input := valid
			mutate(&input)
			raw, _ := json.Marshal(input)
			result, err := ParseSourceSummary(string(raw))
			if err != nil {
				t.Fatalf("应接受模型给出的摘要：%v", err)
			}
			if len(result.Topics) == 0 || len(result.Topics) > 6 || len(result.Topics) != len(slices.Compact(slices.Clone(result.Topics))) {
				t.Fatalf("主题应去重、非空且不超过 6 个：%v", result.Topics)
			}
		})
	}
	for _, name := range []string{"非中文", "空摘要"} {
		t.Run(name, func(t *testing.T) {
			input := valid
			input.Summary = map[string]string{"非中文": strings.Repeat("a", 130), "空摘要": "  "}[name]
			raw, _ := json.Marshal(input)
			if _, err := ParseSourceSummary(string(raw)); err == nil {
				t.Fatal("应拒绝空摘要或非中文摘要")
			}
		})
	}
	for _, raw := range []string{"无效", `{"summary":1}`, `{} {}`, `null`} {
		if _, err := ParseSourceSummary(raw); err == nil {
			t.Fatal("应拒绝无效 JSON")
		}
	}
}

func TestFakeSourceTopics(t *testing.T) {
	text := strings.Repeat("知页融合大模型与向量检索，毕业设计采用 pgvector 管理个人资料。300 页、2026 年、1536 维。", 3)
	raw, err := NewFake(1536).Chat(context.Background(), SourceSummaryMessages(text), ChatOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ParseSourceSummary(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"pgvector", "大模型", "向量检索", "毕业设计"} {
		if !slices.Contains(result.Topics, expected) {
			t.Fatalf("主题缺少 %s：%v", expected, result.Topics)
		}
	}
	for _, topic := range result.Topics {
		if topic == "合大" || !strings.ContainsFunc(topic, unicode.IsLetter) {
			t.Fatalf("主题含数字或跨词碎片：%v", result.Topics)
		}
	}
	for _, input := range []string{"300 2026 1536", "", "300个", "GPT4 GPT4 数据库 数据库", "量子力学；天体物理", "is in to the documentation"} {
		first, second := sourceKeywords(input), sourceKeywords(input)
		if len(first) < 3 || len(first) > 6 || !slices.Equal(first, second) {
			t.Fatalf("主题数量或确定性不正确：%v", first)
		}
		for i, topic := range first {
			if !strings.ContainsFunc(topic, unicode.IsLetter) || slices.Contains(first[:i], topic) || slices.Contains([]string{"is", "in", "to", "the"}, topic) {
				t.Fatalf("出现纯数字或重复主题：%v", first)
			}
		}
	}
}

func TestSummaryPromptLimit(t *testing.T) {
	messages := SourceSummaryMessages(strings.Repeat("字", 12001))
	if len([]rune(messages[1].Content)) != 12000 {
		t.Fatal("摘要输入未按字符截断")
	}
}
