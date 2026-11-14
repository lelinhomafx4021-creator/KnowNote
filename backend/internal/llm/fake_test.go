package llm

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"knownote/internal/config"
)

func TestFakeEmbeddings(t *testing.T) {
	fake := NewFake(1536)
	texts := []string{"机器学习 神经网络 训练", "机器学习 神经网络 推理", "苹果 香蕉 水果", ""}
	for len(texts) < 130 {
		texts = append(texts, "机器学习 神经网络 训练")
	}
	first, err := fake.Embed(context.Background(), texts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fake.Embed(context.Background(), texts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first[0], first[129]) || len(first) != len(texts) {
		t.Fatal("向量应确定且跨批次一致")
	}
	for _, vector := range first {
		if len(vector) != 1536 {
			t.Fatal("向量维度错误")
		}
		var norm float64
		for _, value := range vector {
			norm += float64(value) * float64(value)
		}
		if math.Abs(norm-1) > 1e-5 {
			t.Fatal("向量没有归一化")
		}
	}
	if dot(first[0], first[1]) <= dot(first[0], first[2]) {
		t.Fatal("共享词语的文本应更相似")
	}
	custom, err := NewFake(24).Embed(context.Background(), []string{"维度可配置"})
	if err != nil || len(custom[0]) != 24 {
		t.Fatal("自定义向量维度错误")
	}
}

func dot(a, b []float32) float64 {
	var result float64
	for i := range a {
		result += float64(a[i]) * float64(b[i])
	}
	return result
}

func TestFakeChatAndStream(t *testing.T) {
	fake := NewFake(32)
	messages := []Message{{Role: "system", Content: "资料：[1] 机器学习通过数据训练模型。\n[2] 模型需要在新数据上验证。"}, {Role: "user", Content: "怎样训练和验证模型？"}}
	answer, err := fake.Chat(context.Background(), messages, ChatOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"[1]", "[2]", "怎样训练和验证模型", "机器学习"} {
		if !strings.Contains(answer, value) {
			t.Fatalf("回答缺少 %s", value)
		}
	}
	var deltas strings.Builder
	count := 0
	streamed, err := fake.ChatStream(context.Background(), messages, ChatOptions{}, func(text string) error {
		if !utf8.ValidString(text) || utf8.RuneCountInString(text) != 1 {
			t.Fatal("Fake 流应逐字输出完整字符")
		}
		count++
		deltas.WriteString(text)
		return nil
	})
	if err != nil || streamed != answer || deltas.String() != answer || count != utf8.RuneCountInString(answer) {
		t.Fatal("流式回答与普通回答不一致")
	}
	callbackError := errors.New("客户端断开")
	partial, err := fake.ChatStream(context.Background(), messages, ChatOptions{}, func(string) error { return callbackError })
	if !errors.Is(err, callbackError) || utf8.RuneCountInString(partial) != 1 {
		t.Fatal("应返回中断时已生成的内容")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fake.Embed(ctx, []string{"取消"}); !errors.Is(err, context.Canceled) {
		t.Fatal("向量化应响应取消")
	}
	if _, err := fake.ChatStream(ctx, messages, ChatOptions{}, func(string) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal("流式回答应响应取消")
	}
	if _, err := fake.TTS(context.Background(), "演示", "alloy"); !errors.Is(err, ErrNotConfigured) {
		t.Fatal("Fake 不应伪造音频")
	}
	if client := New(config.Config{EmbeddingDim: 32}); client.Name() != "fake" {
		t.Fatal("没有密钥时应启用 Fake")
	}
}
