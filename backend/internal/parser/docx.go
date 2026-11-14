package parser

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func parseDOCX(ctx context.Context, file *os.File) (Result, error) {
	z, err := openOffice(ctx, file)
	if err != nil {
		return Result{}, err
	}
	root, err := z.xml("word/document.xml")
	if err != nil {
		return Result{}, err
	}
	styles := map[string]*xmlNode{}
	if z.files["word/styles.xml"] != nil {
		styleRoot, err := z.xml("word/styles.xml")
		if err != nil {
			return Result{}, err
		}
		for _, style := range styleRoot.Children {
			styles[style.value("styleId")] = style
		}
	}
	var heading func(string, int) int
	heading = func(id string, depth int) int {
		if depth > 10 {
			return 0
		}
		normalized := strings.ToLower(strings.ReplaceAll(id, " ", ""))
		for _, prefix := range []string{"heading", "标题"} {
			if strings.HasPrefix(normalized, prefix) {
				v, _ := strconv.Atoi(strings.TrimPrefix(normalized, prefix))
				if v >= 1 && v <= 9 {
					return min(v, 6)
				}
			}
		}
		style := styles[id]
		if style == nil {
			return 0
		}
		if level := style.child("pPr").child("outlineLvl"); level != nil {
			v, e := strconv.Atoi(level.value("val"))
			if e == nil && v < 9 {
				return min(v+1, 6)
			}
		}
		if name := style.child("name").value("val"); name != "" && name != id {
			for _, prefix := range []string{"heading ", "标题 "} {
				if strings.HasPrefix(strings.ToLower(name), prefix) {
					v, _ := strconv.Atoi(strings.TrimPrefix(strings.ToLower(name), prefix))
					if v >= 1 && v <= 9 {
						return min(v, 6)
					}
				}
			}
		}
		return heading(style.child("basedOn").value("val"), depth+1)
	}
	// 编号表决定有序/无序列表，保持段落层级。
	numbering := map[string]map[string]string{}
	numRefs := map[string]string{}
	if z.files["word/numbering.xml"] != nil {
		n, err := z.xml("word/numbering.xml")
		if err != nil {
			return Result{}, err
		}
		for _, a := range n.Children {
			if a.Name == "abstractNum" {
				levels := map[string]string{}
				for _, l := range a.Children {
					if l.Name == "lvl" {
						levels[l.value("ilvl")] = l.child("numFmt").value("val")
					}
				}
				numbering[a.value("abstractNumId")] = levels
			}
			if a.Name == "num" {
				numRefs[a.value("numId")] = a.child("abstractNumId").value("val")
			}
		}
	}
	sections := []Section{}
	current := Section{}
	flush := func() {
		if strings.TrimSpace(current.Content) != "" {
			current.Page = len(sections) + 1
			if current.Label == "" {
				current.Label = "正文"
			}
			sections = append(sections, current)
		}
		current = Section{}
	}
	body := root.child("body")
	if body == nil {
		return Result{}, fmt.Errorf("DOCX 缺少正文")
	}
	for _, block := range body.Children {
		switch block.Name {
		case "p":
			text := block.text()
			if strings.TrimSpace(text) == "" {
				continue
			}
			props := block.child("pPr")
			level := heading(props.child("pStyle").value("val"), 0)
			if outline := props.child("outlineLvl"); outline != nil {
				v, e := strconv.Atoi(outline.value("val"))
				if e == nil && v >= 0 && v < 9 {
					level = min(v+1, 6)
				}
			}
			if level > 0 {
				flush()
				current.Title = Clean(text)
				current.Label = "章节：" + current.Title
				text = strings.Repeat("#", level) + " " + text
			} else if num := props.child("numPr"); num != nil && num.child("numId").value("val") != "0" {
				lv := num.child("ilvl").value("val")
				indent, _ := strconv.Atoi(lv)
				marker := "- "
				format := numbering[numRefs[num.child("numId").value("val")]][lv]
				if format != "" && format != "bullet" && format != "none" {
					marker = "1. "
				}
				text = strings.Repeat("  ", min(8, max(0, indent))) + marker + text
			}
			current.Content += text + "\n\n"
		case "tbl":
			var rows [][]string
			for _, row := range block.Children {
				if row.Name != "tr" {
					continue
				}
				var cells []string
				for _, cell := range row.Children {
					if cell.Name != "tc" {
						continue
					}
					var parts []string
					for _, p := range cell.all("p") {
						parts = append(parts, p.text())
					}
					cells = append(cells, strings.Join(parts, "\n"))
				}
				rows = append(rows, cells)
			}
			current.Content += markdownTable(rows) + "\n\n"
		}
	}
	flush()
	return sectionResult(sections, 0), nil
}
