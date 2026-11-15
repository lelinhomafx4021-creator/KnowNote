package exporter

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"mime"
	"regexp"
	"strings"
	"unicode/utf8"

	"knownote/internal/model"
)

const (
	FormatPPTX = "pptx"
	FormatDOCX = "docx"
	FormatPDF  = "pdf"

	mimePPTX = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	mimeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	mimePDF  = "application/pdf"
)

// ContentType 返回导出格式对应的 MIME 类型和文件扩展名。
func ContentType(format string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatPPTX:
		return mimePPTX, FormatPPTX, nil
	case FormatDOCX:
		return mimeDOCX, FormatDOCX, nil
	case FormatPDF:
		return mimePDF, FormatPDF, nil
	default:
		return "", "", fmt.Errorf("不支持的导出格式")
	}
}

// DefaultFormat 返回产物类型的默认导出格式。
func DefaultFormat(kind string) string {
	switch kind {
	case "ppt":
		return FormatPPTX
	case "pdf":
		return FormatPDF
	default:
		return FormatDOCX
	}
}

// Disposition 生成安全的下载或预览响应头（若传入名已含正确扩展名则不再重复追加）。
func Disposition(name, ext string, inline bool) string {
	mode := "attachment"
	if inline && ext == FormatPDF {
		mode = "inline"
	}
	base := strings.TrimSuffix(name, "."+ext)
	filename := SafeFilename(base, ext)
	header := mime.FormatMediaType(mode, map[string]string{"filename": filename})
	if header == "" {
		return fmt.Sprintf(`%s; filename="export.%s"`, mode, ext)
	}
	return header
}

// SafeFilename 清理文件名中的路径与控制字符。
func SafeFilename(title, ext string) string {
	clean := strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`\/:*?"<>|`, r) {
			return ' '
		}
		return r
	}, strings.TrimSpace(title))
	clean = strings.Join(strings.Fields(clean), " ")
	if runes := []rune(clean); len(runes) > 64 {
		clean = strings.TrimSpace(string(runes[:64]))
	}
	if clean == "" {
		clean = "知页导出"
	}
	return clean + "." + ext
}

type zipEntry struct {
	name string
	data string
}

func buildZip(entries []zipEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, entry := range entries {
		w, err := zw.Create(entry.name)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("写入压缩包条目失败: %w", err)
		}
		if _, err := w.Write([]byte(entry.data)); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("写入压缩包内容失败: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("打包导出文件失败: %w", err)
	}
	return buf.Bytes(), nil
}

func escapeXML(text string) string {
	// 去除 OOXML 不允许的控制字符（保留 \t \n \r）。
	sanitized := strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || r >= 0x20 {
			return r
		}
		return -1
	}, text)
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(sanitized))
	return b.String()
}

type inlineRun struct {
	text     string
	bold     bool
	code     bool
	citation bool
}

var inlineTokenPattern = regexp.MustCompile("(\\*\\*[^\\n*]+\\*\\*|`[^`\\n]+`|\\[[1-9][0-9]*\\])")

// parseInlineSpans 解析 **加粗**、`行内代码` 与 [n] 引用标注。
func parseInlineSpans(text string) []inlineRun {
	matches := inlineTokenPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		if text == "" {
			return nil
		}
		return []inlineRun{{text: text}}
	}
	var runs []inlineRun
	pos := 0
	for _, m := range matches {
		if m[0] > pos {
			runs = append(runs, inlineRun{text: text[pos:m[0]]})
		}
		token := text[m[0]:m[1]]
		switch {
		case strings.HasPrefix(token, "**") && strings.HasSuffix(token, "**"):
			runs = append(runs, inlineRun{text: token[2 : len(token)-2], bold: true})
		case strings.HasPrefix(token, "`") && strings.HasSuffix(token, "`"):
			runs = append(runs, inlineRun{text: token[1 : len(token)-1], code: true})
		default:
			runs = append(runs, inlineRun{text: token, bold: true, citation: true})
		}
		pos = m[1]
	}
	if pos < len(text) {
		runs = append(runs, inlineRun{text: text[pos:]})
	}
	return runs
}

type blockKind int

const (
	blockHeading blockKind = iota + 1
	blockParagraph
	blockBullet
	blockOrdered
	blockQuote
	blockTable
	blockRule
)

type mdBlock struct {
	kind  blockKind
	level int
	text  string
	rows  [][]string
}

var (
	orderedLinePattern  = regexp.MustCompile(`^([0-9]+)[.)]\s+(.*)$`)
	tableDividerPattern = regexp.MustCompile(`^\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?$`)
)

func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isTableLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.Contains(trimmed, "|") && len(splitTableRow(trimmed)) >= 2
}

// parseMarkdown 将常见的 Markdown 文本拆分为结构化块，供 Word 与 PPT 转换复用。
func parseMarkdown(md string) []mdBlock {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var blocks []mdBlock
	var para []string
	flushPara := func() {
		if len(para) == 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(para, " "))
		if text != "" {
			blocks = append(blocks, mdBlock{kind: blockParagraph, text: text})
		}
		para = nil
	}

	inCode := false
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "```") {
			flushPara()
			inCode = !inCode
			continue
		}
		if inCode {
			if trimmed != "" {
				blocks = append(blocks, mdBlock{kind: blockQuote, text: trimmed})
			}
			continue
		}
		if trimmed == "" {
			flushPara()
			continue
		}
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			flushPara()
			blocks = append(blocks, mdBlock{kind: blockRule})
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level >= 1 && level <= 6 && len(trimmed) > level && trimmed[level] == ' ' {
				flushPara()
				blocks = append(blocks, mdBlock{kind: blockHeading, level: min(level, 3), text: strings.TrimSpace(trimmed[level+1:])})
				continue
			}
		}
		if isTableLine(trimmed) && i+1 < len(lines) && tableDividerPattern.MatchString(strings.TrimSpace(lines[i+1])) {
			flushPara()
			rows := [][]string{splitTableRow(trimmed)}
			cols := len(rows[0])
			i += 2
			for i < len(lines) && isTableLine(lines[i]) {
				row := splitTableRow(lines[i])
				for len(row) < cols {
					row = append(row, "")
				}
				rows = append(rows, row[:cols])
				i++
			}
			i--
			blocks = append(blocks, mdBlock{kind: blockTable, rows: rows})
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			flushPara()
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			if text != "" {
				blocks = append(blocks, mdBlock{kind: blockQuote, text: text})
			}
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "• ") {
			flushPara()
			indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
			text := strings.TrimSpace(trimmed[2:])
			if strings.HasPrefix(trimmed, "• ") {
				text = strings.TrimSpace(strings.TrimPrefix(trimmed, "• "))
			}
			if text != "" {
				blocks = append(blocks, mdBlock{kind: blockBullet, level: min(indent/2, 2), text: text})
			}
			continue
		}
		if match := orderedLinePattern.FindStringSubmatch(trimmed); len(match) == 3 {
			flushPara()
			if text := strings.TrimSpace(match[2]); text != "" {
				blocks = append(blocks, mdBlock{kind: blockOrdered, level: 0, text: text})
			}
			continue
		}
		para = append(para, trimmed)
	}
	flushPara()
	return blocks
}

// MindmapToMarkdown 将思维导图树展开为带层级的 Markdown，便于导出成 Word/PDF/PPT。
func MindmapToMarkdown(root *model.MindMapNode) string {
	if root == nil {
		return ""
	}
	var b strings.Builder
	var walk func(node model.MindMapNode, depth int)
	walk = func(node model.MindMapNode, depth int) {
		switch depth {
		case 0:
			fmt.Fprintf(&b, "# %s\n\n", node.Label)
		case 1:
			fmt.Fprintf(&b, "\n## %s\n", node.Label)
		case 2:
			fmt.Fprintf(&b, "- **%s**\n", node.Label)
		default:
			fmt.Fprintf(&b, "%s- %s\n", strings.Repeat("  ", depth-2), node.Label)
		}
		for _, child := range node.Children {
			walk(child, depth+1)
		}
	}
	walk(*root, 0)
	return strings.TrimSpace(b.String())
}

// SlideDeckToMarkdown 将幻灯片大纲与讲稿整理为 Markdown，供保存笔记和文档导出复用。
func SlideDeckToMarkdown(deck *model.SlideDeck) string {
	if deck == nil {
		return ""
	}
	var b strings.Builder
	if strings.TrimSpace(deck.Subtitle) != "" {
		fmt.Fprintf(&b, "> %s\n\n", strings.TrimSpace(deck.Subtitle))
	}
	for i, slide := range deck.Slides {
		fmt.Fprintf(&b, "## 第%d页 · %s\n", i+1, slide.Title)
		if strings.TrimSpace(slide.Subtitle) != "" {
			fmt.Fprintf(&b, "*%s*\n\n", strings.TrimSpace(slide.Subtitle))
		}
		for _, bullet := range slide.Bullets {
			fmt.Fprintf(&b, "- %s\n", strings.TrimSpace(bullet))
		}
		if strings.TrimSpace(slide.Notes) != "" {
			fmt.Fprintf(&b, "\n> **教师讲稿**：%s\n", strings.TrimSpace(slide.Notes))
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// MarkdownToSlideDeck 将任意 Markdown 文本自动分节转换为幻灯片集合，支持跨格式导出 PPTX。
func MarkdownToSlideDeck(title, md string) model.SlideDeck {
	blocks := parseMarkdown(md)
	deckTitle := strings.TrimSpace(title)
	if deckTitle == "" {
		deckTitle = "知页知识课件"
	}
	var slides []model.Slide
	current := model.Slide{Title: deckTitle}
	flush := func() {
		if len(current.Bullets) > 0 || strings.TrimSpace(current.Notes) != "" {
			// 每页最多放 6 条要点，避免投影版面拥挤。
			for start := 0; start < max(1, len(current.Bullets)); start += 6 {
				end := min(len(current.Bullets), start+6)
				pageTitle := current.Title
				if start > 0 {
					pageTitle = fmt.Sprintf("%s（续）", current.Title)
				}
				slide := model.Slide{
					Title:    pageTitle,
					Subtitle: current.Subtitle,
					Bullets:  current.Bullets[start:end],
				}
				if start == 0 {
					slide.Notes = current.Notes
				}
				slides = append(slides, slide)
			}
		}
		current = model.Slide{}
	}
	stripInline := func(s string) string {
		s = strings.ReplaceAll(s, "**", "")
		s = strings.ReplaceAll(s, "`", "")
		return strings.TrimSpace(s)
	}
	for _, block := range blocks {
		switch block.kind {
		case blockHeading:
			if block.level == 1 && len(slides) == 0 && len(current.Bullets) == 0 {
				deckTitle = stripInline(block.text)
				current.Title = deckTitle
				continue
			}
			flush()
			current.Title = stripInline(block.text)
		case blockBullet, blockOrdered:
			current.Bullets = append(current.Bullets, clipExportText(stripInline(block.text), 110))
		case blockQuote:
			text := stripInline(block.text)
			if current.Notes == "" {
				current.Notes = text
			} else {
				current.Notes += "\n" + text
			}
		case blockParagraph:
			text := stripInline(block.text)
			if text != "" {
				current.Bullets = append(current.Bullets, clipExportText(text, 110))
			}
		case blockTable:
			if len(block.rows) > 1 {
				headers := block.rows[0]
				for _, row := range block.rows[1:] {
					var pairs []string
					for idx, cell := range row {
						if strings.TrimSpace(cell) == "" {
							continue
						}
						if idx < len(headers) && headers[idx] != "" {
							pairs = append(pairs, headers[idx]+": "+cell)
						} else {
							pairs = append(pairs, cell)
						}
					}
					if len(pairs) > 0 {
						current.Bullets = append(current.Bullets, clipExportText(strings.Join(pairs, "；"), 110))
					}
				}
			}
		}
	}
	flush()
	if len(slides) == 0 {
		slides = append(slides, model.Slide{Title: deckTitle, Bullets: []string{"暂无结构化正文内容"}})
	}
	return model.SlideDeck{Title: deckTitle, Subtitle: "由知页 KnowNote 基于学习资料生成", Slides: slides}
}

func clipExportText(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit-1]) + "…"
}
