package chat

import (
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"knownote/internal/model"
	"knownote/internal/rag"
)

const MinSimilarity = 0.2

var citationMarker = regexp.MustCompile(`\[([1-9][0-9]*)\]`)

// FilterCitations 只保留实际出现的编号，保留原来的编号与顺序。
func FilterCitations(content string, citations []model.Citation) []model.Citation {
	used := make(map[int]bool)
	for _, match := range citationMarker.FindAllStringSubmatch(content, -1) {
		index, err := strconv.Atoi(match[1])
		if err == nil {
			used[index] = true
		}
	}
	result := make([]model.Citation, 0)
	for _, citation := range citations {
		if used[citation.Index] {
			result = append(result, citation)
		}
	}
	return result
}

// BuildPassages 合并连续块后，按其最早检索位置编号；块内仍按原文顺序拼接。
func BuildPassages(chunks []rag.ScoredChunk, logger *slog.Logger) []rag.Passage {
	selected := make([]rag.ScoredChunk, 0, len(chunks))
	candidates := make([]rag.ScoredChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		candidates = append(candidates, chunk)
		if chunk.Score >= MinSimilarity {
			selected = append(selected, chunk)
		}
	}
	if len(selected) == 0 && len(candidates) > 0 {
		// 低分不等于没有资料：保留最相近的三块，仍由提示词约束资料不足时的回答。
		sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
		selected = candidates[:min(3, len(candidates))]
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("检索片段均低于阈值，使用最高相似度片段作为上下文", "threshold", MinSimilarity, "chunks", len(selected), "bestScore", selected[0].Score)
	}
	chunks = selected
	type group struct {
		first  int
		chunks []rag.ScoredChunk
	}
	bySource := make(map[string][]int)
	for i, chunk := range chunks {
		key := fmt.Sprintf("%s:%d:%v", chunk.SourceID, chunk.SectionIndex, pageValue(chunk.Page))
		bySource[key] = append(bySource[key], i)
	}
	var groups []group
	for _, positions := range bySource {
		sort.Slice(positions, func(i, j int) bool { return chunks[positions[i]].ChunkIndex < chunks[positions[j]].ChunkIndex })
		for i := 0; i < len(positions); {
			g := group{first: positions[i], chunks: []rag.ScoredChunk{chunks[positions[i]]}}
			j := i + 1
			for j < len(positions) && chunks[positions[j]].ChunkIndex == chunks[positions[j-1]].ChunkIndex+1 {
				g.first = min(g.first, positions[j])
				g.chunks = append(g.chunks, chunks[positions[j]])
				j++
			}
			groups = append(groups, g)
			i = j
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].first < groups[j].first })
	passages := make([]rag.Passage, 0, len(groups))
	for i, group := range groups {
		content := group.chunks[0].Content
		for _, chunk := range group.chunks[1:] {
			left, right := []rune(content), []rune(chunk.Content)
			overlap := 0
			for n := min(80, min(len(left), len(right))); n > 0; n-- {
				if string(left[len(left)-n:]) == string(right[:n]) {
					overlap = n
					break
				}
			}
			if overlap == 0 {
				content += "\n"
			}
			content += string(right[overlap:])
		}
		quote := []rune(strings.Join(strings.Fields(content), " "))
		first := group.chunks[0]
		passages = append(passages, rag.Passage{Content: content, Citation: model.Citation{
			ChunkID: first.ChunkID, Page: first.Page, LocationLabel: first.LocationLabel, Index: i + 1, SourceID: first.SourceID, SourceTitle: first.SourceTitle, Quote: string(quote[:min(150, len(quote))]),
		}})
	}
	return passages
}

func pageValue(page *int) int {
	if page == nil {
		return 0
	}
	return *page
}
