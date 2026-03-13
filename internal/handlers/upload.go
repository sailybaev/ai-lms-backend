package handlers

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	UploadDir string
}

func NewUploadHandler(uploadDir string) *UploadHandler {
	return &UploadHandler{UploadDir: uploadDir}
}

var allowedMimeTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/gif":  "gif",
	"image/webp": "webp",
}

const maxUploadSize = 5 * 1024 * 1024 // 5MB

func generateFilename(ext string) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}
	return fmt.Sprintf("%d-%s.%s", time.Now().Unix(), string(b), ext)
}

// UploadPhoto handles POST /api/org/:org/admin/photo and POST /api/org/:org/profile/photo
func (h *UploadHandler) UploadPhoto(c *gin.Context) {
	// Limit file size
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize+1024)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read file from request"})
		return
	}
	defer file.Close()

	// Check file size
	if header.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large. Maximum size is 5MB"})
		return
	}

	// Read first 512 bytes to detect MIME type
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}
	buf = buf[:n]

	mimeType := http.DetectContentType(buf)
	// Trim parameters from MIME type (e.g. "image/jpeg; charset=utf-8" -> "image/jpeg")
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	ext, ok := allowedMimeTypes[mimeType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only JPEG, PNG, GIF, and WebP are allowed"})
		return
	}

	// Ensure upload directory exists
	if err := os.MkdirAll(h.UploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	filename := generateFilename(ext)
	destPath := filepath.Join(h.UploadDir, filename)

	// Create destination file
	dst, err := os.Create(destPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer dst.Close()

	// Write the already-read bytes first
	if _, err := dst.Write(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file"})
		return
	}

	// Copy remaining bytes
	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file"})
		return
	}

	avatarURL := fmt.Sprintf("/uploads/avatars/%s", filename)
	c.JSON(http.StatusOK, gin.H{
		"message":   "File uploaded successfully",
		"avatarUrl": avatarURL,
	})
}
