package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"knownote/internal/config"
	"knownote/internal/storage"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type ImageOptions struct{ Size string }
type ImageResult struct {
	ObjectKey     string
	ContentType   string
	Width, Height int
}
type ImageClient interface {
	Generate(ctx context.Context, prompt string, opts ImageOptions) (ImageResult, error)
}
type imageClient struct {
	config config.Config
	store  storage.Store
	http   *http.Client
}

// NewImage 通过 Store.Put 保存生成结果，兼容本地与私有 OSS；业务路由尚未开放。
func NewImage(c config.Config, store storage.Store) ImageClient {
	c = c.Resolved()
	return &imageClient{c, store, &http.Client{Timeout: requestTimeout}}
}
func (c *imageClient) Generate(ctx context.Context, prompt string, opts ImageOptions) (ImageResult, error) {
	userID, _ := ctx.Value(userKey{}).(string)
	if _, err := uuid.Parse(userID); err != nil {
		return ImageResult{}, fmt.Errorf("图像生成缺少用户归属")
	}
	if strings.TrimSpace(prompt) == "" {
		return ImageResult{}, fmt.Errorf("请输入图像提示词")
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	var data []byte
	var err error
	if c.config.ImageAPIKey == "" || c.config.ImageProvider == "" {
		data, err = fakeImage(prompt)
	} else {
		data, err = retry(ctx, func() ([]byte, error) { return c.generate(ctx, prompt, opts) })
	}
	if err != nil {
		return ImageResult{}, modelError(err)
	}
	decoded, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || decoded.Width > 4096 || decoded.Height > 4096 {
		return ImageResult{}, fmt.Errorf("模型返回的图片格式或尺寸无效")
	}
	ext, contentType := "png", "image/png"
	if format == "jpeg" {
		ext, contentType = "jpg", "image/jpeg"
	}
	key := c.store.Key("images/" + userID + "/" + uuid.NewString() + "." + ext)
	if _, err = c.store.Put(ctx, key, bytes.NewReader(data), contentType); err != nil {
		return ImageResult{}, fmt.Errorf("保存生成图片失败")
	}
	return ImageResult{key, contentType, decoded.Width, decoded.Height}, nil
}
func (c *imageClient) generate(ctx context.Context, prompt string, opts ImageOptions) ([]byte, error) {
	size := opts.Size
	if size == "" {
		size = "1024x1024"
	}
	body, _ := json.Marshal(map[string]any{"model": c.config.ImageModel, "prompt": prompt, "n": 1, "size": size, "response_format": "b64_json"})
	request, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.config.ImageBaseURL, "/")+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("图像服务地址无效")
	}
	request.Header.Set("Authorization", "Bearer "+c.config.ImageAPIKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &imageHTTPError{response.StatusCode}
	}
	var result struct {
		Data []struct {
			B64 string `json:"b64_json"`
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 24<<20)).Decode(&result); err != nil || len(result.Data) == 0 {
		return nil, fmt.Errorf("图像服务返回了无效结果")
	}
	if result.Data[0].B64 != "" {
		data, err := base64.StdEncoding.DecodeString(result.Data[0].B64)
		if err != nil || len(data) > 16<<20 {
			return nil, fmt.Errorf("图像数据无效")
		}
		return data, nil
	}
	u, err := url.Parse(result.Data[0].URL)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("图像下载地址无效")
	}
	// 下载不附带模型服务鉴权，避免将 API Key 发到图片 CDN。
	request, err = http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err = c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, &imageHTTPError{response.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (16<<20)+1))
	if len(data) > 16<<20 {
		return nil, fmt.Errorf("图像文件过大")
	}
	return data, err
}

type imageHTTPError struct{ status int }

func (e *imageHTTPError) Error() string { return "图像服务请求失败" }
func fakeImage(prompt string) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, 768, 384))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.RGBA{239, 235, 255, 255}}, image.Point{}, draw.Src)
	var face font.Face = basicfont.Face7x13
	// 有中文系统字体时直接绘制，否则显示 Unicode 转义，仍保留提示词内容。
	for _, path := range []string{"/System/Library/Fonts/STHeiti Light.ttc", "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			continue
		}
		parsed, err := collection.Font(0)
		if err != nil {
			continue
		}
		loaded, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: 20, DPI: 72})
		if err == nil {
			face = loaded
			defer loaded.Close()
			break
		}
	}
	if face == basicfont.Face7x13 {
		prompt = strings.Trim(strconv.QuoteToASCII(prompt), "\"")
	}
	drawer := font.Drawer{Dst: canvas, Src: image.NewUniform(color.RGBA{60, 40, 100, 255}), Face: face, Dot: fixed.P(24, 36)}
	drawer.DrawString("KnowNote Fake Image")
	x, y := 24, 78
	for _, r := range prompt {
		advance, _ := face.GlyphAdvance(r)
		if x+advance.Ceil() > 740 || r == '\n' {
			x = 24
			y += 30
		}
		if y > 360 {
			break
		}
		drawer.Dot = fixed.P(x, y)
		if r != '\n' {
			drawer.DrawString(string(r))
			x += advance.Ceil()
		}
	}
	var output bytes.Buffer
	err := png.Encode(&output, canvas)
	return output.Bytes(), err
}
