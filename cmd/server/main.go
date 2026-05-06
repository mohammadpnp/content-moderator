// cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/mohammadpnp/content-moderator/api/content"
	"github.com/mohammadpnp/content-moderator/api/moderation"
	grpcadapter "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/grpc"
	"github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http"
	custommiddleware "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http/middleware"
	"github.com/mohammadpnp/content-moderator/internal/adapter/outbound/postgres"
	pgrepo "github.com/mohammadpnp/content-moderator/internal/adapter/outbound/postgres"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/test/mock"
	"go.opentelemetry.io/otel"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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

	migrationDir := os.Getenv("MIGRATIONS_DIR")
	if migrationDir == "" {
		projectRoot, err := postgres.FindProjectRoot()
		if err != nil {
			log.Fatalf("Cannot find project root: %v", err)
		}
		migrationDir = filepath.Join(projectRoot, "deploy", "migrations")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)

	if err := postgres.RunMigrations(migrationDir, dsn); err != nil {
		log.Fatalf("Migration error: %v", err)
	}

	log.Println("Connected to PostgreSQL")

	// Tracer
	tracerProvider, err := initTracer()
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer tracerProvider.Shutdown(context.Background())

	// Repositories
	contentRepo := pgrepo.NewContentRepository(db)

	// Message broker (mock for now)
	broker := mock.NewMockMessageBroker()

	// Cache (mock for now)
	cacheStore := mock.NewMockCacheStore()

	// AI Client
	// TODO: Replace with real TritonClient + CircuitBreaker when TRITON_HOST is set
	aiClient := mock.NewMockAIClient()
	// tritonHost := os.Getenv("TRITON_HOST")
	// if tritonHost != "" {
	//     tc, err := tritonadapter.NewTritonClient(tritonHost + ":8001")
	//     if err != nil {
	//         log.Fatalf("Failed to create Triton client: %v", err)
	//     }
	//     aiClient = tritonadapter.NewCircuitBreakerAIClient(tc)
	// }

	// Services
	contentSvc := service.NewContentService(contentRepo, broker)
	moderationSvc := service.NewModerationService(contentRepo, aiClient, cacheStore, broker)

	// ========================
	// HTTP server (Fiber)
	// ========================
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

	http.SetupRoutes(app, contentSvc)

	// ========================
	// gRPC server
	// ========================
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcadapter.RecoveryUnaryInterceptor(),
			grpcadapter.LoggingUnaryInterceptor(),
			grpcadapter.MetricsUnaryInterceptor(),
			grpcadapter.AuthUnaryInterceptor(),
			grpcadapter.TracingUnaryInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			grpcadapter.RecoveryStreamInterceptor(),
			grpcadapter.LoggingStreamInterceptor(),
			grpcadapter.MetricsStreamInterceptor(),
			grpcadapter.AuthStreamInterceptor(),
			grpcadapter.TracingStreamInterceptor(),
		),
	)

	// Register gRPC services
	content.RegisterContentServiceServer(grpcServer, grpcadapter.NewContentServer(contentSvc, moderationSvc))
	moderation.RegisterModerationServiceServer(grpcServer, grpcadapter.NewModerationServer(moderationSvc))

	// Enable reflection for debugging tools like grpcurl
	reflection.Register(grpcServer)

	// ========================
	// Start servers
	// ========================
	go func() {
		port := os.Getenv("HTTP_PORT")
		if port == "" {
			port = "8080"
		}
		log.Printf("Starting Fiber HTTP server on port %s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	go func() {
		grpcPort := os.Getenv("GRPC_PORT")
		if grpcPort == "" {
			grpcPort = "9090"
		}
		lis, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			log.Fatalf("Failed to listen on gRPC port: %v", err)
		}
		log.Printf("Starting gRPC server on port %s", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// ========================
	// Graceful shutdown
	// ========================
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Fatalf("HTTP forced shutdown: %v", err)
	}
	grpcServer.GracefulStop()
	log.Println("Both servers exited gracefully")
}

func initTracer() (*sdktrace.TracerProvider, error) {
	exporter, err := stdout.New(stdout.WithPrettyPrint())
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp, nil
}
