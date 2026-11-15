package exporter

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"knownote/internal/model"
)

const (
	pptxRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`

	pptxTheme = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="KnowNote Theme">
  <a:themeElements>
    <a:clrScheme name="KnowNote">
      <a:dk1><a:srgbClr val="0F172A"/></a:dk1>
      <a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
      <a:dk2><a:srgbClr val="334155"/></a:dk2>
      <a:lt2><a:srgbClr val="F8FAFC"/></a:lt2>
      <a:accent1><a:srgbClr val="2563EB"/></a:accent1>
      <a:accent2><a:srgbClr val="0EA5E9"/></a:accent2>
      <a:accent3><a:srgbClr val="8B5CF6"/></a:accent3>
      <a:accent4><a:srgbClr val="10B981"/></a:accent4>
      <a:accent5><a:srgbClr val="F59E0B"/></a:accent5>
      <a:accent6><a:srgbClr val="EF4444"/></a:accent6>
      <a:hlink><a:srgbClr val="2563EB"/></a:hlink>
      <a:folHlink><a:srgbClr val="7C3AED"/></a:folHlink>
    </a:clrScheme>
    <a:fontScheme name="KnowNote">
      <a:majorFont>
        <a:latin typeface="Calibri"/>
        <a:ea typeface="Hiragino Sans GB"/>
        <a:cs typeface="Calibri"/>
      </a:majorFont>
      <a:minorFont>
        <a:latin typeface="Calibri"/>
        <a:ea typeface="Hiragino Sans GB"/>
        <a:cs typeface="Calibri"/>
      </a:minorFont>
    </a:fontScheme>
    <a:fmtScheme name="KnowNote">
      <a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>
      <a:lnStyleLst><a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="9525"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>
      <a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>
      <a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst>
    </a:fmtScheme>
  </a:themeElements>
</a:theme>`

	pptxSlideMaster = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:bg><p:bgRef idx="1001"><a:schemeClr val="lt1"/></p:bgRef></p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
  <p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
  <p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>
</p:sldMaster>`

	pptxSlideMasterRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`

	pptxSlideLayout = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">
  <p:cSld name="Blank">
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sldLayout>`

	pptxSlideLayoutRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`

	pptxNotesMaster = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notesMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
        <p:spPr><a:xfrm><a:off x="685800" y="4343400"/><a:ext cx="5486400" cy="4114800"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
        <p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="zh-CN"/></a:p></p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
  <p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
</p:notesMaster>`

	pptxNotesMasterRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`
)

// BuildPPTX 将 SlideDeck 构造为标准 16:9 PowerPoint (.pptx) 文件字节。
func BuildPPTX(deck model.SlideDeck, badge string) ([]byte, error) {
	title := strings.TrimSpace(deck.Title)
	if title == "" {
		title = "知页智能课件"
	}
	subtitle := strings.TrimSpace(deck.Subtitle)
	if subtitle == "" {
		subtitle = "由知页 KnowNote 基于选定资料整理生成"
	}
	badgeText := strings.TrimSpace(badge)
	if badgeText == "" {
		badgeText = "KnowNote · 教学课件"
	}
	contentSlides := deck.Slides
	if len(contentSlides) == 0 {
		contentSlides = []model.Slide{{Title: "课程核心要点", Bullets: []string{"请补充课程资料以生成详细课件内容"}}}
	}
	totalSlides := 1 + len(contentSlides)

	var contentTypes strings.Builder
	contentTypes.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/notesMasters/notesMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml"/>
  <Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`)

	var presRels strings.Builder
	presRels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="notesMasters/notesMaster1.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>`)

	var sldIdLst strings.Builder
	entries := []zipEntry{
		{name: "_rels/.rels", data: pptxRootRels},
		{name: "ppt/theme/theme1.xml", data: pptxTheme},
		{name: "ppt/slideMasters/slideMaster1.xml", data: pptxSlideMaster},
		{name: "ppt/slideMasters/_rels/slideMaster1.xml.rels", data: pptxSlideMasterRels},
		{name: "ppt/slideLayouts/slideLayout1.xml", data: pptxSlideLayout},
		{name: "ppt/slideLayouts/_rels/slideLayout1.xml.rels", data: pptxSlideLayoutRels},
		{name: "ppt/notesMasters/notesMaster1.xml", data: pptxNotesMaster},
		{name: "ppt/notesMasters/_rels/notesMaster1.xml.rels", data: pptxNotesMasterRels},
	}

	addSlide := func(index int, slideXML, notesText string) {
		relID := fmt.Sprintf("rId%d", 3+index)
		fmt.Fprintf(&contentTypes, "\n  <Override PartName=\"/ppt/slides/slide%d.xml\" ContentType=\"application/vnd.openxmlformats-officedocument.presentationml.slide+xml\"/>", index)
		fmt.Fprintf(&contentTypes, "\n  <Override PartName=\"/ppt/notesSlides/notesSlide%d.xml\" ContentType=\"application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml\"/>", index)
		fmt.Fprintf(&presRels, "\n  <Relationship Id=\"%s\" Type=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide\" Target=\"slides/slide%d.xml\"/>", relID, index)
		fmt.Fprintf(&sldIdLst, `<p:sldId id="%d" r:id="%s"/>`, 255+index, relID)

		entries = append(entries,
			zipEntry{name: fmt.Sprintf("ppt/slides/slide%d.xml", index), data: slideXML},
			zipEntry{name: fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", index), data: fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide%d.xml"/>
</Relationships>`, index)},
			zipEntry{name: fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", index), data: buildNotesSlideXML(notesText)},
			zipEntry{name: fmt.Sprintf("ppt/notesSlides/_rels/notesSlide%d.xml.rels", index), data: fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster" Target="../notesMasters/notesMaster1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="../slides/slide%d.xml"/>
</Relationships>`, index)},
		)
	}

	coverNotes := fmt.Sprintf("课程封面：%s。本课件共 %d 页（含封面）。", title, totalSlides)
	addSlide(1, buildCoverSlideXML(title, subtitle, badgeText, totalSlides), coverNotes)
	for i, slide := range contentSlides {
		pageNum := i + 2
		addSlide(pageNum, buildContentSlideXML(slide, pageNum, totalSlides), slide.Notes)
	}

	contentTypes.WriteString("\n</Types>")
	presRels.WriteString("\n</Relationships>")
	presentationXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
  <p:notesMasterIdLst><p:notesMasterId r:id="rId2"/></p:notesMasterIdLst>
  <p:sldIdLst>%s</p:sldIdLst>
  <p:sldSz cx="12192000" cy="6858000"/>
  <p:notesSz cx="6858000" cy="9144000"/>
</p:presentation>`, sldIdLst.String())

	entries = append(entries,
		zipEntry{name: "[Content_Types].xml", data: contentTypes.String()},
		zipEntry{name: "ppt/_rels/presentation.xml.rels", data: presRels.String()},
		zipEntry{name: "ppt/presentation.xml", data: presentationXML},
	)
	return buildZip(entries)
}

func buildCoverSlideXML(title, subtitle, badge string, total int) string {
	footer := fmt.Sprintf("共 %d 页 · 知页 KnowNote 智能生成", total)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:bg><p:bgPr><a:solidFill><a:srgbClr val="F8FAFC"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      %s
      %s
      %s
      %s
      %s
      %s
    </p:spTree>
  </p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sld>`,
		pptRectShape(2, "AccentLeft", 0, 0, 340000, 6858000, "0F172A", ""),
		pptRectShape(3, "AccentStripe", 340000, 0, 80000, 6858000, "2563EB", ""),
		pptBadgeShape(4, "Badge", 960000, 1280000, 3600000, 440000, "DBEAFE", "1D4ED8", badge, 1300),
		pptTextShape(5, "Title", "ctrTitle", 960000, 2040000, 10200000, 1850000, pptRunParagraph(title, 3600, true, "0F172A", "l")),
		pptTextShape(6, "Subtitle", "subTitle", 960000, 4050000, 9800000, 1100000, pptRunParagraph(subtitle, 1800, false, "475569", "l")),
		pptTextShape(7, "Footer", "", 960000, 5980000, 8000000, 400000, pptRunParagraph(footer, 1200, false, "64748B", "l")),
	)
}

func buildContentSlideXML(slide model.Slide, pageNum, total int) string {
	titleText := strings.TrimSpace(slide.Title)
	if titleText == "" {
		titleText = fmt.Sprintf("第 %d 节核心内容", pageNum-1)
	}
	subText := strings.TrimSpace(slide.Subtitle)
	bodyY := 1620000
	bodyH := 4460000
	var extra string
	if subText != "" {
		extra = pptTextShape(10, "Subhead", "", 760000, 1120000, 10672000, 360000, pptRunParagraph(subText, 1400, true, "2563EB", "l"))
		bodyY = 1780000
		bodyH = 4300000
	}

	bullets := slide.Bullets
	if len(bullets) == 0 {
		bullets = []string{"暂无条目内容"}
	}
	fontSz := 1850
	if len(bullets) >= 6 || maxBulletRunes(bullets) > 55 {
		fontSz = 1550
	} else if len(bullets) == 5 {
		fontSz = 1700
	}

	var bodyParagraphs strings.Builder
	for _, b := range bullets {
		bodyParagraphs.WriteString(pptBulletParagraph(strings.TrimSpace(b), fontSz))
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      %s
      %s
      %s
      %s
      %s
      %s
      %s
      %s
    </p:spTree>
  </p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sld>`,
		pptRectShape(2, "TopBar", 0, 0, 12192000, 72000, "2563EB", ""),
		pptTextShape(3, "SlideTitle", "title", 760000, 380000, 10672000, 700000, pptRunParagraph(titleText, 2500, true, "0F172A", "l")),
		extra,
		pptRectShape(4, "Divider", 760000, bodyY-160000, 10672000, 14000, "E2E8F0", ""),
		pptRoundedCardShape(5, "Card", 760000, bodyY, 10672000, bodyH, "F8FAFC", "E2E8F0"),
		pptTextShape(6, "Body", "body", 1040000, bodyY+200000, 10112000, bodyH-400000, bodyParagraphs.String()),
		pptTextShape(7, "FooterLeft", "", 760000, 6280000, 6000000, 340000, pptRunParagraph("知页 KnowNote · 智能备课课件", 1100, false, "64748B", "l")),
		pptTextShape(8, "FooterRight", "", 9400000, 6280000, 2032000, 340000, pptRunParagraph(fmt.Sprintf("%d / %d", pageNum, total), 1100, true, "64748B", "r")),
	)
}

func buildNotesSlideXML(notes string) string {
	text := strings.TrimSpace(notes)
	if text == "" {
		text = "无教师讲稿备注。"
	}
	var paras strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		paras.WriteString(pptRunParagraph(strings.TrimSpace(line), 1200, false, "1E293B", "l"))
	}
	if paras.Len() == 0 {
		paras.WriteString(pptRunParagraph("无教师讲稿备注。", 1200, false, "1E293B", "l"))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Notes"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
        <p:spPr><a:xfrm><a:off x="685800" y="4343400"/><a:ext cx="5486400" cy="4114800"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>
        <p:txBody><a:bodyPr wrap="square"/><a:lstStyle/>%s</p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:notes>`, paras.String())
}

func pptRectShape(id int, name string, x, y, cx, cy int, fillHex, strokeHex string) string {
	return pptGeomShape(id, name, "rect", x, y, cx, cy, fillHex, strokeHex)
}

func pptRoundedCardShape(id int, name string, x, y, cx, cy int, fillHex, strokeHex string) string {
	return pptGeomShape(id, name, "roundRect", x, y, cx, cy, fillHex, strokeHex)
}

func pptGeomShape(id int, name, geom string, x, y, cx, cy int, fillHex, strokeHex string) string {
	ln := `<a:ln><a:noFill/></a:ln>`
	if strokeHex != "" {
		ln = fmt.Sprintf(`<a:ln w="12700"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:ln>`, strokeHex)
	}
	return fmt.Sprintf(`<p:sp>
  <p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
  <p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="%s"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="%s"/></a:solidFill>%s</p:spPr>
  <p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="zh-CN"/></a:p></p:txBody>
</p:sp>`, id, escapeXML(name), x, y, cx, cy, geom, fillHex, ln)
}

func pptBadgeShape(id int, name string, x, y, cx, cy int, fillHex, textHex, text string, sz int) string {
	return fmt.Sprintf(`<p:sp>
  <p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
  <p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="roundRect"><a:avLst><a:gd name="adj" fmla="val 40000"/></a:avLst></a:prstGeom><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:ln><a:noFill/></a:ln></p:spPr>
  <p:txBody><a:bodyPr wrap="none" anchor="ctr" lIns="140000" rIns="140000" tIns="40000" bIns="40000"/><a:lstStyle/>%s</p:txBody>
</p:sp>`, id, escapeXML(name), x, y, cx, cy, fillHex, pptRunParagraph(text, sz, true, textHex, "ctr"))
}

func pptTextShape(id int, name, phType string, x, y, cx, cy int, paragraphs string) string {
	nv := `<p:nvPr/>`
	if phType != "" {
		nv = fmt.Sprintf(`<p:nvPr><p:ph type="%s"/></p:nvPr>`, phType)
	}
	return fmt.Sprintf(`<p:sp>
  <p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr txBox="1"/><p:nvPr>%s</p:nvPr></p:nvSpPr>
  <p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/></p:spPr>
  <p:txBody><a:bodyPr wrap="square" lIns="0" rIns="0" tIns="0" bIns="0"/><a:lstStyle/>%s</p:txBody>
</p:sp>`, id, escapeXML(name), nv, x, y, cx, cy, paragraphs)
}

func pptRunParagraph(text string, sz int, bold bool, colorHex, align string) string {
	bAttr := "0"
	if bold {
		bAttr = "1"
	}
	return fmt.Sprintf(`<a:p><a:pPr algn="%s"/><a:r><a:rPr lang="zh-CN" sz="%d" b="%s" dirty="0"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:latin typeface="Calibri"/><a:ea typeface="Hiragino Sans GB"/><a:cs typeface="Calibri"/></a:rPr><a:t>%s</a:t></a:r></a:p>`,
		align, sz, bAttr, colorHex, escapeXML(text))
}

func pptBulletParagraph(text string, sz int) string {
	runs := parseInlineSpans(text)
	if len(runs) == 0 {
		runs = []inlineRun{{text: text}}
	}
	var rb strings.Builder
	for _, r := range runs {
		bAttr := "0"
		color := "1E293B"
		if r.bold {
			bAttr = "1"
		}
		if r.citation {
			color = "2563EB"
		}
		fmt.Fprintf(&rb, `<a:r><a:rPr lang="zh-CN" sz="%d" b="%s" dirty="0"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:latin typeface="Calibri"/><a:ea typeface="Hiragino Sans GB"/><a:cs typeface="Calibri"/></a:rPr><a:t>%s</a:t></a:r>`,
			sz, bAttr, color, escapeXML(r.text))
	}
	return fmt.Sprintf(`<a:p><a:pPr lvl="0" marL="320000" indent="-240000" algn="l"><a:lnSpc><a:spcPct val="135000"/></a:lnSpc><a:spcAft><a:spcPts val="1200"/></a:spcAft><a:buColor><a:srgbClr val="2563EB"/></a:buColor><a:buFont typeface="Arial"/><a:buChar char="•"/></a:pPr>%s</a:p>`,
		rb.String())
}

func maxBulletRunes(bullets []string) int {
	maxLen := 0
	for _, b := range bullets {
		if n := utf8.RuneCountInString(b); n > maxLen {
			maxLen = n
		}
	}
	return maxLen
}
