package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"knownote/pkg/response"
)

const UserIDKey = "userID"
const TokenLifetime = 7 * 24 * time.Hour

type Tokens struct{ secret []byte }

func NewTokens(secret string) *Tokens { return &Tokens{secret: []byte(secret)} }

func (t *Tokens) Issue(userID string) (string, error) {
	if _, err := uuid.Parse(userID); err != nil || len(t.secret) == 0 {
		return "", fmt.Errorf("生成登录凭证失败")
	}
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: userID, Issuer: "knownote", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(TokenLifetime)),
	}).SignedString(t.secret)
}

func (t *Tokens) Verify(raw string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("knownote"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid || len(t.secret) == 0 {
		return "", fmt.Errorf("登录凭证无效或已过期")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil || id == uuid.Nil {
		return "", fmt.Errorf("登录凭证无效或已过期")
	}
	return id.String(), nil
}

func Auth(tokens *Tokens) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.Query("token")
		if header := c.GetHeader("Authorization"); header != "" {
			parts := strings.Fields(header)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "请先登录")
				return
			}
			raw = parts[1]
		}
		userID, err := tokens.Verify(raw)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "登录凭证无效或已过期，请重新登录")
			return
		}
		c.Set(UserIDKey, userID)
		c.Next()
	}
}

func UserID(c *gin.Context) string { return c.GetString(UserIDKey) }
