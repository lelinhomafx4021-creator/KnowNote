package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func avatarImage(t *testing.T, format string, img image.Image) []byte {
	t.Helper()
	var data bytes.Buffer
	var err error
	if format == "png" {
		err = png.Encode(&data, img)
	} else {
		err = jpeg.Encode(&data, img, &jpeg.Options{Quality: 95})
	}
	if err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestNormalizeAvatarFormatsAndDimensions(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 37, 19))
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			data, err := NormalizeAvatar(bytes.NewReader(avatarImage(t, format, img)))
			if err != nil {
				t.Fatal(err)
			}
			config, gotFormat, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil || gotFormat != "jpeg" || config.Width != 256 || config.Height != 256 {
				t.Fatalf("规范化结果必须是 256×256 JPEG：%+v %s %v", config, gotFormat, err)
			}
		})
	}
}

func TestNormalizeAvatarRejectsInvalidAndOversize(t *testing.T) {
	valid := avatarImage(t, "png", image.NewRGBA(image.Rect(0, 0, 4, 4)))
	for _, input := range [][]byte{nil, []byte("这是一段文本，改名为头像.jpg 也不是图片"), []byte("RIFF0000WEBPVP8 "), valid[:len(valid)-15]} {
		if _, err := NormalizeAvatar(bytes.NewReader(input)); !errors.Is(err, ErrAvatarFormat) {
			t.Fatalf("无效或损坏图片未拒绝：%v", err)
		}
	}
	oversize := make([]byte, MaxAvatarSize+1)
	copy(oversize, valid)
	if _, err := NormalizeAvatar(bytes.NewReader(oversize)); !errors.Is(err, ErrAvatarTooLarge) {
		t.Fatalf("超大头像未拒绝：%v", err)
	}
	if _, err := NormalizeAvatar(bytes.NewReader(oversize[:MaxAvatarSize])); err != nil {
		t.Fatalf("恰好 2MB 的合法图片被拒绝：%v", err)
	}
	// 只修改 IHDR 和校验码，不分配超大像素缓冲，验证解码前的像素数限制。
	bomb := bytes.Clone(valid)
	binary.BigEndian.PutUint32(bomb[16:20], 100_000)
	binary.BigEndian.PutUint32(bomb[20:24], 100_000)
	binary.BigEndian.PutUint32(bomb[29:33], crc32.ChecksumIEEE(bomb[12:29]))
	if _, err := NormalizeAvatar(bytes.NewReader(bomb)); !errors.Is(err, ErrAvatarFormat) {
		t.Fatalf("超大像素图片未拒绝：%v", err)
	}
}

func TestNormalizeAvatarCenterCrop(t *testing.T) {
	for _, wide := range []bool{true, false} {
		width, height := 60, 20
		if !wide {
			width, height = height, width
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				position := x
				if !wide {
					position = y
				}
				fill := color.RGBA{R: 255, A: 255}
				if position >= 20 && position < 40 {
					fill = color.RGBA{G: 255, A: 255}
				} else if position >= 40 {
					fill = color.RGBA{B: 255, A: 255}
				}
				img.SetRGBA(x, y, fill)
			}
		}
		data, err := NormalizeAvatar(bytes.NewReader(avatarImage(t, "png", img)))
		if err != nil {
			t.Fatal(err)
		}
		result, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range []image.Point{{0, 0}, {255, 0}, {0, 255}, {255, 255}, {128, 128}} {
			r, g, b, _ := result.At(p.X, p.Y).RGBA()
			if r > 5*257 || g < 250*257 || b > 5*257 {
				t.Fatalf("没有保留中央绿色方块：横图=%v 坐标=%v 颜色=%d,%d,%d", wide, p, r, g, b)
			}
		}
	}
}

func TestNormalizeAvatarWhiteBackgroundAndMetadata(t *testing.T) {
	data, err := NormalizeAvatar(bytes.NewReader(avatarImage(t, "png", image.NewNRGBA(image.Rect(0, 0, 1, 1)))))
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(0, 0).RGBA()
	if r < 254*257 || g < 254*257 || b < 254*257 {
		t.Fatal("透明区域没有填充白色")
	}
	original := avatarImage(t, "jpeg", image.NewRGBA(image.Rect(0, 0, 2, 2)))
	metadata := []byte("头像原始元数据不应保留")
	segment := []byte{0xff, 0xfe, 0, byte(len(metadata) + 2)}
	original = append(append(append(bytes.Clone(original[:2]), segment...), metadata...), original[2:]...)
	normalized, err := NormalizeAvatar(bytes.NewReader(original))
	if err != nil || bytes.Contains(normalized, metadata) {
		t.Fatalf("重新编码没有移除元数据：%v", err)
	}
}
