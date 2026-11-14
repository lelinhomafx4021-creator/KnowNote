package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"knownote/internal/llm"
	"knownote/internal/model"
)

type UserView struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Nickname  string    `json:"nickname"`
	AvatarURL string    `json:"avatarUrl,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type AuthResult struct {
	Token string   `json:"token"`
	User  UserView `json:"user"`
}

func userView(user model.User) UserView {
	return UserView{ID: user.ID, Email: user.Email, Nickname: user.Nickname, AvatarURL: user.AvatarURL, CreatedAt: user.CreatedAt}
}

func (s *Service) Register(ctx context.Context, email, password, nickname string) (AuthResult, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}
	user := model.User{Email: strings.ToLower(strings.TrimSpace(email)), PasswordHash: string(hash), Nickname: strings.TrimSpace(nickname)}
	var token string
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrEmailConflict
			}
			return err
		}
		settings := model.UserSettings{UserID: user.ID, Model: s.DefaultModel, AnswerStyle: "concise", Language: "zh-CN"}
		if err := tx.Create(&settings).Error; err != nil {
			return err
		}
		var err error
		token, err = s.Tokens.Issue(user.ID)
		return err
	})
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Token: token, User: userView(user)}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthResult, error) {
	var user model.User
	err := s.DB.WithContext(ctx).Where("email = ?", strings.ToLower(strings.TrimSpace(email))).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AuthResult{}, ErrUnauthorized
	}
	if err != nil {
		return AuthResult{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return AuthResult{}, ErrUnauthorized
	}
	token, err := s.Tokens.Issue(user.ID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Token: token, User: userView(user)}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (UserView, error) {
	var user model.User
	if err := s.DB.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return UserView{}, databaseError(err)
	}
	return userView(user), nil
}

const StorageTotal int64 = 5 * 1024 * 1024 * 1024

type SettingsView struct {
	Model        string `json:"model"`
	AnswerStyle  string `json:"answerStyle"`
	Language     string `json:"language"`
	StorageUsed  int64  `json:"storageUsed"`
	StorageTotal int64  `json:"storageTotal"`
}

func (s *Service) Settings(ctx context.Context, userID string) (SettingsView, error) {
	var settings model.UserSettings
	if err := s.DB.WithContext(ctx).Where("user_id = ?", userID).First(&settings).Error; err != nil {
		return SettingsView{}, databaseError(err)
	}
	var used int64
	if err := s.DB.WithContext(ctx).Model(&model.Source{}).Where("user_id = ?", userID).Select("COALESCE(SUM(size), 0)").Scan(&used).Error; err != nil {
		return SettingsView{}, err
	}
	if !llm.AllowedModel(s.Models, settings.Model) {
		settings.Model = s.DefaultModel
	}
	return SettingsView{Model: settings.Model, AnswerStyle: settings.AnswerStyle, Language: settings.Language, StorageUsed: used, StorageTotal: StorageTotal}, nil
}

func (s *Service) UpdateSettings(ctx context.Context, userID string, updates map[string]any) (SettingsView, error) {
	if value, ok := updates["model"].(string); ok && !llm.AllowedModel(s.Models, value) {
		return SettingsView{}, &Error{Status: 400, Code: 40001, Message: "请选择可用的对话模型"}
	}
	if len(updates) != 0 {
		result := s.DB.WithContext(ctx).Model(&model.UserSettings{}).Where("user_id = ?", userID).Updates(updates)
		if result.Error != nil {
			return SettingsView{}, result.Error
		}
		if result.RowsAffected == 0 {
			return SettingsView{}, ErrNotFound
		}
	}
	return s.Settings(ctx, userID)
}
