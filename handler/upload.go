package handler

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	storagePath string
	apiKey      string
	publicURL   string
	maxBytes    int64
)

func Init(path, key, url string, max int64) {
	storagePath = path
	apiKey = key
	publicURL = strings.TrimRight(url, "/")
	maxBytes = max
}

// UploadHandler accepts multipart/form-data (field "file") or raw body.
func UploadHandler(c *gin.Context) {
	if apiKey != "" && c.GetHeader("X-Api-Key") != apiKey {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
		return
	}

	var (
		data []byte
		ext  string
	)

	ct := c.ContentType()
	if strings.HasPrefix(ct, "multipart/") {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "field 'file' required"})
			return
		}
		defer file.Close()
		ext = filepath.Ext(header.Filename)
		b, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
			return
		}
		data = b
	} else {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return
		}
		data = b
		exts, _ := mime.ExtensionsByType(ct)
		if len(exts) > 0 {
			ext = exts[0]
		}
	}

	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty file"})
		return
	}

	ext = normalizeExt(ext, data)
	key := uuid.New().String() + ext
	dest := filepath.Join(storagePath, key)

	if err := os.WriteFile(dest, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	url := fmt.Sprintf("%s/%s", publicURL, key)
	c.JSON(http.StatusCreated, gin.H{"url": url, "key": key})
}

// normalizeExt resolves missing or wrong extensions using magic bytes.
func normalizeExt(ext string, data []byte) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".svg", ".bmp", ".ico":
		return ext
	}
	// detect by magic bytes
	if len(data) >= 4 {
		switch {
		case data[0] == 0xFF && data[1] == 0xD8:
			return ".jpg"
		case data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G':
			return ".png"
		case data[0] == 'G' && data[1] == 'I' && data[2] == 'F':
			return ".gif"
		case data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F':
			return ".webp"
		}
	}
	return ext
}
