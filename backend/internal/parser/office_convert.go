package parser

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
