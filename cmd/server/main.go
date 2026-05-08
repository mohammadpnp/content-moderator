package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
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
	httpadapter "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http"
	custommiddleware "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http/middleware"
	wsadapter "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/websocket"
	natsadapter "github.com/mohammadpnp/content-moderator/internal/adapter/outbound/nats"
	"github.com/mohammadpnp/content-moderator/internal/adapter/outbound/postgres"
	redisadapter "github.com/mohammadpnp/content-moderator/internal/adapter/outbound/redis"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/internal/worker"
	"github.com/mohammadpnp/content-moderator/test/mock"

	"go.opentelemetry.io/otel"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	// ── ۱. بارگذاری تنظیمات ──────────────────────────────────────
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// ── ۲. راه‌اندازی Tracer ─────────────────────────────────────
	tracerProvider, err := initTracer()
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tracerProvider.Shutdown(context.Background()); err != nil {
			log.Printf("Tracer shutdown error: %v", err)
		}
	}()

	// ── ۳. اتصال به PostgreSQL و اجرای Migration ─────────────────
	db, err := postgres.NewDB()
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

	// ── ۴. راه‌اندازی سرویس‌های زیرساختی ─────────────────────────
	// NATS
	natsBroker, err := natsadapter.NewNATSBroker()
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer natsBroker.Close()

	// Redis client
	redisClient, err := redisadapter.NewClient(context.Background())
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()
	log.Println("Connected to Redis")

	// Context کلی برنامه (برای انتشار سیگنال خاموشی)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// مانیتورینگ Dead Letter Queue
	go func() {
		if err := natsBroker.StartDLQMonitor(ctx); err != nil {
			log.Printf("Warning: DLQ Monitor failed to start: %v", err)
		}
	}()

	// Cache (Redis)
	cacheStore := redisadapter.NewCacheStore(redisClient)

	// AI Client (فعلاً Mock)
	aiClient := mock.NewMockAIClient()
	// TODO: در آینده با TritonClient جایگزین شود

	// Repository
	contentRepo := postgres.NewContentRepository(db)

	// ── ۵. سرویس‌های دامنه ───────────────────────────────────────
	contentSvc := service.NewContentService(contentRepo, natsBroker)
	moderationSvc := service.NewModerationService(contentRepo, aiClient, cacheStore, natsBroker)

	// ── ۶. راه‌اندازی Worker Pool ────────────────────────────────
	pool := worker.NewPool(worker.DefaultConfig(), moderationSvc, natsBroker)
	go func() {
		if err := pool.Start(ctx); err != nil {
			log.Fatalf("Worker pool failed: %v", err)
		}
	}()

	// ── Realtime Broadcaster (Redis Pub/Sub) ─────────────────
	broadcaster := redisadapter.NewPubSubAdapter(redisClient)

	// ── WebSocket Hub ────────────────────────────────────────
	hub := wsadapter.NewHub(broadcaster)
	go func() {
		if err := hub.Run(); err != nil {
			log.Fatalf("WebSocket Hub failed: %v", err)
		}
	}()
	defer hub.Shutdown()

	// ── Notification Bridge ─────────────────────────────────
	notifBridge := service.NewNotificationBridge(natsBroker, broadcaster)
	go func() {
		if err := notifBridge.Start(ctx); err != nil {
			log.Fatalf("Notification Bridge failed: %v", err)
		}
	}()

	// ── ۷. راه‌اندازی HTTP Server (Fiber) ────────────────────────
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

	httpadapter.SetupRoutes(app, contentSvc)
	app.Get("/ws", wsadapter.NewWSHandler(hub))

	go func() {
		port := os.Getenv("HTTP_PORT")
		if port == "" {
			port = "8080"
		}
		log.Printf("Starting HTTP server on :%s", port)
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// ── ۸. راه‌اندازی gRPC Server ────────────────────────────────
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

	content.RegisterContentServiceServer(grpcServer, grpcadapter.NewContentServer(contentSvc, moderationSvc))
	moderation.RegisterModerationServiceServer(grpcServer, grpcadapter.NewModerationServer(moderationSvc))
	reflection.Register(grpcServer)

	go func() {
		grpcPort := os.Getenv("GRPC_PORT")
		if grpcPort == "" {
			grpcPort = "9090"
		}
		lis, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			log.Fatalf("Failed to listen on gRPC port: %v", err)
		}
		log.Printf("Starting gRPC server on :%s", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// ── pprof server ─────────────────────────────────────────────
	pprofPort := os.Getenv("PPROF_PORT")
	if pprofPort == "" {
		pprofPort = "6060"
	}
	go func() {
		log.Printf("pprof server listening on :%s", pprofPort)
		// pprof handlers روی DefaultServeMux ثبت شدن (با blank import)
		if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()

	// ── ۹. Graceful Shutdown ─────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %v, shutting down...", sig)

	cancel() // اعلام توقف به Worker Pool و DLQ Monitor

	// توقف HTTP با مهلت
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("HTTP forced shutdown: %v", err)
	}

	// توقف gRPC
	grpcServer.GracefulStop()

	log.Println("All services stopped")
}

// ── تابع راه‌انداز Tracer ────────────────────────────────────────
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
