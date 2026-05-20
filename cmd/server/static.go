package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"electricquery/internal/logger"

	"github.com/gin-gonic/gin"
)

// serveFrontend serves the Vite-built SPA from frontend/dist.
// All unmatched routes fall back to index.html (SPA routing).
//
// Not using r.StaticFS("/", ...) because it registers a catch-all /*filepath wildcard
// that conflicts with static path segments like /health in the Gin radix tree.
// NoRoute lives outside the routing tree — no conflicts, perfect SPA fallback.
func serveFrontend(r *gin.Engine) {
	distDir := filepath.Join("frontend", "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		return
	}

	indexPath := filepath.Join(distDir, "index.html")
	fs := http.Dir(distDir)

	// Serve static files directly; any non-file path returns index.html for SPA routing.
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Strip leading "/" → relative path within dist
		rel := strings.TrimPrefix(path, "/")
		if rel == "" {
			rel = "index.html"
		}

		// Check if it's an actual file on disk
		if _, err := fs.Open(rel); err == nil {
			// File exists → serve it directly (assets, chunks, images, etc.)
			c.File(filepath.Join(distDir, rel))
			c.Abort()
			return
		}

		// Non-existent path (e.g. /dashboard, /settings) → serve index.html for Vue Router
		if _, err := os.Stat(indexPath); err == nil {
			c.File(indexPath)
			c.Abort()
			return
		}

		// index.html missing — shouldn't happen, but respond with a plain message
		c.String(http.StatusNotFound, "index.html not found in dist")
		c.Abort()
	})

	logger.Info("前端静态文件已挂载", "dist", distDir)
}
