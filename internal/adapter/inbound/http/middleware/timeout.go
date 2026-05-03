package middleware

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TimeoutMiddleware(timeout time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), timeout)
		defer cancel()

		c.SetUserContext(ctx)
		return c.Next()
	}
}
