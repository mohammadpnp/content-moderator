package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/joho/godotenv"

	"github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http"
	custommiddleware "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http/middleware"
	pgrepo "github.com/mohammadpnp/content-moderator/internal/adapter/outbound/postgres"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/test/mock"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	db, err := pgrepo.NewDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	contentRepo := pgrepo.NewContentRepository(db)

	broker := mock.NewMockMessageBroker()
	contentSvc := service.NewContentService(contentRepo, broker)

	// ساخت Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	})

	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "${pid} ${locals:requestid} ${status} - ${method} ${path} ${latency}\n",
	}))
	app.Use(recover.New())
	app.Use(custommiddleware.TimeoutMiddleware(30 * time.Second))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// ثبت مسیرها
	http.SetupRoutes(app, contentSvc)

	// Graceful shutdown
	go func() {
		port := os.Getenv("HTTP_PORT")
		if port == "" {
			port = "8080"
		}
		log.Printf("Starting Fiber server on port %s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Fatalf("Forced to shutdown: %v", err)
	}
	log.Println("Server exited gracefully")
}
