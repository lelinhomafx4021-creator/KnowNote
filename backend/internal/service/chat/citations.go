package chat

import (
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
func BuildPassages(chunks []rag.ScoredChunk) []rag.Passage {
	type group struct {
		first  int
		chunks []rag.ScoredChunk
	}
	bySource := make(map[string][]int)
	for i, chunk := range chunks {
		if chunk.Score >= MinSimilarity && strings.TrimSpace(chunk.Content) != "" {
			bySource[chunk.SourceID] = append(bySource[chunk.SourceID], i)
		}
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
			Index: i + 1, SourceID: first.SourceID, SourceTitle: first.SourceTitle, Quote: string(quote[:min(150, len(quote))]),
		}})
	}
	return passages
}
