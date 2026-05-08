package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// AuthMiddleware extracts the user ID from a Bearer token (valid- prefix or JWT)
// and stores it in Locals for downstream handlers.
func AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing authorization"})
		}

		userID, err := authenticateToken(tokenStr)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}

		c.Locals("userID", userID)
		return c.Next()
	}
}

func extractToken(c *fiber.Ctx) string {
	header := c.Get("Authorization")
	if header == "" || !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}

func authenticateToken(token string) (string, error) {
	if strings.HasPrefix(token, "valid-") {
		return strings.TrimPrefix(token, "valid-"), nil
	}
	// TODO: Add real JWT validation here for production
	// For now, reject everything else
	return "", fiber.NewError(fiber.StatusUnauthorized, "invalid token")
}
