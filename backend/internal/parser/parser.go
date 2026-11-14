package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const Placeholder = "（暂不支持自动转写，可粘贴字幕文本作为来源）"
const MaxTextSize int64 = 20 << 20

type Input struct {
	Type, URL, Text string
	File            *os.File
}
type Result struct {
	Title, Content string
	Placeholder    bool
}

func (p *Parser) Parse(ctx context.Context, input Input) (result Result, err error) {
	defer func() {
		if recover() != nil {
			result = Result{}
			err = fmt.Errorf("文件格式损坏，无法解析")
		}
	}()
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if input.Type == "audio" || input.Type == "youtube" || (input.Type == "web" && IsVideoURL(input.URL)) {
		return Result{Content: Placeholder, Placeholder: true}, nil
	}
	switch input.Type {
	case "web":
		return p.web(ctx, input.URL)
	case "text", "markdown":
		if input.File == nil {
			result.Content = input.Text
			break
		}
		data, err := readLimited(input.File, MaxTextSize)
		if err != nil {
			return Result{}, err
		}
		result.Content, err = decodeText(data)
		if err != nil {
			return Result{}, err
		}
	case "pdf":
		if input.File == nil {
			return Result{}, fmt.Errorf("找不到上传文件")
		}
		result.Content, err = parsePDF(ctx, input.File)
	case "docx":
		if input.File == nil {
			return Result{}, fmt.Errorf("找不到上传文件")
		}
		result.Content, err = parseDOCX(ctx, input.File)
	default:
		return Result{}, fmt.Errorf("暂不支持此来源类型")
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("读取正文失败")
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("正文过大，无法处理")
	}
	return data, nil
}

func decodeText(data []byte) (string, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if utf8.Valid(data) {
		return string(data), nil
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(data)
	if err != nil || bytes.Contains(decoded, []byte("�")) {
		return "", fmt.Errorf("文本编码无法识别，请使用 UTF-8 或 GBK")
	}
	return string(decoded), nil
}

func parsePDF(ctx context.Context, file *os.File) (string, error) {
	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("读取 PDF 失败")
	}
	reader, err := pdf.NewReader(file, stat.Size())
	if err != nil {
		return "", fmt.Errorf("PDF 格式无效或已加密，无法读取")
	}
	var output strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("PDF 页面文字提取失败")
		}
		// 损坏字体映射常产生替代符；空页继续解析后续页面。
		text = strings.Map(func(r rune) rune {
			if r == utf8.RuneError {
				return -1
			}
			return r
		}, text)
		output.WriteString(text)
		output.WriteString("\n\n")
		if int64(output.Len()) > MaxTextSize {
			return "", fmt.Errorf("PDF 正文过大，无法处理")
		}
	}
	text := Clean(output.String())
	if !strings.ContainsFunc(text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return "", fmt.Errorf("未能从 PDF 中提取文字，可能是扫描件")
	}
	return text, nil
}

func parseDOCX(ctx context.Context, file *os.File) (string, error) {
	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("读取 DOCX 失败")
	}
	archive, err := zip.NewReader(file, stat.Size())
	if err != nil {
		return "", fmt.Errorf("DOCX 格式无效")
	}
	for _, entry := range archive.File {
		if entry.Name != "word/document.xml" {
			continue
		}
		if entry.UncompressedSize64 > uint64(MaxTextSize) {
			return "", fmt.Errorf("DOCX 正文过大，无法处理")
		}
		stream, err := entry.Open()
		if err != nil {
			return "", fmt.Errorf("读取 DOCX 正文失败")
		}
		defer stream.Close()
		data, err := readLimited(stream, MaxTextSize)
		if err != nil {
			return "", err
		}
		decoder := xml.NewDecoder(bytes.NewReader(data))
		var output strings.Builder
		for {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			token, err := decoder.Token()
			if err == io.EOF {
				return output.String(), nil
			}
			if err != nil {
				return "", fmt.Errorf("DOCX 正文 XML 损坏")
			}
			switch value := token.(type) {
			case xml.StartElement:
				switch value.Name.Local {
				case "t":
					var text string
					if err := decoder.DecodeElement(&text, &value); err != nil {
						return "", fmt.Errorf("DOCX 段落解析失败")
					}
					output.WriteString(text)
				case "br", "cr":
					output.WriteByte('\n')
				case "tab":
					output.WriteByte(' ')
				}
			case xml.EndElement:
				if value.Name.Local == "p" {
					output.WriteString("\n\n")
				}
			}
		}
	}
	return "", fmt.Errorf("DOCX 缺少正文文件")
}

// Clean 保留段落边界，合并行内空白，删除控制字符和无效字符。
func Clean(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	text = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, text)
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" && (len(lines) == 0 || lines[len(lines)-1] == "") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
