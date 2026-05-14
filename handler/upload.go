package handler

import (
	"bytes"
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
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
// resizes to max 1200px wide and re-encodes as JPEG quality 82.
func UploadHandler(c *gin.Context) {
	if apiKey != "" && c.GetHeader("X-Api-Key") != apiKey {
		log.Printf("WARN upload: API key inválida — X-Api-Key recebida não corresponde à configurada")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

	var data []byte
	ct := c.ContentType()
	log.Printf("INFO upload: request recebida — Content-Type=%s", ct)

	if strings.HasPrefix(ct, "multipart/") {
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			log.Printf("ERROR upload: campo 'file' não encontrado no multipart: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "campo 'file' obrigatório no form-data"})
			return
		}
		defer file.Close()
		log.Printf("INFO upload: arquivo recebido — nome=%s size_header=%d", header.Filename, header.Size)
		b, err := io.ReadAll(file)
		if err != nil {
			log.Printf("ERROR upload: erro ao ler arquivo: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "erro ao ler arquivo enviado"})
			return
		}
		data = b
	} else {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("ERROR upload: erro ao ler body raw: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "erro ao ler body"})
			return
		}
		data = b
	}

	if len(data) == 0 {
		log.Printf("ERROR upload: arquivo vazio recebido")
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo vazio"})
		return
	}

	log.Printf("INFO upload: %d bytes recebidos — processando imagem", len(data))

	processed, err := processImage(data)
	var key string
	if err != nil {
		log.Printf("WARN upload: processamento de imagem falhou (%v) — salvando arquivo original", err)
		ext := detectExt(data)
		key = uuid.New().String() + ext
		dest := filepath.Join(storagePath, key)
		if err2 := os.WriteFile(dest, data, 0644); err2 != nil {
			log.Printf("ERROR upload: erro ao salvar arquivo original em %s: %v", dest, err2)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("erro ao salvar arquivo: %v", err2)})
			return
		}
		log.Printf("INFO upload: arquivo original salvo — key=%s size=%d", key, len(data))
	} else {
		key = uuid.New().String() + ".webp"
		dest := filepath.Join(storagePath, key)
		if err2 := os.WriteFile(dest, processed, 0644); err2 != nil {
			log.Printf("ERROR upload: erro ao salvar imagem processada em %s: %v", dest, err2)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("erro ao salvar imagem: %v", err2)})
			return
		}
		log.Printf("INFO upload: imagem processada — key=%s original=%dB processado=%dB (%.0f%% redução)",
			key, len(data), len(processed), float64(len(data)-len(processed))/float64(len(data))*100)
	}

	url := fmt.Sprintf("%s/%s", publicURL, key)
	log.Printf("INFO upload: sucesso — url=%s", url)
	c.JSON(http.StatusCreated, gin.H{"url": url, "key": key})
}

// processImage decodes any supported format, resizes to max 1200px wide,
// and re-encodes as WebP quality 82.
func processImage(data []byte) ([]byte, error) {
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	orig := img.Bounds()
	if orig.Dx() > maxImageWidth {
		img = imaging.Resize(img, maxImageWidth, 0, imaging.Lanczos)
		log.Printf("INFO upload: redimensionado %dx%d → %dx%d", orig.Dx(), orig.Dy(), img.Bounds().Dx(), img.Bounds().Dy())
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.Options{Lossless: false, Quality: 82}); err != nil {
		return nil, fmt.Errorf("webp encode: %w", err)
	}
	return buf.Bytes(), nil
}

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
