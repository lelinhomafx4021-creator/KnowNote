package service

import (
	"context"
	"gorm.io/gorm"
	"knownote/internal/model"
	"strings"
	"time"
)

var ErrAuthLimited = &Error{429, 42901, "尝试次数过多，请稍后再试"}

// Authenticate 用数据库邮箱级事务锁串行化失败计数与记录，多个服务进程共用限流。
func (s *Service) Authenticate(ctx context.Context, email, password, nickname, ip, userAgent string, register bool, validationErr error) (AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 254 {
		email = ""
	}
	var result AuthResult
	var outcome error
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 10))", email).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.LoginLog{}).Where("email = ? AND success = false AND created_at >= ?", email, time.Now().Add(-10*time.Minute)).Count(&count).Error; err != nil {
			return err
		}
		outcome = validationErr
		if count >= 5 {
			outcome = ErrAuthLimited
		} else if outcome == nil {
			scoped := *s
			scoped.DB = tx
			if register {
				result, outcome = scoped.Register(ctx, email, password, nickname)
			} else {
				result, outcome = scoped.Login(ctx, email, password)
			}
		}
		entry := model.LoginLog{Email: email, Success: outcome == nil, IP: ip, UserAgent: string([]rune(userAgent)[:min(512, len([]rune(userAgent)))])}
		if outcome == nil {
			entry.UserID = &result.User.ID
		}
		return tx.Create(&entry).Error
	})
	if err != nil {
		return AuthResult{}, err
	}
	return result, outcome
}

func (s *Service) LoginLogs(ctx context.Context, userID string) ([]model.LoginLog, error) {
	var user model.User
	if err := s.DB.WithContext(ctx).Select("id", "email").Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, databaseError(err)
	}
	items := make([]model.LoginLog, 0)
	err := s.DB.WithContext(ctx).Where("user_id = ? OR (user_id IS NULL AND email = ?)", userID, user.Email).Order("created_at DESC, id DESC").Limit(20).Find(&items).Error
	return items, err
}
