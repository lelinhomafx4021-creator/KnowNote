package exporter

import (
	"fmt"
	"strings"
)

const (
	docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
</Types>`

	docxRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

	docxDocRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
</Relationships>`

	docxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Hiragino Sans GB" w:cs="Calibri"/>
        <w:color w:val="1E293B"/>
        <w:sz w:val="21"/>
        <w:szCs w:val="21"/>
        <w:lang w:val="en-US" w:eastAsia="zh-CN"/>
      </w:rPr>
    </w:rPrDefault>
    <w:pPrDefault>
      <w:pPr>
        <w:spacing w:before="0" w:after="120" w:line="320" w:lineRule="auto"/>
      </w:pPr>
    </w:pPrDefault>
  </w:docDefaults>
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Title">
    <w:name w:val="Title"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:spacing w:before="40" w:after="100" w:line="280" w:lineRule="auto"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Hiragino Sans GB" w:cs="Calibri"/>
      <w:b/>
      <w:color w:val="0F172A"/>
      <w:sz w:val="40"/>
      <w:szCs w:val="40"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Subtitle">
    <w:name w:val="Subtitle"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:pBdr>
        <w:bottom w:val="single" w:sz="8" w:space="8" w:color="CBD5E1"/>
      </w:pBdr>
      <w:spacing w:before="0" w:after="240"/>
    </w:pPr>
    <w:rPr>
      <w:color w:val="64748B"/>
      <w:sz w:val="19"/>
      <w:szCs w:val="19"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:keepNext/>
      <w:spacing w:before="280" w:after="120"/>
      <w:outlineLvl w:val="0"/>
    </w:pPr>
    <w:rPr>
      <w:b/>
      <w:color w:val="0F172A"/>
      <w:sz w:val="32"/>
      <w:szCs w:val="32"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:keepNext/>
      <w:pBdr>
        <w:bottom w:val="single" w:sz="4" w:space="4" w:color="E2E8F0"/>
      </w:pBdr>
      <w:spacing w:before="240" w:after="100"/>
      <w:outlineLvl w:val="1"/>
    </w:pPr>
    <w:rPr>
      <w:b/>
      <w:color w:val="1D4ED8"/>
      <w:sz w:val="26"/>
      <w:szCs w:val="26"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading3">
    <w:name w:val="heading 3"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:keepNext/>
      <w:spacing w:before="180" w:after="80"/>
      <w:outlineLvl w:val="2"/>
    </w:pPr>
    <w:rPr>
      <w:b/>
      <w:color w:val="1E293B"/>
      <w:sz w:val="23"/>
      <w:szCs w:val="23"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="ListParagraph">
    <w:name w:val="List Paragraph"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:spacing w:before="0" w:after="80" w:line="300" w:lineRule="auto"/>
    </w:pPr>
  </w:style>
</w:styles>`

	docxNumbering = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="1">
    <w:lvl w:ilvl="0">
      <w:start w:val="1"/>
      <w:numFmt w:val="bullet"/>
      <w:lvlText w:val="•"/>
      <w:lvlJc w:val="left"/>
      <w:pPr><w:ind w:left="420" w:hanging="240"/></w:pPr>
    </w:lvl>
    <w:lvl w:ilvl="1">
      <w:start w:val="1"/>
      <w:numFmt w:val="bullet"/>
      <w:lvlText w:val="◦"/>
      <w:lvlJc w:val="left"/>
      <w:pPr><w:ind w:left="780" w:hanging="240"/></w:pPr>
    </w:lvl>
    <w:lvl w:ilvl="2">
      <w:start w:val="1"/>
      <w:numFmt w:val="bullet"/>
      <w:lvlText w:val="▪"/>
      <w:lvlJc w:val="left"/>
      <w:pPr><w:ind w:left="1140" w:hanging="240"/></w:pPr>
    </w:lvl>
  </w:abstractNum>
  <w:abstractNum w:abstractNumId="2">
    <w:lvl w:ilvl="0">
      <w:start w:val="1"/>
      <w:numFmt w:val="decimal"/>
      <w:lvlText w:val="%1."/>
      <w:lvlJc w:val="left"/>
      <w:pPr><w:ind w:left="420" w:hanging="280"/></w:pPr>
    </w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="1"/></w:num>
  <w:num w:numId="2"><w:abstractNumId w:val="2"/></w:num>
</w:numbering>`
)

// BuildDOCX 将标题与 Markdown 正文生成标准 Word (.docx) 文档字节。
func BuildDOCX(title, subtitle, markdown string) ([]byte, error) {
	var body strings.Builder
	docTitle := strings.TrimSpace(title)
	if docTitle == "" {
		docTitle = "知页学习文档"
	}
	writeDocxStyledParagraph(&body, "Title", docTitle)
	meta := strings.TrimSpace(subtitle)
	if meta == "" {
		meta = "由知页 KnowNote 基于选定资料整理生成"
	}
	writeDocxStyledParagraph(&body, "Subtitle", meta)

	blocks := parseMarkdown(markdown)
	if len(blocks) == 0 {
		writeDocxInlineParagraph(&body, "", "暂无正文内容。")
	} else {
		for _, block := range blocks {
			switch block.kind {
			case blockHeading:
				style := fmt.Sprintf("Heading%d", min(3, max(1, block.level)))
				writeDocxInlineParagraph(&body, style, block.text)
			case blockParagraph:
				writeDocxInlineParagraph(&body, "", block.text)
			case blockBullet:
				writeDocxListParagraph(&body, 1, block.level, block.text)
			case blockOrdered:
				writeDocxListParagraph(&body, 2, 0, block.text)
			case blockQuote:
				writeDocxQuoteParagraph(&body, block.text)
			case blockRule:
				body.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="4" w:color="CBD5E1"/></w:pBdr><w:spacing w:before="120" w:after="160"/></w:pPr></w:p>`)
			case blockTable:
				writeDocxTable(&body, block.rows)
			}
		}
	}

	documentXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    %s
    <w:sectPr>
      <w:pgSz w:w="11906" w:h="16838"/>
      <w:pgMar w:top="1247" w:right="1247" w:bottom="1247" w:left="1247" w:header="708" w:footer="708"/>
    </w:sectPr>
  </w:body>
</w:document>`, body.String())

	return buildZip([]zipEntry{
		{name: "[Content_Types].xml", data: docxContentTypes},
		{name: "_rels/.rels", data: docxRootRels},
		{name: "word/_rels/document.xml.rels", data: docxDocRels},
		{name: "word/styles.xml", data: docxStyles},
		{name: "word/numbering.xml", data: docxNumbering},
		{name: "word/document.xml", data: documentXML},
	})
}

func writeDocxStyledParagraph(b *strings.Builder, style, text string) {
	b.WriteString(`<w:p><w:pPr><w:pStyle w:val="`)
	b.WriteString(style)
	b.WriteString(`"/></w:pPr><w:r><w:t xml:space="preserve">`)
	b.WriteString(escapeXML(text))
	b.WriteString(`</w:t></w:r></w:p>`)
}

func writeDocxInlineParagraph(b *strings.Builder, style, text string) {
	b.WriteString(`<w:p>`)
	if style != "" {
		b.WriteString(`<w:pPr><w:pStyle w:val="`)
		b.WriteString(style)
		b.WriteString(`"/></w:pPr>`)
	}
	writeDocxRuns(b, text, style != "")
	b.WriteString(`</w:p>`)
}

func writeDocxListParagraph(b *strings.Builder, numID, level int, text string) {
	fmt.Fprintf(b, `<w:p><w:pPr><w:pStyle w:val="ListParagraph"/><w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr></w:pPr>`, min(2, max(0, level)), numID)
	writeDocxRuns(b, text, false)
	b.WriteString(`</w:p>`)
}

func writeDocxQuoteParagraph(b *strings.Builder, text string) {
	b.WriteString(`<w:p><w:pPr><w:pBdr><w:left w:val="single" w:sz="18" w:space="10" w:color="2563EB"/></w:pBdr><w:shd w:val="clear" w:color="auto" w:fill="F8FAFC"/><w:spacing w:before="80" w:after="140"/><w:ind w:left="220" w:right="120"/></w:pPr>`)
	writeDocxRuns(b, text, false)
	b.WriteString(`</w:p>`)
}

func writeDocxRuns(b *strings.Builder, text string, forceBold bool) {
	runs := parseInlineSpans(text)
	if len(runs) == 0 {
		b.WriteString(`<w:r><w:t></w:t></w:r>`)
		return
	}
	for _, run := range runs {
		b.WriteString(`<w:r>`)
		if run.bold || run.code || run.citation || forceBold {
			b.WriteString(`<w:rPr>`)
			if run.code {
				b.WriteString(`<w:rFonts w:ascii="Menlo" w:hAnsi="Menlo" w:eastAsia="Hiragino Sans GB"/>`)
			}
			if run.bold || forceBold {
				b.WriteString(`<w:b/>`)
			}
			if run.citation {
				b.WriteString(`<w:color w:val="1D4ED8"/>`)
			} else if run.code {
				b.WriteString(`<w:color w:val="0F172A"/><w:shd w:val="clear" w:color="auto" w:fill="F1F5F9"/>`)
			}
			b.WriteString(`</w:rPr>`)
		}
		b.WriteString(`<w:t xml:space="preserve">`)
		b.WriteString(escapeXML(run.text))
		b.WriteString(`</w:t></w:r>`)
	}
}

func writeDocxTable(b *strings.Builder, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	cols := len(rows[0])
	if cols == 0 {
		return
	}
	colWidth := 9412 / cols
	b.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="9412" w:type="dxa"/><w:tblBorders>`)
	for _, side := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		fmt.Fprintf(b, `<w:%s w:val="single" w:sz="4" w:space="0" w:color="CBD5E1"/>`, side)
	}
	b.WriteString(`</w:tblBorders><w:tblCellMar><w:top w:w="90" w:type="dxa"/><w:left w:w="120" w:type="dxa"/><w:bottom w:w="90" w:type="dxa"/><w:right w:w="120" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblGrid>`)
	for c := 0; c < cols; c++ {
		fmt.Fprintf(b, `<w:gridCol w:w="%d"/>`, colWidth)
	}
	b.WriteString(`</w:tblGrid>`)
	for r, row := range rows {
		b.WriteString(`<w:tr>`)
		for c := 0; c < cols; c++ {
			cell := ""
			if c < len(row) {
				cell = row[c]
			}
			fmt.Fprintf(b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>`, colWidth)
			if r == 0 {
				b.WriteString(`<w:shd w:val="clear" w:color="auto" w:fill="EFF6FF"/>`)
			}
			b.WriteString(`</w:tcPr><w:p><w:pPr><w:spacing w:before="20" w:after="40" w:line="260" w:lineRule="auto"/></w:pPr>`)
			writeDocxRuns(b, cell, r == 0)
			b.WriteString(`</w:p></w:tc>`)
		}
		b.WriteString(`</w:tr>`)
	}
	b.WriteString(`</w:tbl><w:p><w:pPr><w:spacing w:before="0" w:after="100"/></w:pPr></w:p>`)
}
