package handler_test

import (
	"context"
	"io"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
	"log/slog"
	"testing"
)

// 默认管理员：首次启动创建并可登录；已存在时不覆盖口令。
func TestSeedAdminCreatesOnceAndKeepsPassword(t *testing.T) {
	database := sourceDatabase(t)
	ctx := context.Background()
	s := service.New(database, middleware.NewTokens("管理员预置测试密钥-至少三十二字节的随机内容"), "测试模型", slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.AdminEmails, s.AdminPassword = []string{"seed-admin@example.test"}, "初始口令-1234567890"
	if err := s.SeedAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := s.Login(ctx, "SEED-admin@example.test", s.AdminPassword)
	if err != nil || result.User.Role != "admin" {
		t.Fatalf("默认管理员应可登录且为管理员: %v %+v", err, result.User)
	}
	// 管理员改过口令后，重启（再次预置）不能把口令改回环境变量里的值。
	s.AdminPassword = "另一个口令-0987654321"
	if err := s.SeedAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login(ctx, "seed-admin@example.test", s.AdminPassword); err == nil {
		t.Fatal("已存在的管理员口令不应被环境变量覆盖")
	}
	if _, err := s.Login(ctx, "seed-admin@example.test", "初始口令-1234567890"); err != nil {
		t.Fatal("原口令应保持有效")
	}
	var count int64
	database.Model(&model.User{}).Where("email = ?", "seed-admin@example.test").Count(&count)
	if count != 1 {
		t.Fatalf("应只有一个默认管理员，实际 %d", count)
	}
	// 未配置口令时不创建任何账号。
	s.AdminEmails, s.AdminPassword = []string{"nobody@example.test"}, ""
	if err := s.SeedAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	database.Model(&model.User{}).Where("email = ?", "nobody@example.test").Count(&count)
	if count != 0 {
		t.Fatal("未配置 ADMIN_PASSWORD 不应创建账号")
	}
}
