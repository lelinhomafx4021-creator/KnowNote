package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func inputFile(t *testing.T, data []byte) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "来源-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(data); err != nil {
		t.Fatal(err)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	return file
}

func TestTextAndDOCX(t *testing.T) {
	p := New()
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文资料\n第二段"))
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{[]byte("\xef\xbb\xbf中文资料\n第二段"), gbk} {
		result, err := p.Parse(context.Background(), Input{Type: "text", File: inputFile(t, data)})
		if err != nil || result.Content != "中文资料\n第二段" {
			t.Fatalf("解码失败：%v", err)
		}
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, _ := writer.Create("word/document.xml")
	io.WriteString(entry, `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>第一</w:t></w:r><w:r><w:t>段</w:t></w:r></w:p><w:p><w:r><w:t>第二段</w:t><w:tab/><w:t>内容</w:t></w:r></w:p></w:body></w:document>`)
	writer.Close()
	result, err := p.Parse(context.Background(), Input{Type: "docx", File: inputFile(t, archive.Bytes())})
	if err != nil || Clean(result.Content) != "第一段\n\n第二段 内容" {
		t.Fatalf("DOCX 段落解析失败：%v %q", err, result.Content)
	}
	if got := Clean("\ufeff  中文\x00\t  内容\r\n\n\n下一段  \x1b"); got != "中文 内容\n\n下一段" {
		t.Fatalf("清洗结果不正确：%q", got)
	}
	if _, err := p.Parse(context.Background(), Input{Type: "docx", File: inputFile(t, []byte("损坏"))}); err == nil {
		t.Fatal("损坏 DOCX 应报错")
	}
}

// 直接生成含 Helvetica 文字与空页的最小 PDF，不依赖外部生成工具。
func pdfFixture(text string) []byte {
	stream := "BT /F1 12 Tf 72 720 Td (" + text + ") Tj ET"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << >> >>",
	}
	var buffer bytes.Buffer
	buffer.WriteString("%PDF-1.4\n")
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

func TestPDF(t *testing.T) {
	p := New()
	result, err := p.Parse(context.Background(), Input{Type: "pdf", File: inputFile(t, pdfFixture("KnowNote PDF text extraction"))})
	if err != nil || !strings.Contains(result.Content, "KnowNote") {
		t.Fatalf("PDF 提取失败：%v %q", err, result.Content)
	}
	_, err = p.Parse(context.Background(), Input{Type: "pdf", File: inputFile(t, pdfFixture(""))})
	if err == nil || err.Error() != "未能从 PDF 中提取文字，可能是扫描件" {
		t.Fatalf("扫描件错误不符合要求：%v", err)
	}
	if _, err = p.Parse(context.Background(), Input{Type: "pdf", File: inputFile(t, []byte("损坏"))}); err == nil {
		t.Fatal("损坏 PDF 应报错")
	}
}

func TestPlaceholderAndURLs(t *testing.T) {
	p := New()
	for _, input := range []Input{{Type: "audio"}, {Type: "youtube", URL: "https://youtu.be/123"}, {Type: "web", URL: "https://www.bilibili.com/video/123"}} {
		result, err := p.Parse(context.Background(), input)
		if err != nil || !result.Placeholder || result.Content != Placeholder {
			t.Fatal("音视频应返回占位正文")
		}
	}
	for _, url := range []string{"file:///etc/passwd", "http://127.0.0.1", "http://[::1]", "http://10.0.0.1", "http://localhost", "https://user:pass@example.com", "http://100.64.0.1"} {
		if ValidateURL(url) == nil {
			t.Fatalf("应该拒绝地址 %s", url)
		}
	}
	for _, ip := range []string{"127.0.0.1", "10.1.1.1", "169.254.169.254", "::1", "fc00::1", "0.0.0.0"} {
		if publicIP(net.ParseIP(ip)) {
			t.Fatal("内网地址未拦截")
		}
	}
	if err := ValidateURL("https://example.com"); err != nil {
		t.Fatal(err)
	}
	if IsVideoURL("https://youtube.com.evil.example/video") {
		t.Fatal("视频域名后缀检查错误")
	}
}

func TestWeb(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.UserAgent(), "Mozilla/") {
			t.Error("缺少浏览器 UA")
		}
		switch r.URL.Path {
		case "/large":
			w.Header().Set("Content-Length", fmt.Sprint(MaxWebSize+1))
		case "/unknown-large":
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			io.WriteString(w, strings.Repeat("a", int(MaxWebSize+1)))
		case "/error":
			w.WriteHeader(503)
		case "/slow":
			<-r.Context().Done()
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, "<html><head><title>网页标题</title></head><body><article><p>"+strings.Repeat("这是用于检索的网页正文。", 100)+"</p></article></body></html>")
		}
	}))
	defer server.Close()
	p := New()
	// 测试通过自定义传输连接本地服务器，生产客户端仍保持公网地址检查。
	u := server.Listener.Addr().String()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, u)
	}
	p.client.Transport = transport
	defer transport.CloseIdleConnections()
	result, err := p.Parse(context.Background(), Input{Type: "web", URL: "http://example.com/"})
	if err != nil || result.Title != "网页标题" || !strings.Contains(result.Content, "网页正文") {
		t.Fatalf("网页提取失败：%v", err)
	}
	for _, path := range []string{"/large", "/unknown-large", "/error"} {
		if _, err := p.Parse(context.Background(), Input{Type: "web", URL: "http://example.com" + path}); err == nil {
			t.Fatal("应拒绝过大或异常响应")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := p.Parse(ctx, Input{Type: "web", URL: "http://example.com/slow"}); err == nil {
		t.Fatal("取消后应停止网页抓取")
	}
}
