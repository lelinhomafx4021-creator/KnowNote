package rag

import (
	"knownote/internal/parser"
	"strings"
	"unicode"
)

const ChunkSize = 500
const ChunkOverlap = 80

// Chunk 按 Unicode 字符计数，优先在后半段的段落或句末切分，保留 80 字上下文。
// 最后不足 100 字时并入前块，因此单块最多 599 字。
func Chunk(text string) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	var chunks []string
	for start := 0; start < len(runes); {
		end := min(start+ChunkSize, len(runes))
		if end < len(runes) {
			paragraph, sentence := 0, 0
			for i := start + ChunkSize/2; i < end; i++ {
				if runes[i] == '\n' {
					paragraph = i + 1
				}
				if strings.ContainsRune("。！？.!?；;", runes[i]) {
					sentence = i + 1
				}
			}
			if paragraph > 0 {
				end = paragraph
			} else if sentence > 0 {
				end = sentence
			}
			if len(runes)-end < 100 {
				end = len(runes)
			}
		}
		chunks = append(chunks, string(runes[start:end]))
		if end == len(runes) {
			break
		}
		start = end - ChunkOverlap
	}
	return chunks
}

// WordCount 与预览、切分一样按字符计算，忽略空白。
func WordCount(text string) int {
	count := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}

// ChunkSections 对每个分节独立切分，重叠区间不会跨越位置边界。
func ChunkSections(sections []parser.Section) ([]string, []Location) {
	var texts []string
	var positions []Location
	for i, section := range sections {
		for _, part := range Chunk(section.Content) {
			texts = append(texts, part)
			positions = append(positions, Location{Page: section.Page, Label: section.Label, Index: i + 1})
		}
	}
	return texts, positions
}

type Location struct {
	Page, Index int
	Label       string
}
