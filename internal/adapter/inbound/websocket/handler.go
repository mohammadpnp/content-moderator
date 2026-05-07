// internal/adapter/inbound/websocket/handler.go
package websocket

import (
	"log"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// NewWSHandler returns a Fiber handler that upgrades an HTTP connection to WebSocket.
// Expects a "token" query parameter with a JWT (or mock valid‑ prefix).
func NewWSHandler(hub *Hub) fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		token := c.Query("token")
		userID, err := authenticate(token)
		if err != nil {
			log.Printf("WS auth failed: %v", err)
			c.Close()
			return
		}
		client := &Client{
			hub:    hub,
			conn:   c,
			send:   make(chan []byte, sendBufferSize),
			userID: userID,
		}
		hub.register <- client

		// Start writer/reader goroutines
		go client.writePump()
		client.readPump()

		// After readPump returns (connection closed), unregister
		hub.unregister <- client
	})
}

// authenticate is the same simplified logic used in gRPC interceptor.
// Replace with real JWT validation in production.
func authenticate(token string) (string, error) {
	if strings.HasPrefix(token, "valid-") {
		return strings.TrimPrefix(token, "valid-"), nil
	}
	// TODO: real JWT parsing
	return "", fiber.NewError(http.StatusUnauthorized, "invalid token")
}
