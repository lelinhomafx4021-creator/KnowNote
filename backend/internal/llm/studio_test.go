package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildStudioContext(t *testing.T) {
	var sources []StudioSource
	for i := 0; i < 20; i++ {
		sources = append(sources, StudioSource{Title: strings.Repeat("标题", 80), Summary: strings.Repeat("摘要汉字🙂", 500), Topics: []string{"检索", "向量", "分块"}, Chunks: []string{strings.Repeat("原文\"\\\n", 600), strings.Repeat("代表内容", 600), strings.Repeat("后半部分", 600)}})
	}
	input, err := BuildStudioContext(sources, StudioContextLimit)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(input) || utf8.RuneCountInString(input) > StudioContextLimit {
		t.Fatal("上下文超限或损坏编码")
	}
	var items []StudioSource
	if err := json.Unmarshal([]byte(input), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != len(sources) {
		t.Fatal("遗漏来源")
	}
	for _, item := range items {
		if item.Summary == "" || len(item.Chunks) != 3 || len(item.Topics) != 3 || item.Chunks[0] == "" {
			t.Fatal("预算未覆盖摘要、主题与代表片段")
		}
	}
	if sources[0].Summary != strings.Repeat("摘要汉字🙂", 500) {
		t.Fatal("截断修改了原始数据")
	}
	if _, err := BuildStudioContext(sources, 5); err == nil {
		t.Fatal("无法容纳来源时应明确报错")
	}
	if empty, err := BuildStudioContext(nil, StudioContextLimit); err != nil || empty != "[]" {
		t.Fatal("空输入错误")
	}
}

func TestFakeStudio(t *testing.T) {
	input, err := BuildStudioContext([]StudioSource{{Title: "检索资料", Summary: "2020年，资料讨论了检索增强生成的方法与验证条件。", Topics: []string{"检索增强", "独立验证"}, Chunks: []string{"检索片段帮助模型依据资料回答。"}}}, StudioContextLimit)
	if err != nil {
		t.Fatal(err)
	}
	fake := NewFake(8)
	for kind := range StudioTitles {
		output, err := fake.Chat(context.Background(), StudioMessages(kind, "检索笔记", input), ChatOptions{})
		if err != nil {
			t.Fatal(err)
		}
		switch kind {
		case "study-guide":
			for _, part := range []string{"## 学习目标", "## 简答题", "参考答案要点", "## 论述题", "## 术语表"} {
				if !strings.Contains(output, part) {
					t.Fatal("学习指南缺少结构", part)
				}
			}
		case "briefing":
			for _, part := range []string{"## 执行摘要", "## 核心主题", "### 检索资料", "## 关键结论与建议"} {
				if !strings.Contains(output, part) {
					t.Fatal("简报缺少结构", part)
				}
			}
		case "faq":
			if n := strings.Count(output, "### "); n < 6 || n > 10 {
				t.Fatal("问答数量错误")
			}
		case "timeline":
			if !strings.Contains(output, "2020年") || !strings.Contains(output, "## 人物表") {
				t.Fatal("时间线没有提取原文日期")
			}
		case "mindmap":
			if !json.Valid([]byte(output)) || !strings.Contains(output, "独立验证") {
				t.Fatal("思维导图未使用来源topics")
			}
		}
	}
	output, err := fake.Chat(context.Background(), AudioMessages("笔记", input), ChatOptions{})
	if err != nil {
		t.Fatal(err)
	}
	count := utf8.RuneCountInString(output) - strings.Count(output, "小知：")*3 - strings.Count(output, "小页：")*3 - strings.Count(output, "\n")
	if count < 1200 || count > 1800 || !strings.Contains(output, "小知：") || !strings.Contains(output, "小页：") {
		t.Fatalf("对谈结构或字数错误：%d", count)
	}
	for _, source := range []StudioSource{{Title: "短", Summary: "短"}, {Title: strings.Repeat("长标题", 40), Summary: strings.Repeat("长摘要", 200), Topics: []string{strings.Repeat("长主题", 30)}}} {
		script := fakeAudio([]StudioSource{source})
		length := utf8.RuneCountInString(script) - strings.Count(script, "小知：")*3 - strings.Count(script, "小页：")*3 - strings.Count(script, "\n")
		if length < 1200 || length > 1800 {
			t.Fatalf("极短或极长来源导致脚本超出字数：%d", length)
		}
	}
	withoutDates := []StudioSource{{Title: "概念资料", Summary: "本文讨论概念，但没有时间信息。"}}
	timeline, err := fakeStudio("timeline", withoutDates)
	if err != nil || !strings.Contains(timeline, "资料没有明确时间信息") {
		t.Fatal("没有时间时未说明")
	}
}
