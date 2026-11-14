package parser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const Placeholder = "（暂不支持自动转写，可粘贴字幕文本作为来源）"
const MaxTextSize int64 = 20 << 20

type Input struct {
	Type, URL, Text string
	OriginalExt     string
	File            *os.File
}
type Result struct {
	Title, Content string
	Placeholder    bool
	PageCount      int
	Sections       []Section
	Warning        string
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
	case "pdf", "docx", "pptx", "xlsx", "csv":
		if input.File == nil {
			return Result{}, fmt.Errorf("找不到上传文件")
		}
		file := input.File
		if input.OriginalExt == "doc" || input.OriginalExt == "ppt" || input.OriginalExt == "xls" {
			converted, cleanup, convertErr := Convert(ctx, file.Name(), input.Type)
			if convertErr != nil {
				return Result{}, convertErr
			}
			defer cleanup()
			file, err = os.Open(converted)
			if err != nil {
				return Result{}, fmt.Errorf("读取转换文件失败")
			}
			defer file.Close()
		}
		switch input.Type {
		case "pdf":
			result, err = parsePDF(ctx, file)
		case "docx":
			result, err = parseDOCX(ctx, file)
		case "pptx":
			result, err = parsePPTX(ctx, file)
		case "xlsx":
			result, err = parseXLSX(ctx, file)
		case "csv":
			result, err = parseCSV(ctx, file)
		}
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
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		line = strings.Join(strings.Fields(line), " ")
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "1. ") {
			line = strings.Repeat(" ", min(indent, 16)) + line
		}
		if line == "" && (len(lines) == 0 || lines[len(lines)-1] == "") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}
