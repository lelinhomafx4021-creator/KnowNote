// Package testfixture 在内存中生成多格式测试资料，不依赖外部样例或系统软件。
package testfixture

import (
	"archive/zip"
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const WordNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
const SlideNS = `xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
const relNS = `xmlns="http://schemas.openxmlformats.org/package/2006/relationships"`

func ZIP(files map[string]string) []byte {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	var names []string
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry, _ := writer.Create(name)
		_, _ = entry.Write([]byte(files[name]))
	}
	_ = writer.Close()
	return buffer.Bytes()
}
func DOCX() []byte {
	return ZIP(map[string]string{
		"[Content_Types].xml":          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`,
		"_rels/.rels":                  `<Relationships ` + relNS + `><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships ` + relNS + `><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"word/styles.xml":              `<w:styles ` + WordNS + `><w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:pPr><w:outlineLvl w:val="0"/></w:pPr></w:style><w:style w:type="paragraph" w:styleId="Custom"><w:basedOn w:val="Heading1"/></w:style></w:styles>`,
		"word/document.xml":            `<w:document ` + WordNS + `><w:body><w:p><w:pPr><w:pStyle w:val="Custom"/></w:pPr><w:r><w:t>知识管理</w:t></w:r></w:p><w:p><w:r><w:t>知页支持资料检索和带引用问答。</w:t></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>功能</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>说明</w:t></w:r></w:p></w:tc></w:tr><w:tr><w:tc><w:p><w:r><w:t>检索</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>向量引用</w:t></w:r></w:p></w:tc></w:tr></w:tbl><w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>学习计划</w:t></w:r></w:p><w:p><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>整理资料</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr></w:body></w:document>`,
	})
}
func PPTX() []byte {
	title := func(text string) string {
		return `<p:sp><p:nvSpPr><p:cNvPr id="2" name="标题"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>` + text + `</a:t></a:r></a:p></p:txBody></p:sp>`
	}
	slide := func(body string) string {
		return `<p:sld ` + SlideNS + `><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>` + body + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`
	}
	return ZIP(map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/><Override PartName="/ppt/slides/slide2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/><Override PartName="/ppt/notesSlides/notesSlide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"/></Types>`,
		"_rels/.rels":                      `<Relationships ` + relNS + `><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation ` + SlideNS + `><p:sldIdLst><p:sldId id="256" r:id="rSecond"/><p:sldId id="257" r:id="rFirst"/></p:sldIdLst><p:sldSz cx="9144000" cy="6858000"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships ` + relNS + `><Relationship Id="rFirst" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/><Relationship Id="rSecond" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":            slide(title("第二张：学习计划")),
		"ppt/slides/slide2.xml":            slide(title("第一张：知识管理") + `<p:sp><p:nvSpPr><p:cNvPr id="3" name="正文"/><p:cNvSpPr/><p:nvPr><p:ph type="body"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:pPr lvl="0"><a:buChar char="•"/></a:pPr><a:r><a:t>资料检索</a:t></a:r></a:p><a:p><a:pPr lvl="1"><a:buChar char="•"/></a:pPr><a:r><a:t>带引用问答</a:t></a:r></a:p></p:txBody></p:sp><p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="4" name="表格"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr/><a:tblGrid><a:gridCol w="1000000"/><a:gridCol w="1000000"/></a:tblGrid><a:tr h="100000"><a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>功能</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc><a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>价值</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc></a:tr><a:tr h="100000"><a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>引用</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc><a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>可追溯</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc></a:tr></a:tbl></a:graphicData></a:graphic></p:graphicFrame>`),
		"ppt/slides/_rels/slide2.xml.rels": `<Relationships ` + relNS + `><Relationship Id="rNotes" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide1.xml"/></Relationships>`,
		"ppt/notesSlides/notesSlide1.xml":  `<p:notes ` + SlideNS + `><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="备注"/><p:cNvSpPr/><p:nvPr><p:ph type="body"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:p><a:r><a:t>演讲者提醒：核对资料原文。</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:notes>`,
	})
}
func XLSX() ([]byte, error) {
	book := excelize.NewFile()
	defer book.Close()
	book.SetSheetName("Sheet1", "成绩")
	book.NewSheet("计划")
	for sheet, rows := range map[string][][]any{"成绩": {{"姓名", "分数"}, {"小知", 95}, {"小页", 98}}, "计划": {{"任务", "状态"}, {"知识管理", "已完成"}}} {
		for i, row := range rows {
			cell, _ := excelize.CoordinatesToCellName(1, i+1)
			if err := book.SetSheetRow(sheet, cell, &row); err != nil {
				return nil, err
			}
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
func CSV() []byte {
	data, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("姓名;任务\n小知;资料检索\n小页;知识管理\n"))
	return data
}

// PDF 直接生成带 ToUnicode 字符映射的多页中文 PDF，无需字体文件或外部程序。
func PDF() []byte {
	objects := []string{"", "", `<< /Type /Font /Subtype /Type0 /BaseFont /STSong-Light /Encoding /UniGB-UCS2-H /DescendantFonts [4 0 R] /ToUnicode 5 0 R >>`, `<< /Type /Font /Subtype /CIDFontType0 /BaseFont /STSong-Light /CIDSystemInfo << /Registry (Adobe) /Ordering (GB1) /Supplement 4 >> /DW 1000 >>`}
	cmap := `/CIDInit /ProcSet findresource begin 12 dict begin begincmap /CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def /CMapName /Adobe-Identity-UCS def /CMapType 2 def 1 begincodespacerange <0000> <FFFF> endcodespacerange 1 beginbfrange <0000> <FFFF> <0000> endbfrange endcmap CMapName currentdict /CMap defineresource pop end end`
	objects = append(objects, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(cmap), cmap))
	var kids []string
	for page := 1; page <= 3; page++ {
		lines := []string{"知页文档页眉", fmt.Sprintf("第%d页：知识管理与资料检索。", page), "KnowNote supports multi-page retrieval.", "The docu-", "ment contains citations.", "固定页脚"}
		var stream strings.Builder
		stream.WriteString("BT /F1 12 Tf 20 TL 60 740 Td\n")
		for i, line := range lines {
			if i > 0 {
				stream.WriteString("T*\n")
			}
			stream.WriteString("<")
			for _, r := range utf16.Encode([]rune(line)) {
				fmt.Fprintf(&stream, "%04X", r)
			}
			stream.WriteString("> Tj\n")
		}
		stream.WriteString("ET")
		pageID := len(objects) + 1
		contentID := pageID + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageID))
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", contentID), fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", stream.Len(), stream.String()))
	}
	objects[0] = "<< /Type /Catalog /Pages 2 0 R >>"
	objects[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count 3 >>", strings.Join(kids, " "))
	var buffer bytes.Buffer
	buffer.WriteString("%PDF-1.7\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, buffer.Len())
		fmt.Fprintf(&buffer, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := buffer.Len()
	fmt.Fprintf(&buffer, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&buffer, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buffer, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return buffer.Bytes()
}
