package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/test/mock"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	log.Println("Starting Content Moderator Service (Phase 0 - Foundation)...")

	// ============================================
	// Dependency Injection (Manual Wiring)
	// In Phase 0, we use Mock implementations
	// In Phase 1+, we'll replace these with real adapters
	// ============================================

	// 1. Create outbound adapters (currently mocks)
	log.Println("Initializing outbound adapters (mocks for Phase 0)...")
	contentRepo := mock.NewMockContentRepository()
	aiClient := mock.NewMockAIClient()
	messageBroker := mock.NewMockMessageBroker()
	cacheStore := mock.NewMockCacheStore()

	// 2. Create services with constructor injection
	log.Println("Initializing services...")
	contentSvc := service.NewContentService(contentRepo, messageBroker)
	moderationSvc := service.NewModerationService(contentRepo, aiClient, cacheStore, messageBroker)
	notificationSvc := service.NewNotificationService(messageBroker)

	// Log that services are initialized (to avoid unused variable warnings)
	log.Printf("Content Service: %T", contentSvc)
	log.Printf("Moderation Service: %T", moderationSvc)
	log.Printf("Notification Service: %T", notificationSvc)

	// ============================================
	// HTTP Server Setup
	// ============================================

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy","phase":"0","message":"Foundation phase running with mocks"}`)
	})

	// Readiness check endpoint
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ready"}`)
	})

	// Server configuration
	port := getEnv("HTTP_PORT", "8080")
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ============================================
	// Graceful Shutdown
	// ============================================

	// Run server in a goroutine
	go func() {
		log.Printf("Server starting on port %s", port)
		log.Printf("Health check: http://localhost:%s/health", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal: %v. Starting graceful shutdown...", sig)

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
