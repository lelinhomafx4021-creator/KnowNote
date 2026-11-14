package parser

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
)

const MaxSheetRows = 2000

func sheetText(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	headers := rows[0]
	var out []string
	for _, row := range rows[1:] {
		var cells []string
		for i, value := range row {
			if strings.TrimSpace(value) == "" {
				continue
			}
			name := fmt.Sprintf("列%d", i+1)
			if i < len(headers) && strings.TrimSpace(headers[i]) != "" {
				name = headers[i]
			}
			cells = append(cells, name+"："+value)
		}
		if len(cells) > 0 {
			out = append(out, strings.Join(cells, "；"))
		}
	}
	if len(out) == 0 {
		return strings.Join(headers, "；")
	}
	return strings.Join(out, "\n")
}
func parseXLSX(ctx context.Context, file *os.File) (Result, error) {
	book, err := excelize.OpenReader(file, excelize.Options{UnzipSizeLimit: 100 << 20, UnzipXMLSizeLimit: 20 << 20})
	if err != nil {
		return Result{}, fmt.Errorf("XLSX 格式无效")
	}
	defer book.Close()
	var sections []Section
	var warnings []string
	total := 0
	for _, name := range book.GetSheetList() {
		rows, err := book.Rows(name)
		if err != nil {
			return Result{}, fmt.Errorf("读取工作表失败")
		}
		var data [][]string
		truncated := false
		for rows.Next() {
			if err := ctx.Err(); err != nil {
				rows.Close()
				return Result{}, err
			}
			if len(data) >= MaxSheetRows+1 {
				truncated = true
				break
			}
			cols, err := rows.Columns()
			if err != nil {
				rows.Close()
				return Result{}, fmt.Errorf("读取表格行失败")
			}
			for _, v := range cols {
				total += len(v)
			}
			if int64(total) > MaxTextSize {
				rows.Close()
				return Result{}, fmt.Errorf("表格正文过大")
			}
			data = append(data, cols)
		}
		rowErr := rows.Error()
		rows.Close()
		if rowErr != nil {
			return Result{}, fmt.Errorf("读取工作表失败")
		}
		if truncated {
			warnings = append(warnings, "工作表“"+name+"”仅解析前 2000 行数据")
		}
		sections = append(sections, Section{Page: len(sections) + 1, Label: "工作表：" + name, Title: name, Content: sheetText(data)})
	}
	result := sectionResult(sections, len(sections))
	result.Warning = strings.Join(warnings, "；")
	return result, nil
}
func parseCSV(ctx context.Context, file *os.File) (Result, error) {
	data, err := readLimited(file, MaxTextSize)
	if err != nil {
		return Result{}, err
	}
	text, err := decodeText(data)
	if err != nil {
		return Result{}, err
	}
	// 选择能稳定形成最多列的分隔符；csv.Reader 正确处理引号内的分隔符和换行。
	delimiter := ','
	best := 0
	for _, candidate := range []rune{',', '\t', ';', '|'} {
		r := csv.NewReader(strings.NewReader(text))
		r.Comma = candidate
		r.FieldsPerRecord = -1
		score := 0
		width := 0
		valid := true
		for i := 0; i < 10; i++ {
			row, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				valid = false
				break
			}
			if i == 0 {
				width = len(row)
			}
			if len(row) == width {
				score += max(0, width-1)
			}
		}
		if valid && score > best {
			delimiter = candidate
			best = score
		}
	}
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	var rows [][]string
	truncated := false
	for {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Result{}, fmt.Errorf("CSV 格式无效，请检查引号或分隔符")
		}
		if len(rows) >= MaxSheetRows+1 {
			truncated = true
			break
		}
		rows = append(rows, row)
	}
	result := sectionResult([]Section{{Page: 1, Title: "CSV", Label: "工作表：CSV", Content: sheetText(rows)}}, 1)
	if truncated {
		result.Warning = "CSV 仅解析前 2000 行数据"
	}
	return result, nil
}
