package parser

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var officeSlots = make(chan struct{}, 2)

func SofficePath() (string, error) {
	if configured := os.Getenv("SOFFICE_PATH"); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("需要安装 LibreOffice 才能解析旧版 Office 文件")
	}
	for _, name := range []string{"soffice", "libreoffice", "/Applications/LibreOffice.app/Contents/MacOS/soffice"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("需要安装 LibreOffice 才能解析旧版 Office 文件")
}

// Convert 每次使用独立配置目录，避免连接用户正在打开的 Office 进程。
func Convert(ctx context.Context, input, format string) (string, func(), error) {
	executable, err := SofficePath()
	if err != nil {
		return "", func() {}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	select {
	case officeSlots <- struct{}{}:
	case <-ctx.Done():
		return "", func() {}, ctx.Err()
	}
	defer func() { <-officeSlots }()
	dir, err := os.MkdirTemp("", "knownote-office-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("创建转换目录失败")
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	linkCJKFonts(filepath.Join(dir, "profile"))
	profile := (&url.URL{Scheme: "file", Path: filepath.Join(dir, "profile")}).String()
	command := exec.CommandContext(ctx, executable, "-env:UserInstallation="+profile, "--headless", "--convert-to", format, "--outdir", dir, input)
	// 不输出转换器日志，日志可能包含用户文档内容或内部路径。
	if err := command.Run(); err != nil {
		cleanup()
		if ctx.Err() != nil {
			return "", func() {}, fmt.Errorf("Office 文件转换超时或已取消")
		}
		return "", func() {}, fmt.Errorf("Office 文件转换失败，请检查文件格式")
	}
	output := filepath.Join(dir, filepath.Base(input[:len(input)-len(filepath.Ext(input))])+"."+format)
	if info, err := os.Stat(output); err != nil || info.Size() == 0 {
		cleanup()
		return "", func() {}, fmt.Errorf("Office 文件转换失败，请检查文件格式")
	}
	return output, cleanup, nil
}

// 无界面运行的 LibreOffice 往往读取不到系统字体，中文会被替换成拉丁字体导致预览 PDF 中文空白。
// 这里把可用的中文字体以符号链接放进本次转换的配置目录 user/fonts，LibreOffice 会直接加载。
// 可用 SOFFICE_FONT_PATHS（冒号分隔的字体文件或目录）追加；项目内 ./fonts 目录也会被使用。
var cjkFontCandidates = []string{
	// macOS
	"/System/Library/Fonts/Hiragino Sans GB.ttc",
	"/System/Library/Fonts/STHeiti Medium.ttc",
	"/System/Library/Fonts/STHeiti Light.ttc",
	"/System/Library/Fonts/Supplemental/Songti.ttc",
	"/Library/Fonts/Arial Unicode.ttf",
	"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
	// Linux（如 apt install fonts-noto-cjk fonts-wqy-microhei）
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/opentype/noto/NotoSerifCJK-Regular.ttc",
	"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
	"/usr/share/fonts/wqy-microhei/wqy-microhei.ttc",
	"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
	// Windows
	`C:\Windows\Fonts\msyh.ttc`,
	`C:\Windows\Fonts\simsun.ttc`,
}

// CJKFontFiles 返回本机可用于 LibreOffice 转换的中文字体文件（去重、按优先级）。
func CJKFontFiles() []string {
	var candidates []string
	if extra := os.Getenv("SOFFICE_FONT_PATHS"); extra != "" {
		candidates = append(candidates, filepath.SplitList(extra)...)
	}
	candidates = append(candidates, "fonts")
	candidates = append(candidates, cjkFontCandidates...)
	seen := map[string]bool{}
	var files []string
	add := func(path string) {
		lower := strings.ToLower(path)
		if !(strings.HasSuffix(lower, ".ttf") || strings.HasSuffix(lower, ".ttc") || strings.HasSuffix(lower, ".otf") || strings.HasSuffix(lower, ".otc")) {
			return
		}
		if abs, err := filepath.Abs(path); err == nil && !seen[abs] {
			seen[abs] = true
			files = append(files, abs)
		}
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			add(candidate)
			continue
		}
		entries, err := os.ReadDir(candidate)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				add(filepath.Join(candidate, entry.Name()))
			}
		}
	}
	return files
}

func linkCJKFonts(profileDir string) {
	files := CJKFontFiles()
	if len(files) == 0 {
		return
	}
	fontDir := filepath.Join(profileDir, "user", "fonts")
	if err := os.MkdirAll(fontDir, 0o700); err != nil {
		return
	}
	for i, file := range files {
		// 文件名加序号，避免不同目录下同名字体互相覆盖。
		name := fmt.Sprintf("%02d-%s", i, filepath.Base(file))
		_ = os.Symlink(file, filepath.Join(fontDir, name))
	}
}
