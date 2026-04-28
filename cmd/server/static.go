package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// serveFrontend 挂载前端静态文件
// dist 目录不存在时静默跳过（开发模式可仅跑后端 API）
func serveFrontend(r *gin.Engine) {
	distDir := filepath.Join("frontend", "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		// dist 不存在，跳过静态文件挂载（后端 API 仍可正常访问）
		return
	}

	r.Static("/assets", filepath.Join(distDir, "assets"))
	r.StaticFS("/", http.Dir(distDir))

	// SPA 兜底：所有未匹配路由返回 index.html（让 Vue Router 处理）
	r.NoRoute(func(c *gin.Context) {
		indexPath := filepath.Join(distDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			c.File(indexPath)
		} else {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "页面不存在"})
		}
	})

	log.Printf("[boot] 前端静态文件已挂载: %s", distDir)
}
