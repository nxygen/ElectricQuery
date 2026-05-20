package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"electricquery/internal/logger"

	"github.com/gin-gonic/gin"
)

func serveFrontend(r *gin.Engine) {
	distDir := filepath.Join("frontend", "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		return
	}

	indexPath := filepath.Join(distDir, "index.html")
	fs := http.Dir(distDir)

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		rel := strings.TrimPrefix(path, "/")
		if rel == "" {
			rel = "index.html"
		}

		if _, err := fs.Open(rel); err == nil {
			c.File(filepath.Join(distDir, rel))
			c.Abort()
			return
		}

		if _, err := os.Stat(indexPath); err == nil {
			c.File(indexPath)
			c.Abort()
			return
		}

		c.String(http.StatusNotFound, "index.html not found in dist")
		c.Abort()
	})

	logger.Info("前端静态文件已挂载", "dist", distDir)
}
