package handler

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
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

	// Parse resize params
	w := 0
	if wStr := c.Query("w"); wStr != "" {
		if n, err := strconv.Atoi(wStr); err == nil && n > 0 && n <= 4000 {
			w = n
		}
	}
	q := 80
	if qStr := c.Query("q"); qStr != "" {
		if n, err := strconv.Atoi(qStr); err == nil && n >= 1 && n <= 100 {
			q = n
		}
	}

	ext := strings.ToLower(filepath.Ext(key))
	ct, ok := mimeTypes[ext]
	if !ok {
		ct = mime.TypeByExtension(ext)
		if ct == "" {
			ct = "application/octet-stream"
		}
	}

	// Resize/re-encode only if w param set and file is an image we can process
	if w > 0 && (ext == ".webp" || ext == ".jpg" || ext == ".jpeg" || ext == ".png") {
		if resized, err := resizeImage(data, w, q); err == nil {
			etag := fmt.Sprintf(`"%x"`, md5.Sum(resized))
			if c.GetHeader("If-None-Match") == etag {
				c.Status(http.StatusNotModified)
				return
			}
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			c.Header("ETag", etag)
			c.Data(http.StatusOK, "image/webp", resized)
			return
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

func resizeImage(data []byte, width, quality int) ([]byte, error) {
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}

	if img.Bounds().Dx() > width {
		img = imaging.Resize(img, width, 0, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.Options{Lossless: false, Quality: float32(quality)}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
