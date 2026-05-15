package handler

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"log"
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

// variantKey returns the cache filename for a resized variant.
// Format: {base}__w{width}q{quality}.webp
func variantKey(key string, w, q int) string {
	ext := filepath.Ext(key)
	base := strings.TrimSuffix(key, ext)
	return fmt.Sprintf("%s__w%dq%d.webp", base, w, q)
}

func ServeHandler(c *gin.Context) {
	key := c.Param("key")
	if strings.Contains(key, "..") || strings.Contains(key, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key"})
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
	canResize := w > 0 && (ext == ".webp" || ext == ".jpg" || ext == ".jpeg" || ext == ".png")

	if canResize {
		vKey := variantKey(key, w, q)
		vPath := filepath.Join(storagePath, vKey)

		// Serve cached variant if it exists — zero CPU cost.
		if cached, err := os.ReadFile(vPath); err == nil {
			serveBytes(c, cached, "image/webp")
			return
		}

		// Variant not cached — read original, resize, cache to disk.
		origData, err := os.ReadFile(filepath.Join(storagePath, key))
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "read error"})
			}
			return
		}

		resized, err := resizeImage(origData, w, q)
		if err != nil {
			log.Printf("WARN serve: resize %s w=%d failed (%v) — serving original", key, w, err)
			serveBytes(c, origData, mimeTypeFor(ext))
			return
		}

		// Persist variant so next request is a cache hit.
		if err := os.WriteFile(vPath, resized, 0644); err != nil {
			log.Printf("WARN serve: could not cache variant %s: %v", vKey, err)
		} else {
			log.Printf("INFO serve: cached variant %s (%d bytes)", vKey, len(resized))
		}

		serveBytes(c, resized, "image/webp")
		return
	}

	// No resize — serve original.
	data, err := os.ReadFile(filepath.Join(storagePath, key))
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read error"})
		}
		return
	}

	serveBytes(c, data, mimeTypeFor(ext))
}

func serveBytes(c *gin.Context, data []byte, ct string) {
	etag := fmt.Sprintf(`"%x"`, md5.Sum(data))
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("ETag", etag)
	c.Data(http.StatusOK, ct, data)
}

func mimeTypeFor(ext string) string {
	if ct, ok := mimeTypes[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
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

	// Remove all cached variants for this key.
	ext := filepath.Ext(key)
	base := strings.TrimSuffix(key, ext)
	prefix := base + "__w"
	entries, _ := os.ReadDir(storagePath)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			vPath := filepath.Join(storagePath, e.Name())
			if err := os.Remove(vPath); err == nil {
				log.Printf("INFO delete: removed variant %s", e.Name())
			}
		}
	}

	c.Status(http.StatusNoContent)
}
