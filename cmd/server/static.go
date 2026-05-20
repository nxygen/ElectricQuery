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
//
// 注意：使用 r.NoRoute + gin.WrapH(http.FileServer(...)) 而非 r.StaticFS("/")。
// r.StaticFS("/", ...) 会在 Gin radix tree 中注册 catch-all /*filepath，
// 当 /health 等路径已存在时与之冲突（同一前缀下不能同时有通配符和静态段）。
// NoRoute 的处理函数不在路由树中，仅在"无任何路由匹配"时触发，
// 因此不会与 API 路由冲突，同时完美实现 SPA 兜底。
func serveFrontend(r *gin.Engine) {
	distDir := filepath.Join("frontend", "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		// dist 不存在，跳过静态文件挂载（后端 API 仍可正常访问）
		return
	}

	fs := http.FileSystem(http.Dir(distDir))

	// 所有未匹配路由由 FileServer 处理：
	//   - 存在的文件（如 /assets/index-abc.js） → 200 + 文件内容
	//   - 不存在的路径（如 /dashboard）          → 404
	//   - 根路径 /                              → 200 + index.html
	r.NoRoute(gin.WrapH(http.FileServer(fs)))

	log.Printf("[boot] 前端静态文件已挂载: %s", distDir)
}
