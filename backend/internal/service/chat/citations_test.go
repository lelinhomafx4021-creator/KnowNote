package chat

import (
	"strings"
	"testing"

	"knownote/internal/model"
	"knownote/internal/rag"
)

func TestFilterCitations(t *testing.T) {
	citations := []model.Citation{{Index: 1}, {Index: 2}, {Index: 3}, {Index: 12}}
	filtered := FilterCitations("只引用 [3][12]，重复 [3]，不存在 [99]，不完整 [1，以及 [02]", citations)
	if len(filtered) != 2 || filtered[0].Index != 3 || filtered[1].Index != 12 {
		t.Fatalf("引用过滤或原编号保留失败：%v", filtered)
	}
	if got := FilterCitations("无引用", citations); got == nil || len(got) != 0 {
		t.Fatal("无引用应返回空数组")
	}
}

func TestBuildPassages(t *testing.T) {
	chunks := []rag.ScoredChunk{
		{SourceID: "a", SourceTitle: "来源甲", ChunkIndex: 1, Content: "重叠段后续\n" + strings.Repeat("文", 160), Score: 0.8},
		{SourceID: "b", SourceTitle: "来源乙", ChunkIndex: 0, Content: "乙正文", Score: 0.7},
		{SourceID: "a", SourceTitle: "来源甲", ChunkIndex: 0, Content: "开头重叠段", Score: 0.6},
		{SourceID: "a", ChunkIndex: 3, Content: "不相邻", Score: 0.5},
		{SourceID: "c", Content: "低相似度", Score: MinSimilarity - 0.01},
	}
	passages := BuildPassages(chunks)
	if len(passages) != 3 || passages[0].Citation.SourceID != "a" || passages[1].Citation.SourceID != "b" || passages[2].Content != "不相邻" {
		t.Fatalf("未按首次出现顺序合并相邻块：%v", passages)
	}
	if !strings.HasPrefix(passages[0].Content, "开头重叠段后续") || len([]rune(passages[0].Citation.Quote)) != 150 || strings.ContainsAny(passages[0].Citation.Quote, "\r\n") {
		t.Fatal("合并重叠或中文引用摘录不正确")
	}
	for i, passage := range passages {
		if passage.Citation.Index != i+1 {
			t.Fatal("引用编号不连续")
		}
	}
}
