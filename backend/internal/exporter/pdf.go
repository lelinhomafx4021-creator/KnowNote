package exporter

import (
	"context"
	"fmt"
	"os"
	"strings"

	"knownote/internal/model"
	"knownote/internal/parser"
)

type exportSource struct {
	title    string
	subtitle string
	markdown string
	slides   *model.SlideDeck
	prefer   string
}

// ExportArtifact 将工作室产物导出为 pptx、docx 或 pdf 文件。
func ExportArtifact(ctx context.Context, art model.Artifact, format string) (data []byte, filename string, contentType string, err error) {
	if format == "" {
		format = art.Format
	}
	if format == "" {
		format = DefaultFormat(art.Kind)
	}
	markdown := art.Content
	if art.Mindmap != nil && strings.TrimSpace(markdown) == "" {
		markdown = MindmapToMarkdown(art.Mindmap)
	}
	if art.Slides != nil && strings.TrimSpace(markdown) == "" {
		markdown = SlideDeckToMarkdown(art.Slides)
	}
	subtitle := "基于课程来源生成 · 知页 KnowNote"
	if art.RetrievalMode == "rag" && strings.TrimSpace(art.Prompt) != "" {
		subtitle = "RAG 定向检索主题：" + clipExportText(art.Prompt, 48)
	}
	return exportDocument(ctx, exportSource{
		title:    art.Title,
		subtitle: subtitle,
		markdown: markdown,
		slides:   art.Slides,
		prefer:   DefaultFormat(art.Kind),
	}, format)
}

// ExportNote 将学习笔记导出为 docx、pdf 或 pptx 文件。
func ExportNote(ctx context.Context, note model.Note, format string) (data []byte, filename string, contentType string, err error) {
	if format == "" {
		format = FormatDOCX
	}
	return exportDocument(ctx, exportSource{
		title:    note.Title,
		subtitle: "知页学习笔记 · 可编辑导出文档",
		markdown: note.Content,
		prefer:   FormatDOCX,
	}, format)
}

func exportDocument(ctx context.Context, src exportSource, format string) ([]byte, string, string, error) {
	contentType, ext, err := ContentType(format)
	if err != nil {
		return nil, "", "", err
	}
	filename := SafeFilename(src.title, ext)

	var data []byte
	switch ext {
	case FormatPPTX:
		var deck model.SlideDeck
		if src.slides != nil {
			deck = *src.slides
		} else {
			deck = MarkdownToSlideDeck(src.title, src.markdown)
		}
		data, err = BuildPPTX(deck, src.subtitle)
	case FormatDOCX:
		data, err = BuildDOCX(src.title, src.subtitle, src.markdown)
	case FormatPDF:
		var intermediate []byte
		intermediateExt := FormatDOCX
		if src.slides != nil && src.prefer == FormatPPTX {
			intermediateExt = FormatPPTX
			intermediate, err = BuildPPTX(*src.slides, src.subtitle)
		} else {
			intermediate, err = BuildDOCX(src.title, src.subtitle, src.markdown)
		}
		if err == nil {
			data, err = ConvertToPDF(ctx, intermediate, intermediateExt)
		}
	}
	if err != nil {
		return nil, "", "", err
	}
	return data, filename, contentType, nil
}

// ConvertToPDF 使用项目内集成的 LibreOffice 转换管道将 docx/pptx 渲染为带中文字体的 PDF。
func ConvertToPDF(ctx context.Context, officeBytes []byte, ext string) ([]byte, error) {
	if _, err := parser.SofficePath(); err != nil {
		return nil, fmt.Errorf("需要安装 LibreOffice 才能导出 PDF 文件")
	}
	if ext != FormatDOCX && ext != FormatPPTX {
		return nil, fmt.Errorf("不支持转换为 PDF 的中间格式")
	}
	tmp, err := os.CreateTemp("", "knownote-export-*."+ext)
	if err != nil {
		return nil, fmt.Errorf("创建临时导出文件失败")
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(officeBytes); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("写入临时导出文件失败")
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("关闭临时导出文件失败")
	}
	pdfPath, cleanup, err := parser.Convert(ctx, tmpPath, "pdf")
	if err != nil {
		return nil, err
	}
	defer cleanup()
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil || len(pdfBytes) == 0 {
		return nil, fmt.Errorf("读取生成的 PDF 文件失败")
	}
	return pdfBytes, nil
}
