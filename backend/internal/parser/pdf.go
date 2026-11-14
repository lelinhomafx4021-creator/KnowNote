package parser

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"unicode"

	pdfium "github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

var pdfOnce sync.Once
var pdfPool pdfium.Pool
var pdfInitErr error
var hyphenBreak = regexp.MustCompile(`([A-Za-z])-\n([a-z])`)

func parsePDF(ctx context.Context, file *os.File) (Result, error) {
	pdfOnce.Do(func() {
		pdfPool, pdfInitErr = webassembly.Init(webassembly.Config{MaxTotal: 2, MaxIdle: 0,
			FSConfig: wazero.NewFSConfig(), Stdout: io.Discard, Stderr: io.Discard,
			RuntimeConfig: wazero.NewRuntimeConfig().WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling).WithCloseOnContextDone(true),
		})
	})
	if pdfInitErr != nil {
		return Result{}, fmt.Errorf("PDF 解析引擎初始化失败")
	}
	instance, err := pdfPool.GetInstanceWithContext(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("PDF 解析已取消或繁忙")
	}
	defer instance.Close()
	// Kill 能中断损坏文件引发的长时间 WASM 运算。
	stopped := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
			_ = instance.Kill()
		case <-stopped:
		}
	}()
	defer func() { close(stopped); <-done }()
	stat, err := file.Stat()
	if err != nil {
		return Result{}, fmt.Errorf("读取 PDF 失败")
	}
	doc, err := instance.OpenDocument(&requests.OpenDocument{FileReader: file, FileReaderSize: stat.Size()})
	if err != nil {
		return Result{}, fmt.Errorf("PDF 格式无效或已加密，无法读取")
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
	count, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return Result{}, fmt.Errorf("读取 PDF 页数失败")
	}
	sections := make([]Section, 0, count.PageCount)
	size := 0
	for i := 0; i < count.PageCount; i++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		page, err := instance.GetPageText(&requests.GetPageText{Page: requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: i}}})
		if err != nil {
			return Result{}, fmt.Errorf("PDF 页面文字提取失败")
		}
		text := Clean(page.Text)
		size += len(text)
		if int64(size) > MaxTextSize {
			return Result{}, fmt.Errorf("PDF 正文过大，无法处理")
		}
		sections = append(sections, Section{Page: i + 1, Label: fmt.Sprintf("第 %d 页", i+1), Content: text})
	}
	cleanPDFSections(sections)
	result := sectionResult(sections, count.PageCount)
	if !strings.ContainsFunc(result.Content, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return Result{}, fmt.Errorf("未能从 PDF 中提取文字，可能是扫描件（本系统暂不支持 OCR）")
	}
	return result, nil
}

// 只删除多页边缘重复行；正文中的重复段落与空页位置仍保留。
func cleanPDFSections(sections []Section) {
	repeated := map[string]int{}
	for _, section := range sections {
		lines := strings.Split(section.Content, "\n")
		seen := map[string]bool{}
		for _, index := range []int{0, len(lines) - 1} {
			if line := lines[index]; line != "" && !seen[line] {
				repeated[line]++
				seen[line] = true
			}
		}
	}
	for i := range sections {
		lines := strings.Split(sections[i].Content, "\n")
		for _, index := range []int{0, len(lines) - 1} {
			if len(sections) >= 3 && repeated[lines[index]] >= max(3, (len(sections)+1)/2) {
				lines[index] = ""
			}
		}
		sections[i].Content = hyphenBreak.ReplaceAllString(strings.TrimSpace(strings.Join(lines, "\n")), "$1$2")
	}
}
