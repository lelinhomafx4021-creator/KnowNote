package parser

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func slideParagraphs(node *xmlNode, list bool, defaultBullet ...bool) string {
	var out []string
	for _, p := range node.all("p") {
		text := p.text()
		if strings.TrimSpace(text) == "" {
			continue
		}
		props := p.child("pPr")
		level, _ := strconv.Atoi(props.value("lvl"))
		marker := ""
		inherited := node.child("lstStyle").child(fmt.Sprintf("lvl%dpPr", level+1))
		bullet := props.child("buChar") != nil || props.child("buAutoNum") != nil || props.value("lvl") != "" || inherited.child("buChar") != nil || inherited.child("buAutoNum") != nil || (len(defaultBullet) > 0 && defaultBullet[0])
		if list && props.child("buNone") == nil && inherited.child("buNone") == nil && bullet {
			marker = "- "
			if props.child("buAutoNum") != nil || inherited.child("buAutoNum") != nil {
				marker = "1. "
			}
		}
		if marker != "" {
			text = strings.Repeat("  ", min(8, max(0, level))) + marker + text
		}
		out = append(out, text)
	}
	return strings.Join(out, "\n")
}
func parsePPTX(ctx context.Context, file *os.File) (Result, error) {
	z, err := openOffice(ctx, file)
	if err != nil {
		return Result{}, err
	}
	presentation, err := z.xml("ppt/presentation.xml")
	if err != nil {
		return Result{}, err
	}
	rels, err := z.rels("ppt/presentation.xml")
	if err != nil {
		return Result{}, err
	}
	var sections []Section
	for _, id := range presentation.all("sldId") {
		part := rels[id.value("rel:id")]
		if part == "" {
			return Result{}, fmt.Errorf("PPTX 幻灯片关联缺失")
		}
		slide, err := z.xml(part)
		if err != nil {
			return Result{}, err
		}
		section := Section{Page: len(sections) + 1}
		var body []string
		// 按 XML 中的形状次序处理文本框、分组与表格。
		var visit func(*xmlNode)
		visit = func(node *xmlNode) {
			if node.Name == "sp" {
				kind := ""
				for _, ph := range node.all("ph") {
					kind = ph.value("type")
					break
				}
				text := slideParagraphs(node.child("txBody"), true, kind == "body")
				if kind == "title" || kind == "ctrTitle" {
					section.Title = Clean(slideParagraphs(node.child("txBody"), false))
					if section.Title != "" {
						body = append(body, "# "+section.Title)
					}
				} else if text != "" {
					body = append(body, text)
				}
				return
			}
			if node.Name == "tbl" {
				var rows [][]string
				for _, row := range node.Children {
					if row.Name != "tr" {
						continue
					}
					var cells []string
					for _, cell := range row.Children {
						if cell.Name == "tc" {
							cells = append(cells, slideParagraphs(cell, false))
						}
					}
					rows = append(rows, cells)
				}
				body = append(body, markdownTable(rows))
				return
			}
			for _, child := range node.Children {
				visit(child)
			}
		}
		visit(slide)
		slideRels, err := z.rels(part)
		if err != nil {
			return Result{}, err
		}
		// notesSlide 通过 slide 的 relationship 寻址，禁止用文件名猜测关联。
		for _, target := range slideRels {
			if !strings.HasPrefix(target, "ppt/notesSlides/") || !strings.HasSuffix(target, ".xml") {
				continue
			}
			notes, err := z.xml(target)
			if err != nil {
				return Result{}, err
			}
			var texts []string
			for _, shape := range notes.all("sp") {
				kind := ""
				for _, ph := range shape.all("ph") {
					kind = ph.value("type")
					break
				}
				if kind == "sldNum" || kind == "hdr" || kind == "ftr" || kind == "dt" || kind == "sldImg" {
					continue
				}
				if text := slideParagraphs(shape.child("txBody"), true); text != "" {
					texts = append(texts, text)
				}
			}
			if len(texts) > 0 {
				body = append(body, "备注：\n"+strings.Join(texts, "\n"))
			}
		}
		section.Label = fmt.Sprintf("第 %d 张幻灯片", section.Page)
		if section.Title != "" {
			section.Label += "：" + section.Title
		}
		section.Content = strings.Join(body, "\n\n")
		sections = append(sections, section)
	}
	if len(sections) == 0 {
		return Result{}, fmt.Errorf("PPTX 没有幻灯片")
	}
	return sectionResult(sections, len(sections)), nil
}
