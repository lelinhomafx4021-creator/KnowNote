package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestTokens(t *testing.T) {
	secret := strings.Repeat("t", 32)
	tokens := NewTokens(secret)
	id := uuid.NewString()
	raw, err := tokens.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	if actual, err := tokens.Verify(raw); err != nil || actual != id {
		t.Fatal("签发的凭证无法通过验证")
	}
	parsed, err := jwt.ParseWithClaims(raw, &jwt.RegisteredClaims{}, func(*jwt.Token) (any, error) { return []byte(secret), nil })
	if err != nil {
		t.Fatal(err)
	}
	claims := parsed.Claims.(*jwt.RegisteredClaims)
	if claims.ExpiresAt.Sub(claims.IssuedAt.Time) != TokenLifetime {
		t.Fatal("登录有效期不是七天")
	}
	if _, err := NewTokens("other-secret").Verify(raw); err == nil {
		t.Fatal("错误密钥不应通过验证")
	}
	for _, test := range []struct {
		name   string
		method jwt.SigningMethod
		mutate func(*jwt.RegisteredClaims)
	}{
		{"过期", jwt.SigningMethodHS256, func(c *jwt.RegisteredClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second)) }},
		{"算法", jwt.SigningMethodHS384, func(*jwt.RegisteredClaims) {}},
		{"缺少有效期", jwt.SigningMethodHS256, func(c *jwt.RegisteredClaims) { c.ExpiresAt = nil }},
		{"错误用户", jwt.SigningMethodHS256, func(c *jwt.RegisteredClaims) { c.Subject = "invalid" }},
		{"错误签发者", jwt.SigningMethodHS256, func(c *jwt.RegisteredClaims) { c.Issuer = "other" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := jwt.RegisteredClaims{Subject: id, Issuer: "knownote", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}
			test.mutate(&c)
			raw, err := jwt.NewWithClaims(test.method, c).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tokens.Verify(raw); err == nil {
				t.Fatal("无效凭证不应通过验证")
			}
		})
	}
}

func TestAuthHeadersAndQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := NewTokens(strings.Repeat("a", 32))
	id := uuid.NewString()
	raw, err := tokens.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/private", Auth(tokens), func(c *gin.Context) {
		if UserID(c) != id {
			t.Error("用户身份未写入上下文")
		}
		c.Status(http.StatusOK)
	})
	for _, test := range []struct {
		name, query, header string
		status              int
	}{
		{"请求头", "", "Bearer " + raw, 200}, {"音频查询参数", "?token=" + raw, "", 200},
		{"未登录", "", "", 401}, {"无效凭证", "", "Bearer invalid", 401},
		{"错误请求头不得降级", "?token=" + raw, "Basic invalid", 401},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/private"+test.query, nil)
			request.Header.Set("Authorization", test.header)
			writer := httptest.NewRecorder()
			router.ServeHTTP(writer, request)
			if writer.Code != test.status {
				t.Fatalf("状态码为 %d，预期 %d", writer.Code, test.status)
			}
		})
	}
}
