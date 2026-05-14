package main

import (
	"context"
	"fmt"
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
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/mohammadpnp/content-moderator/api/content"
	"github.com/mohammadpnp/content-moderator/api/moderation"
	grpcadapter "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/grpc"
	httpadapter "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http"
	custommw "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http/middleware"
	wsadapter "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/websocket"
	natsadapter "github.com/mohammadpnp/content-moderator/internal/adapter/outbound/nats"
	"github.com/mohammadpnp/content-moderator/internal/adapter/outbound/postgres"
	redisadapter "github.com/mohammadpnp/content-moderator/internal/adapter/outbound/redis"
	"github.com/mohammadpnp/content-moderator/internal/pkg/logger"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/internal/worker"
	"github.com/mohammadpnp/content-moderator/test/mock"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Warn().Msg("No .env file found, using system environment variables")
	}

	// Environment
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	// Initialize zerolog
	logger.Init(env)

	// Initialize tracer
	tp, err := initTracer()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize tracer")
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Error().Err(err).Msg("Tracer shutdown error")
		}
	}()

	// Database
	db, err := postgres.NewDB()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	migrationDir := os.Getenv("MIGRATIONS_DIR")
	if migrationDir == "" {
		projectRoot, err := postgres.FindProjectRoot()
		if err != nil {
			log.Fatal().Err(err).Msg("Cannot find project root")
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
		log.Fatal().Err(err).Msg("Migration error")
	}
	log.Info().Msg("Connected to PostgreSQL")

	// NATS
	natsBroker, err := natsadapter.NewNATSBroker()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to NATS")
	}
	defer natsBroker.Close()

	// Redis
	redisClient, err := redisadapter.NewClient(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisClient.Close()
	log.Info().Msg("Connected to Redis")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// DLQ monitor
	go func() {
		if err := natsBroker.StartDLQMonitor(ctx); err != nil {
			log.Warn().Err(err).Msg("DLQ Monitor failed to start")
		}
	}()

	cacheStore := redisadapter.NewCacheStore(redisClient)

	// AI Client (mock for now)
	aiClient := mock.NewMockAIClient()

	contentRepo := postgres.NewContentRepository(db)

	// Services
	contentSvc := service.NewContentService(contentRepo, natsBroker)
	moderationSvc := service.NewModerationService(contentRepo, aiClient, cacheStore, natsBroker)

	// Worker pool
	pool := worker.NewPool(worker.DefaultConfig(), moderationSvc, natsBroker)
	go func() {
		if err := pool.Start(ctx); err != nil {
			log.Fatal().Err(err).Msg("Worker pool failed")
		}
	}()

	// WebSocket
	broadcaster := redisadapter.NewPubSubAdapter(redisClient)
	hub := wsadapter.NewHub(broadcaster)
	go func() {
		if err := hub.Run(); err != nil {
			log.Fatal().Err(err).Msg("WebSocket Hub failed")
		}
	}()
	defer hub.Shutdown()

	// Notification bridge
	notifBridge := service.NewNotificationBridge(natsBroker, broadcaster)
	go func() {
		if err := notifBridge.Start(ctx); err != nil {
			log.Fatal().Err(err).Msg("Notification Bridge failed")
		}
	}()

	// HTTP server (Fiber)
	app := fiber.New(fiber.Config{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	})

	app.Use(requestid.New())
	app.Use(fiberlogger.New(fiberlogger.Config{
		Format: "${pid} ${locals:requestid} ${status} - ${method} ${path} ${latency}\n",
	}))
	app.Use(recover.New())
	app.Use(custommw.TimeoutMiddleware(30 * time.Second))
	app.Use(custommw.StructuredLoggerMiddleware())
	app.Use(custommw.TracingMiddleware())
	app.Use(custommw.PrometheusMiddleware())
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
		log.Info().Str("port", port).Msg("Starting HTTP server")
		if err := app.Listen(":" + port); err != nil {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// gRPC server
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
			log.Fatal().Err(err).Msg("Failed to listen on gRPC port")
		}
		log.Info().Str("port", grpcPort).Msg("Starting gRPC server")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	// pprof
	pprofPort := os.Getenv("PPROF_PORT")
	if pprofPort == "" {
		pprofPort = "6060"
	}
	go func() {
		log.Info().Str("port", pprofPort).Msg("pprof server listening")
		if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
			log.Error().Err(err).Msg("pprof server error")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("Received signal, shutting down...")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP forced shutdown")
	}

	grpcServer.GracefulStop()

	log.Info().Msg("All services stopped")
}

func initTracer() (*sdktrace.TracerProvider, error) {
	jaegerAddr := os.Getenv("JAEGER_ENDPOINT")
	if jaegerAddr == "" {
		jaegerAddr = "jaeger:4317"
	}
	exporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(jaegerAddr),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("content-moderator"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp, nil
}
