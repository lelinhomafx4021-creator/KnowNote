package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"
	"path"

	"knownote/internal/model"
)

const MaxAvatarSize int64 = 2 << 20
const avatarSize = 256
const maxAvatarPixels = 25_000_000

var ErrAvatarFormat = &Error{Status: 400, Code: 40001, Message: "请上传 PNG 或 JPEG 图片"}
var ErrAvatarTooLarge = &Error{Status: 400, Code: 40001, Message: "头像不能超过 2MB"}

// NormalizeAvatar 按内容识别图片，先限制解码开销，再裁切、缩放并移除元数据。
func NormalizeAvatar(input io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(input, MaxAvatarSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxAvatarSize {
		return nil, ErrAvatarTooLarge
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || config.Width <= 0 || config.Height <= 0 || config.Width > maxAvatarPixels/config.Height {
		return nil, ErrAvatarFormat
	}
	source, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decodedFormat != format {
		return nil, ErrAvatarFormat
	}
	bounds := source.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	left := bounds.Min.X + (bounds.Dx()-side)/2
	top := bounds.Min.Y + (bounds.Dy()-side)/2
	result := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	// 在居中裁切区域内双线性采样；RGBA 的预乘通道与白底合成，避免透明区域变黑。
	for y := 0; y < avatarSize; y++ {
		sy := max(0, min(float64(side-1), (float64(y)+0.5)*float64(side)/avatarSize-0.5))
		y0 := int(math.Floor(sy))
		fy := sy - float64(y0)
		for x := 0; x < avatarSize; x++ {
			sx := max(0, min(float64(side-1), (float64(x)+0.5)*float64(side)/avatarSize-0.5))
			x0 := int(math.Floor(sx))
			fx := sx - float64(x0)
			var channels [3]float64
			for dy := 0; dy <= 1; dy++ {
				wy := 1 - fy
				if dy == 1 {
					wy = fy
				}
				for dx := 0; dx <= 1; dx++ {
					wx := 1 - fx
					if dx == 1 {
						wx = fx
					}
					r, g, b, a := source.At(left+min(x0+dx, side-1), top+min(y0+dy, side-1)).RGBA()
					weight := wx * wy / 257
					channels[0] += float64(r+65535-a) * weight
					channels[1] += float64(g+65535-a) * weight
					channels[2] += float64(b+65535-a) * weight
				}
			}
			result.SetRGBA(x, y, color.RGBA{R: uint8(math.Round(channels[0])), G: uint8(math.Round(channels[1])), B: uint8(math.Round(channels[2])), A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, result, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

func (s *Service) avatarKey(userID string) string {
	return s.Storage.Key(path.Join("avatars", userID, "avatar.jpg"))
}

func (s *Service) UploadAvatar(ctx context.Context, userID string, input io.Reader) (UserView, error) {
	user, err := s.Me(ctx, userID)
	if err != nil {
		return UserView{}, err
	}
	data, err := NormalizeAvatar(input)
	if err != nil {
		return UserView{}, err
	}
	if s.Storage == nil {
		return UserView{}, errors.New("头像存储未配置")
	}
	if _, err := s.Storage.Put(ctx, s.avatarKey(userID), bytes.NewReader(data), "image/jpeg"); err != nil {
		return UserView{}, err
	}
	user.AvatarURL = "/api/files/avatar/" + userID
	result := s.DB.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("avatar_url", user.AvatarURL)
	if result.Error != nil {
		return UserView{}, result.Error
	}
	if result.RowsAffected == 0 {
		return UserView{}, ErrNotFound
	}
	return user, nil
}

func (s *Service) DeleteAvatar(ctx context.Context, userID string) (UserView, error) {
	user, err := s.Me(ctx, userID)
	if err != nil {
		return UserView{}, err
	}
	if s.Storage == nil {
		return UserView{}, errors.New("头像存储未配置")
	}
	if err := s.Storage.Delete(ctx, s.avatarKey(userID)); err != nil {
		return UserView{}, err
	}
	result := s.DB.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("avatar_url", "")
	if result.Error != nil {
		return UserView{}, result.Error
	}
	if result.RowsAffected == 0 {
		return UserView{}, ErrNotFound
	}
	user.AvatarURL = ""
	return user, nil
}

func (s *Service) AvatarFile(ctx context.Context, userID, requestedID string) (string, error) {
	if userID != requestedID {
		return "", ErrNotFound
	}
	user, err := s.Me(ctx, userID)
	if err != nil {
		return "", err
	}
	if user.AvatarURL == "" {
		return "", ErrNotFound
	}
	if s.Storage == nil {
		return "", errors.New("头像存储未配置")
	}
	return s.avatarKey(userID), nil
}
