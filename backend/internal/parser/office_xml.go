package parser

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

type xmlNode struct {
	Name     string
	Attr     map[string]string
	Text     string
	Children []*xmlNode
}

func (n *xmlNode) child(name string) *xmlNode {
	if n != nil {
		for _, c := range n.Children {
			if c.Name == name {
				return c
			}
		}
	}
	return nil
}
func (n *xmlNode) all(name string) []*xmlNode {
	var out []*xmlNode
	if n != nil {
		for _, c := range n.Children {
			if c.Name == name {
				out = append(out, c)
			}
			out = append(out, c.all(name)...)
		}
	}
	return out
}
func (n *xmlNode) value(key string) string {
	if n == nil {
		return ""
	}
	return n.Attr[key]
}
func (n *xmlNode) text() string {
	if n == nil {
		return ""
	}
	if n.Name == "t" {
		return n.Text
	}
	if n.Name == "tab" {
		return " "
	}
	if n.Name == "br" || n.Name == "cr" {
		return "\n"
	}
	var out strings.Builder
	for _, c := range n.Children {
		out.WriteString(c.text())
	}
	return out.String()
}

type officeZIP struct {
	files map[string]*zip.File
	ctx   context.Context
	size  int64
}

func openOffice(ctx context.Context, file *os.File) (*officeZIP, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("读取 Office 文件失败")
	}
	z, err := zip.NewReader(file, info.Size())
	if err != nil {
		return nil, fmt.Errorf("Office 文件格式无效")
	}
	if len(z.File) > 20000 {
		return nil, fmt.Errorf("Office 文件包含过多条目")
	}
	out := &officeZIP{files: map[string]*zip.File{}, ctx: ctx}
	for _, f := range z.File {
		out.files[f.Name] = f
	}
	return out, nil
}
func (z *officeZIP) xml(name string) (*xmlNode, error) {
	if err := z.ctx.Err(); err != nil {
		return nil, err
	}
	f := z.files[name]
	if f == nil {
		return nil, fmt.Errorf("Office 文件缺少必要内容：%s", name)
	}
	if f.UncompressedSize64 > uint64(MaxTextSize) || z.size+int64(f.UncompressedSize64) > MaxTextSize {
		return nil, fmt.Errorf("Office 解压正文过大，无法处理")
	}
	r, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("读取 Office 内容失败")
	}
	defer r.Close()
	data, err := readLimited(r, MaxTextSize)
	if err != nil {
		return nil, err
	}
	z.size += int64(len(data))
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	root := &xmlNode{}
	stack := []*xmlNode{root}
	for {
		if err := z.ctx.Err(); err != nil {
			return nil, err
		}
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Office XML 内容损坏")
		}
		switch v := token.(type) {
		case xml.StartElement:
			if len(stack) > 100 {
				return nil, fmt.Errorf("Office XML 嵌套过深")
			}
			n := &xmlNode{Name: v.Name.Local, Attr: map[string]string{}}
			for _, a := range v.Attr {
				n.Attr[a.Name.Local] = a.Value
				if a.Name.Space == "http://schemas.openxmlformats.org/officeDocument/2006/relationships" {
					n.Attr["rel:"+a.Name.Local] = a.Value
				}
			}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, n)
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			stack[len(stack)-1].Text += string(v)
		}
	}
	if len(root.Children) != 1 {
		return nil, fmt.Errorf("Office XML 根节点无效")
	}
	return root.Children[0], nil
}
func (z *officeZIP) rels(part string) (map[string]string, error) {
	name := path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
	out := map[string]string{}
	if z.files[name] == nil {
		return out, nil
	}
	root, err := z.xml(name)
	if err != nil {
		return nil, err
	}
	for _, r := range root.Children {
		if r.Name != "Relationship" || r.value("TargetMode") == "External" {
			continue
		}
		target := r.value("Target")
		if strings.HasPrefix(target, "/") {
			target = path.Clean(strings.TrimPrefix(target, "/"))
		} else {
			target = path.Clean(path.Join(path.Dir(part), target))
		}
		if strings.HasPrefix(target, "../") {
			return nil, fmt.Errorf("Office 关联路径无效")
		}
		out[r.value("Id")] = target
	}
	return out, nil
}
func markdownTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	width := 0
	for _, r := range rows {
		width = max(width, len(r))
	}
	if width == 0 {
		return ""
	}
	var out strings.Builder
	for i, row := range rows {
		out.WriteString("| ")
		for j := 0; j < width; j++ {
			cell := ""
			if j < len(row) {
				cell = row[j]
			}
			cell = strings.ReplaceAll(strings.ReplaceAll(Clean(cell), "|", "\\|"), "\n", "<br>")
			out.WriteString(cell + " | ")
		}
		out.WriteString("\n")
		if i == 0 {
			out.WriteString("|" + strings.Repeat(" --- |", width) + "\n")
		}
	}
	return strings.TrimSpace(out.String())
}
