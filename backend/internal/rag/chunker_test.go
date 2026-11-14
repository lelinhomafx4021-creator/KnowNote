package rag

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChunkCoverageAndOverlap(t *testing.T) {
	cases := map[string]string{
		"空文本": "", "空白": " \n\t", "单字": "知", "恰好五百字": strings.Repeat("知", 500),
		"短尾段合并": strings.Repeat("知", 550), "超长句": strings.Repeat("知识", 1500),
		"中文句子":  strings.Repeat("这是需要保持完整的中文句子。", 100),
		"短段落合并": strings.Repeat("短段落。\n\n", 200),
		"多字节字符": strings.Repeat("😀学习🧠", 350),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			chunks := Chunk(input)
			input = strings.TrimSpace(input)
			if input == "" {
				if len(chunks) != 0 {
					t.Fatal("空文本不能生成块")
				}
				return
			}
			if len(chunks) == 0 {
				t.Fatal("正文没有生成分块")
			}
			rebuilt := chunks[0]
			for i, chunk := range chunks {
				runes := []rune(chunk)
				if !utf8.ValidString(chunk) || len(runes) > 599 || len(runes) == 0 {
					t.Fatalf("无效分块：%d 字", len(runes))
				}
				if i == 0 {
					continue
				}
				previous := []rune(chunks[i-1])
				if string(previous[len(previous)-ChunkOverlap:]) != string(runes[:ChunkOverlap]) {
					t.Fatal("相邻块没有精确重叠 80 字")
				}
				rebuilt += string(runes[ChunkOverlap:])
			}
			if rebuilt != input {
				t.Fatal("重建正文出现丢失或重复")
			}
			if len([]rune(input)) <= 599 && len(chunks) != 1 {
				t.Fatal("短尾段应该合并")
			}
		})
	}
}

func TestChunkBoundaries(t *testing.T) {
	for _, boundary := range []string{"\n\n", "。", "！", "?"} {
		t.Run(fmt.Sprintf("边界%q", boundary), func(t *testing.T) {
			prefix := strings.Repeat("甲", 350) + boundary
			chunks := Chunk(prefix + strings.Repeat("乙", 700))
			if chunks[0] != prefix {
				t.Fatal("没有优先选择段落或句末边界")
			}
		})
	}
	if got := WordCount("中文 abc\n😀\t"); got != 6 {
		t.Fatalf("字符数为 %d", got)
	}
}
