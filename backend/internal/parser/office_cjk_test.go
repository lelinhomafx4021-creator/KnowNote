package parser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 回归测试：LibreOffice 转出的 PDF 必须保留中文（之前无界面运行时找不到系统字体，中文全部空白），
// 并且 PDF 解析能把中文按页提取出来。
func TestOfficeConvertKeepsChinese(t *testing.T) {
	if _, err := SofficePath(); err != nil {
		t.Skip("未安装 LibreOffice")
	}
	if len(CJKFontFiles()) == 0 {
		t.Skip("本机没有可用的中文字体")
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "中文报告.html")
	html := `<html><head><meta charset="utf-8"></head><body><h1>检索增强生成</h1><p>系统通过向量相似度召回相关片段，并标注引用来源。</p>` +
		`<p style="page-break-before:always">第二页：阿里云存储与原件预览。</p></body></html>`
	if err := os.WriteFile(input, []byte(html), 0o600); err != nil {
		t.Fatal(err)
	}
	output, cleanup, err := Convert(context.Background(), input, "pdf")
	if err != nil {
		t.Fatalf("转换失败：%v", err)
	}
	defer cleanup()
	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	result, err := parsePDF(context.Background(), file)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if result.PageCount != 2 {
		t.Fatalf("页数 = %d，期望 2", result.PageCount)
	}
	for _, want := range []string{"检索增强生成", "向量相似度", "阿里云存储"} {
		if !strings.Contains(result.Content, want) {
			t.Fatalf("PDF 正文缺少中文「%s」：%q", want, result.Content)
		}
	}
}
