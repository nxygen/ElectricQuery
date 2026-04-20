// Package middleware 提供 Gin 中间件
package middleware

import (
	"net/http"
	"strings"
	"time"

	"electricquery/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims 是 JWT 的 Payload 结构
type Claims struct {
	UserID    uint   `json:"uid"`
	StudentID string `json:"sid"`
	jwt.RegisteredClaims
}

// GenerateToken 为用户生成 JWT
func GenerateToken(userID uint, studentID string) (string, error) {
	cfg := config.Load()
	expire := time.Duration(cfg.App.JWTExpireHours) * time.Hour
	claims := Claims{
		UserID:    userID,
		StudentID: studentID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "electricquery",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.App.JWTSecret))
}

// ParseToken 解析并验证 JWT，返回 Claims
func ParseToken(tokenStr string) (*Claims, error) {
	cfg := config.Load()
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(cfg.App.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	return claims, nil
}

// JWTAuth 是 Gin JWT 鉴权中间件
// 从 Authorization: Bearer <token> 头中提取并验证 token
// 验证通过后将 user_id 和 student_id 写入 context
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "缺少 Authorization 头"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Authorization 格式错误，应为 Bearer <token>"})
			return
		}

		claims, err := ParseToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "Token 无效或已过期"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("student_id", claims.StudentID)
		c.Next()
	}
}

// InternalAuth 验证内部 API Token（供 worker/scheduler 调用）
func InternalAuth() gin.HandlerFunc {
	cfg := config.Load()
	return func(c *gin.Context) {
		token := c.GetHeader("X-Internal-Token")
		if token == "" {
			token = c.Query("token")
		}
		if cfg.App.InternalToken != "" && token != cfg.App.InternalToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "内部 Token 无效"})
			return
		}
		c.Next()
	}
}

// CORS 跨域中间件（前后端分离部署时需要）
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Internal-Token")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// GetUserID 从 Gin context 安全获取当前用户 ID
func GetUserID(c *gin.Context) uint {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}
