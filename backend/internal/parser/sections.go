package parser

import "strings"

// Section 保留完整分节正文，接口只公开其摘要与位置。
type Section struct {
	Page                  int
	Label, Title, Content string
}

func sectionResult(sections []Section, pageCount int) Result {
	result := Result{Sections: sections, PageCount: pageCount}
	var texts []string
	for i := range result.Sections {
		result.Sections[i].Content = Clean(result.Sections[i].Content)
		texts = append(texts, result.Sections[i].Content)
	}
	result.Content = strings.Join(texts, "\n\n")
	return result
}
