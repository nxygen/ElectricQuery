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
// 使用 UUID（uid）而非自增整数，防止 ID 可枚举
type Claims struct {
	UID      string `json:"uid"`      // User.ID（UUID）
	Username string `json:"username"` // 仅用于日志展示，不作为鉴权依据
	jwt.RegisteredClaims
}

// GenerateToken 为用户签发 JWT，payload 中携带 UUID
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

// ParseToken 解析并验证 JWT，返回 Claims
func ParseToken(tokenStr string) (*Claims, error) {
	cfg := config.Load()
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// 强制验证签名算法，防止 alg:none 攻击
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

// JWTAuth 是 Gin JWT 鉴权中间件
// 验证通过后将 user_id（UUID string）和 username 写入 context
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

		c.Set("user_id", claims.UID)      // UUID string
		c.Set("username", claims.Username)
		c.Next()
	}
}

// GetUserID 从 Gin context 安全获取当前用户 UUID string
func GetUserID(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// InternalAuth 验证内部 API Token（供 worker/scheduler 调用）
// 配置中 internal_token 为空时拒绝所有请求（已在 InitDB 强制校验）
func InternalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()
		token := c.GetHeader("X-Internal-Token")
		if token == "" {
			token = c.Query("token")
		}
		// 无论 cfg.App.InternalToken 是否为空，均严格比对
		if token != cfg.App.InternalToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"code": 401, "msg": "内部 Token 无效"})
			return
		}
		c.Next()
	}
}

// AdminAuth 管理员鉴权：使用独立的 AdminToken，通过 X-Admin-Token 请求头传递
// 前端管理后台在 localStorage 存储该 token，由部署者手动设置
// AdminToken 独立于 InternalToken，权限分离：InternalToken 用于内部服务，AdminToken 用于管理后台
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()
		// 仅从请求头读取，不支持 URL Query 参数（防止 Token 写入日志/历史记录）
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
		// AdminToken 未配置时（向后兼容），fallback 到 InternalToken
		// 注意：生产环境应单独配置 AdminToken
		if cfg.App.AdminToken == "" && token != cfg.App.InternalToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"code": 401, "msg": "管理员 Token 无效"})
			return
		}
		c.Next()
	}
}

// CORS 跨域中间件
// 生产环境通过 config 指定 AllowedOrigin，开发环境允许 *
// 安全策略：Origin 必须与 AllowedOrigin 严格匹配，不匹配则拒绝跨域请求
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()
		origin := c.GetHeader("Origin")

		allowOrigin := "*"
		if cfg.App.Mode != "debug" && cfg.App.AllowedOrigin != "" {
			// 生产：Origin 必须与配置的 AllowedOrigin 严格匹配，否则拒绝
			if origin == cfg.App.AllowedOrigin {
				allowOrigin = origin
			} else {
				// Origin 不匹配时设置为空（不设置 Access-Control-Allow-Origin）
				// 浏览器会因缺少该头而阻止跨域响应，保护后端不被滥用
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
