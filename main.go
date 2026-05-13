package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/Pmmvito/go-image-storage/handler"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("INFO: .env not found, using environment variables")
	}

	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = "/tmp/images"
		log.Println("WARN: STORAGE_PATH not set, using /tmp/images (not persistent)")
	}
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		log.Fatalf("FATAL: cannot create storage directory %s: %v", storagePath, err)
	}

	apiKey := os.Getenv("STORAGE_API_KEY")
	if apiKey == "" {
		log.Println("WARN: STORAGE_API_KEY not set — upload endpoint is unprotected")
	}

	publicURL := os.Getenv("PUBLIC_URL")
	if publicURL == "" {
		publicURL = "http://localhost:8080"
	}

	maxMB := 10
	if v := os.Getenv("MAX_UPLOAD_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxMB = n
		}
	}

	handler.Init(storagePath, apiKey, publicURL, int64(maxMB)<<20)

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.POST("/upload", handler.UploadHandler)
	r.DELETE("/:key", handler.DeleteHandler)
	r.GET("/:key", handler.ServeHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("INFO: storage=%s publicURL=%s maxMB=%d port=%s", storagePath, publicURL, maxMB, port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("FATAL: server error: %v", err)
	}
}
