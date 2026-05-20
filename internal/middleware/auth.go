package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"electricquery/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateCSRF generate CSRF token
func GenerateCSRF() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成 CSRF Token 失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ValidateCSRF validate CSRF token from header
func ValidateCSRF(c *gin.Context) bool {
	cfg := config.Load()
	if cfg.App.Mode == "debug" {
		return true // debug 模式跳过 CSRF 验证
	}

	// 从 Cookie 中获取 CSRF Token
	csrfCookie, err := c.Cookie("csrf_token")
	if err != nil {
		return false
	}

	// 从 Header 中获取 CSRF Token
	csrfHeader := c.GetHeader("X-CSRF-Token")
	if csrfHeader == "" {
		return false
	}

	// 比较 Cookie 和 Header 中的 CSRF Token
	return csrfCookie == csrfHeader
}

func GenerateToken(uid, username string) (string, error) {
	cfg := config.Load()
	expire := time.Duration(cfg.App.JWTExpireHours) * time.Hour
	claims := Claims{
		UID:      uid,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "electricquery",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.App.JWTSecret))
}

func ParseToken(tokenStr string) (*Claims, error) {
	cfg := config.Load()
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.App.JWTSecret), nil
		})
	if err != nil || !token.Valid {
		return nil, err
	}
	return claims, nil
}

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"code": 401, "msg": "缺少 Authorization 头"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"code": 401, "msg": "Authorization 格式错误，应为 Bearer <token>"})
			return
		}

		claims, err := ParseToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"code": 401, "msg": "Token 无效或已过期"})
			return
		}

		// CSRF 验证
		if !ValidateCSRF(c) {
			c.AbortWithStatusJSON(http.StatusForbidden,
				gin.H{"code": 403, "msg": "CSRF Token 验证失败"})
			return
		}

		c.Set("user_id", claims.UID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func GetUserID(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

func InternalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()
		token := c.GetHeader("X-Internal-Token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"code": 401, "msg": "内部 Token 为空，请通过 X-Internal-Token 请求头提供"})
			return
		}
		if token != cfg.App.InternalToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"code": 401, "msg": "内部 Token 无效"})
			return
		}
		c.Next()
	}
}

func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()
		token := c.GetHeader("X-Admin-Token")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"code": 401, "msg": "管理员 Token 为空，请通过 X-Admin-Token 请求头提供"})
			return
		}
		if cfg.App.AdminToken != "" && token != cfg.App.AdminToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"code": 401, "msg": "管理员 Token 无效"})
			return
		}
		if cfg.App.AdminToken == "" && token != cfg.App.InternalToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"code": 401, "msg": "管理员 Token 无效"})
			return
		}
		c.Next()
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()
		origin := c.GetHeader("Origin")

		allowOrigin := "*"
		if cfg.App.Mode != "debug" && cfg.App.AllowedOrigin != "" {
			if origin == cfg.App.AllowedOrigin {
				allowOrigin = origin
			} else {
				allowOrigin = ""
			}
		}

		if allowOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Internal-Token, X-Admin-Token")
		c.Header("Vary", "Origin")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
