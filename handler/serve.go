package handler

import (
	"crypto/md5"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

var mimeTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".svg":  "image/svg+xml",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
}

func ServeHandler(c *gin.Context) {
	key := c.Param("key")
	if strings.Contains(key, "..") || strings.Contains(key, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key"})
		return
	}

	path := filepath.Join(storagePath, key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read error"})
		}
		return
	}

	ext := strings.ToLower(filepath.Ext(key))
	ct, ok := mimeTypes[ext]
	if !ok {
		ct = mime.TypeByExtension(ext)
		if ct == "" {
			ct = "application/octet-stream"
		}
	}

	etag := fmt.Sprintf(`"%x"`, md5.Sum(data))
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("ETag", etag)
	c.Data(http.StatusOK, ct, data)
}

func DeleteHandler(c *gin.Context) {
	if apiKey != "" && c.GetHeader("X-Api-Key") != apiKey {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
		return
	}

	key := c.Param("key")
	if strings.Contains(key, "..") || strings.Contains(key, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key"})
		return
	}

	path := filepath.Join(storagePath, key)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "delete error"})
		}
		return
	}

	c.Status(http.StatusNoContent)
}
