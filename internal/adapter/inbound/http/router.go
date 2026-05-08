package http

import (
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/mohammadpnp/content-moderator/internal/adapter/inbound/http/middleware"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/inbound"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRoutes(app *fiber.App, contentSvc inbound.ContentService) {
	handler := NewContentHandler(contentSvc)

	v1 := app.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware())
	v1.Post("/contents", handler.Create)
	v1.Get("/contents/:id", handler.Get)
	v1.Delete("/contents/:id", handler.Delete)

	v1.Get("/users/:userID/contents", handler.List)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy", "phase": "1"})
	})
	app.Get("/ready", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ready"})
	})

	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
}
