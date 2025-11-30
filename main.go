package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ServerwaveHost/wave-mc-jars-api/internal/cache"
	"github.com/ServerwaveHost/wave-mc-jars-api/internal/handlers"
	"github.com/ServerwaveHost/wave-mc-jars-api/internal/providers"
	"github.com/ServerwaveHost/wave-mc-jars-api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if exists
	_ = godotenv.Load()

	// Get port from environment or default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Set Gin mode
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize cache
	cacheConfig := cache.DefaultConfig()
	c, err := cache.New(cacheConfig)
	if err != nil {
		log.Printf("Warning: Cache initialization error: %v", err)
	}
	defer func() {
		_ = c.Close()
	}()

	// Initialize provider registry
	providerConfig := providers.DefaultConfig()
	registry := providers.NewRegistry(providerConfig)

	// Initialize service
	svc := service.NewJarsService(registry, c)

	// Initialize handlers
	h := handlers.NewHandler(svc)

	// Setup router
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Accept, Content-Type")
		c.Header("Access-Control-Max-Age", "300")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// Health routes
	r.GET("/", h.HealthCheck)
	r.GET("/health", h.HealthCheck)

	// Categories
	r.GET("/categories", h.GetCategories)
	r.GET("/categories/:category", h.GetCategory)
	r.GET("/categories/:category/versions", h.GetVersions)
	r.GET("/categories/:category/versions/:version/builds", h.GetBuilds)
	r.GET("/categories/:category/versions/:version/builds/:build", h.GetBuild)
	r.GET("/categories/:category/versions/:version/builds/:build/download", h.GetDownload)

	// Search
	r.GET("/search", h.Search)

	// Create server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
