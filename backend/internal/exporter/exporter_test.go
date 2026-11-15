package exporter

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"knownote/internal/model"
	"knownote/internal/parser"
)

const sampleMarkdown = `# 操作系统核心复习

> 本讲义根据课程资料自动梳理，聚焦进程调度与内存管理。

## 进程调度算法
- **先来先服务 (FCFS)**：按就绪队列顺序执行，可能产生护航效应 [1]。
- **时间片轮转 (RR)**：兼顾响应时间与公平性，适合分时系统。
1. 短作业优先可降低平均周转时间
2. 多级反馈队列结合响应比与优先级

| 算法 | 适用场景 | 核心权衡 |
|---|---|---|
| FCFS | 批处理 | 简单但等待波动大 |
| RR | 交互系统 | 上下文切换开销 |

### 注意事项
请重点关注` + " `量子时间片` " + `大小的选取原则。
`

func parseBytes(t *testing.T, kind, ext string, data []byte) parser.Result {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out."+ext)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	parsed, err := (&parser.Parser{}).Parse(context.Background(), parser.Input{Type: kind, OriginalExt: ext, File: f})
	if err != nil {
		t.Fatalf("生成的 %s 无法被解析器读取: %v", ext, err)
	}
	return parsed
}

func TestBuildDOCXAndRoundTripParse(t *testing.T) {
	data, err := BuildDOCX("操作系统核心复习", "RAG 定向检索主题：进程调度", sampleMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parseBytes(t, "docx", "docx", data)
	for _, want := range []string{"进程调度算法", "先来先服务", "时间片轮转", "FCFS", "多级反馈队列"} {
		if !strings.Contains(parsed.Content, want) {
			t.Fatalf("解析回读缺少关键词 %q，原文：\n%s", want, parsed.Content)
		}
	}
}

func TestBuildPPTXAndRoundTripParse(t *testing.T) {
	deck := model.SlideDeck{
		Title:    "计算机网络：TCP 可靠传输",
		Subtitle: "课堂教学课件 · 第4章精讲",
		Slides: []model.Slide{
			{
				Title:    "课程学习目标",
				Subtitle: "本科生计算机网络第4章",
				Bullets:  []string{"理解 **停止等待** 与流水线传输机制", "掌握滑动窗口与累计确认设计", "区分流量控制与拥塞控制目标"},
				Notes:    "开场提问学生如何保障不可靠信道传输可靠性，引出 ARQ 机制。",
			},
			{
				Title:    "拥塞控制四阶段",
				Subtitle: "慢开始 · 拥塞避免 · 快重传 · 快恢复",
				Bullets:  []string{"慢开始阶段 cwnd 呈指数增长至 ssthresh", "拥塞避免改为每 RTT 线性加 1", "连续 3 个重复 ACK 触发快重传与快恢复"},
				Notes:    "结合板书画出 cwnd 锯齿状变化曲线，对比 Tahoe 与 Reno 差异。",
			},
		},
	}
	data, err := BuildPPTX(deck, "KnowNote · 课堂课件")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parseBytes(t, "pptx", "pptx", data)
	if parsed.PageCount != 3 {
		t.Fatalf("期望 3 页幻灯片（1 封面 + 2 内容），实际 %d 页", parsed.PageCount)
	}
	for _, want := range []string{"TCP 可靠传输", "课程学习目标", "拥塞控制四阶段", "开场提问", "慢开始阶段"} {
		if !strings.Contains(parsed.Content, want) {
			t.Fatalf("PPTX 解析回读缺少 %q，原文：\n%s", want, parsed.Content)
		}
	}
}

func TestExportArtifactToPDFWithLibreOffice(t *testing.T) {
	if _, err := parser.SofficePath(); err != nil {
		t.Skip("本机未安装 LibreOffice，跳过 PDF 真实转换测试")
	}
	art := model.Artifact{
		Kind:          "pdf",
		Title:         "PDF 讲义 · 进程调度",
		Format:        "pdf",
		Prompt:        "进程调度算法对比",
		RetrievalMode: "rag",
		Content:       sampleMarkdown,
	}
	pdfBytes, filename, mimeType, err := ExportArtifact(context.Background(), art, "pdf")
	if err != nil {
		t.Fatalf("导出 PDF 失败: %v", err)
	}
	if mimeType != mimePDF || !strings.HasSuffix(filename, ".pdf") {
		t.Fatalf("导出元数据异常: %s / %s", mimeType, filename)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("输出不是有效 PDF 头")
	}
	parsed := parseBytes(t, "pdf", "pdf", pdfBytes)
	if !strings.Contains(parsed.Content, "进程调度") || !strings.Contains(parsed.Content, "时间片轮转") {
		t.Fatalf("导出 PDF 中文文本渲染异常或缺失，解析内容：\n%s", parsed.Content)
	}
}

func TestMarkdownAndMindmapCrossExport(t *testing.T) {
	mm := &model.MindMapNode{
		ID: "root", Label: "AI 个人知识库",
		Children: []model.MindMapNode{
			{ID: "c1", Label: "检索", Children: []model.MindMapNode{{ID: "c11", Label: "向量相似度"}}},
		},
	}
	md := MindmapToMarkdown(mm)
	if !strings.Contains(md, "# AI 个人知识库") || !strings.Contains(md, "- **向量相似度**") {
		t.Fatalf("思维导图转 Markdown 异常:\n%s", md)
	}
	deck := MarkdownToSlideDeck("复习大纲", sampleMarkdown)
	if len(deck.Slides) < 2 {
		t.Fatalf("Markdown 转幻灯片页数不足: %d", len(deck.Slides))
	}
}
