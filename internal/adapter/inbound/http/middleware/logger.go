package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func StructuredLoggerMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		traceID := c.Locals("requestid")
		if traceID == nil {
			traceID = "unknown"
		}
		logger := log.With().Str("trace_id", traceID.(string)).Logger()
		c.Locals("logger", &logger)
		return c.Next()
	}
}

func GetLogger(c *fiber.Ctx) *zerolog.Logger {
	if l, ok := c.Locals("logger").(*zerolog.Logger); ok {
		return l
	}
	return &log.Logger
}
