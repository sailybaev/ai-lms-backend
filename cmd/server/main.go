package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/yourusername/ai-lms-backend/internal/config"
	"github.com/yourusername/ai-lms-backend/internal/database"
	"github.com/yourusername/ai-lms-backend/internal/routes"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg := config.Load()

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	// Connect to database
	db := database.Connect(cfg.DatabaseURL)

	// Run auto-migrations
	database.AutoMigrate(db)

	// Ensure upload directories exist
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		log.Printf("Warning: could not create upload directory %s: %v", cfg.UploadDir, err)
	}

	// Set up Gin router
	router := gin.Default()

	// Trust all proxies (adjust for production)
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Printf("Warning: could not set trusted proxies: %v", err)
	}

	// CORS middleware (simple, adjust origins for production)
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Register all routes
	routes.Setup(router, db, cfg)

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("AI LMS Backend server starting on %s", addr)
	log.Printf("Upload directory: %s", cfg.UploadDir)
	log.Printf("AI Base URL: %s", cfg.AIBaseURL)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
