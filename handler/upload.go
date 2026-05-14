package handler

import (
	"bytes"
	"fmt"
	"io"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

var (
	storagePath string
	apiKey      string
	publicURL   string
	maxBytes    int64
)

const maxImageWidth = 1200

func Init(path, key, url string, max int64) {
	storagePath = path
	apiKey = key
	publicURL = strings.TrimRight(url, "/")
	maxBytes = max
}

// UploadHandler accepts multipart/form-data (field "file") or raw body,
// converts image to WebP (max 1200px wide, quality 82), saves and returns URL.
func UploadHandler(c *gin.Context) {
	if apiKey != "" && c.GetHeader("X-Api-Key") != apiKey {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

	var data []byte
	if strings.HasPrefix(c.ContentType(), "multipart/") {
		file, _, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "field 'file' required"})
			return
		}
		defer file.Close()
		b, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
			return
		}
		data = b
	} else {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return
		}
		data = b
	}

	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty file"})
		return
	}

	processed, err := toWebP(data)
	var (
		key  string
		dest string
	)
	if err != nil {
		// not a recognized image — save original with detected extension
		ext := detectExt(data)
		key = uuid.New().String() + ext
		dest = filepath.Join(storagePath, key)
		if err2 := os.WriteFile(dest, data, 0644); err2 != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
			return
		}
	} else {
		key = uuid.New().String() + ".webp"
		dest = filepath.Join(storagePath, key)
		if err2 := os.WriteFile(dest, processed, 0644); err2 != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
			return
		}
	}

	url := fmt.Sprintf("%s/%s", publicURL, key)
	c.JSON(http.StatusCreated, gin.H{"url": url, "key": key})
}

// toWebP decodes any supported image format, resizes to max 1200px wide
// (preserving aspect ratio), and encodes as WebP quality 82.
func toWebP(data []byte) ([]byte, error) {
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}

	if img.Bounds().Dx() > maxImageWidth {
		img = imaging.Resize(img, maxImageWidth, 0, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// detectExt returns file extension from magic bytes.
func detectExt(data []byte) string {
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
	return ".bin"
}
