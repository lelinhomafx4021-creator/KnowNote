package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	oldpdf "github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
	"knownote/internal/testfixture"
)

func TestGeneratedDocuments(t *testing.T) {
	xlsx, err := testfixture.XLSX()
	if err != nil {
		t.Fatal(err)
	}
	fixtures := map[string][]byte{"pdf": testfixture.PDF(), "docx": testfixture.DOCX(), "pptx": testfixture.PPTX(), "xlsx": xlsx, "csv": testfixture.CSV()}
	for kind, data := range fixtures {
		t.Run(kind, func(t *testing.T) {
			if dir := os.Getenv("T7_FIXTURE_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "sample."+kind), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := New().Parse(context.Background(), Input{Type: kind, File: inputFile(t, data)})
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "pdf":
				if result.PageCount != 3 || len(result.Sections) != 3 {
					t.Fatalf("PDF 页数错误：%+v", result)
				}
				for i, section := range result.Sections {
					if section.Page != i+1 || section.Label != fmt.Sprintf("第 %d 页", i+1) || !strings.Contains(section.Content, fmt.Sprintf("第%d页", i+1)) || !strings.Contains(section.Content, "知识管理") || !strings.Contains(section.Content, "document") || strings.Contains(section.Content, "页眉") || strings.Contains(section.Content, "页脚") {
						t.Fatalf("PDF 页码、清洗或中英文提取失败：%+v", section)
					}
				}
				// 对比旧实现的逐页输出，记录字符和断行差异，作为选型证据。
				file := inputFile(t, data)
				info, _ := file.Stat()
				reader, err := oldpdf.NewReader(file, info.Size())
				if err != nil {
					t.Fatal(err)
				}
				old, err := reader.Page(1).GetPlainText(nil)
				t.Logf("旧解析器首面=%q，错误=%v；PDFium 首面=%q", Clean(old), err, result.Sections[0].Content)
			case "pptx":
				if result.PageCount != 2 || len(result.Sections) != 2 || result.Sections[0].Title != "第一张：知识管理" || result.Sections[1].Title != "第二张：学习计划" {
					t.Fatalf("幻灯片顺序/标题错误：%+v", result.Sections)
				}
				for _, part := range []string{"- 资料检索", "  - 带引用问答", "| 功能 | 价值 |", "| 引用 | 可追溯 |", "备注：\n演讲者提醒"} {
					if !strings.Contains(result.Content, part) {
						t.Fatalf("PPTX 缺少 %s：%s", part, result.Content)
					}
				}
			case "docx":
				if len(result.Sections) != 2 || result.Sections[0].Title != "知识管理" || result.Sections[1].Title != "学习计划" {
					t.Fatalf("DOCX 分节错误：%+v", result)
				}
				for _, part := range []string{"# 知识管理", "| 功能 | 说明 |", "| 检索 | 向量引用 |", "  - 整理资料"} {
					if !strings.Contains(result.Content, part) {
						t.Fatalf("DOCX 缺少 %s：%s", part, result.Content)
					}
				}
			case "xlsx":
				if result.PageCount != 2 || len(result.Sections) != 2 || !strings.Contains(result.Content, "姓名：小知；分数：95") || result.Sections[1].Label != "工作表：计划" {
					t.Fatalf("XLSX 错误：%+v", result)
				}
			case "csv":
				if result.PageCount != 1 || !strings.Contains(result.Content, "姓名：小知；任务：资料检索") {
					t.Fatalf("GBK CSV 错误：%+v", result)
				}
			}
		})
	}
}
func TestSheetLimitsAndCSVQuoting(t *testing.T) {
	book := excelize.NewFile()
	defer book.Close()
	book.SetCellValue("Sheet1", "A1", "编号")
	for i := 2; i <= MaxSheetRows+3; i++ {
		book.SetCellValue("Sheet1", fmt.Sprintf("A%d", i), i-1)
	}
	data, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	result, err := New().Parse(context.Background(), Input{Type: "xlsx", File: inputFile(t, data.Bytes())})
	if err != nil || !strings.Contains(result.Warning, "前 2000 行") || strings.Contains(result.Content, "编号：2001") {
		t.Fatalf("XLSX 截断失败：%v", err)
	}
	for _, separator := range []string{",", "\t", ";", "|"} {
		text := "姓名" + separator + "说明\n小知" + separator + "\"含,逗号\n第二行\"\n"
		result, err := New().Parse(context.Background(), Input{Type: "csv", File: inputFile(t, []byte(text))})
		if err != nil || !strings.Contains(result.Content, "说明：含,逗号\n第二行") {
			t.Fatalf("CSV 分隔符/引号失败 %q：%v %s", separator, err, result.Content)
		}
	}
}
func TestLibreOffice(t *testing.T) {
	if _, err := SofficePath(); err != nil {
		t.Skip("未找到 soffice，跳过实际转换")
	}
	dir := t.TempDir()
	xlsx, err := testfixture.XLSX()
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		kind, legacy string
		data         []byte
	}{{"docx", "doc", testfixture.DOCX()}, {"pptx", "ppt", testfixture.PPTX()}, {"xlsx", "xls", xlsx}} {
		t.Run(sample.legacy, func(t *testing.T) {
			file := filepath.Join(dir, "sample."+sample.kind)
			if err := os.WriteFile(file, sample.data, 0600); err != nil {
				t.Fatal(err)
			}
			converted, cleanup, err := Convert(context.Background(), file, sample.legacy)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			legacy, err := os.Open(converted)
			if err != nil {
				t.Fatal(err)
			}
			defer legacy.Close()
			result, err := New().Parse(context.Background(), Input{Type: sample.kind, OriginalExt: sample.legacy, File: legacy})
			if err != nil || !strings.Contains(result.Content, "知识管理") && sample.kind != "xlsx" {
				t.Fatalf("旧版转换失败：%v %s", err, result.Content)
			}
			if export := os.Getenv("T7_FIXTURE_DIR"); export != "" {
				data, err := os.ReadFile(converted)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(export, "sample."+sample.legacy), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if sample.kind != "xlsx" {
				preview, clean, err := Convert(context.Background(), file, "pdf")
				if err != nil {
					t.Fatal(err)
				}
				defer clean()
				data, err := os.ReadFile(preview)
				if err != nil || !strings.HasPrefix(string(data), "%PDF-") {
					t.Fatal("预览 PDF 无效")
				}
			}
		})
	}
}
func TestMissingOffice(t *testing.T) {
	t.Setenv("SOFFICE_PATH", filepath.Join(t.TempDir(), "missing"))
	_, err := New().Parse(context.Background(), Input{Type: "docx", OriginalExt: "doc", File: inputFile(t, []byte("旧格式"))})
	if err == nil || err.Error() != "需要安装 LibreOffice 才能解析旧版 Office 文件" {
		t.Fatalf("旧格式错误不符：%v", err)
	}
	if _, err := New().Parse(context.Background(), Input{Type: "docx", File: inputFile(t, testfixture.DOCX())}); err != nil {
		t.Fatal("新版格式不应依赖 LibreOffice")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	<-ctx.Done()
	if _, _, err := Convert(ctx, "sample.doc", "docx"); err == nil {
		t.Fatal("取消转换应报错")
	}
}
