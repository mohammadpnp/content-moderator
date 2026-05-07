package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
)

const (
	// Buffer size for outgoing messages per client (protects against slow consumers)
	sendBufferSize = 256
	// Redis channel on which real‑time notifications are published
	redisChannel = "moderation:notifications"
)

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID string
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[string]map[*Client]bool // userID -> set of clients
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMsg

	broadcaster outbound.RealtimeBroadcaster
	ctx         context.Context
	cancel      context.CancelFunc
}

type broadcastMsg struct {
	userID  string
	message []byte
}

func NewHub(broadcaster outbound.RealtimeBroadcaster) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:     make(map[string]map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcast:   make(chan broadcastMsg, 100), // small buffer
		broadcaster: broadcaster,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// (inside hub.go, after Client definition)

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512) // maximum message size (for control messages)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		// We don't expect client messages, but we read to keep connection alive.
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Channel closed by hub, send close message and exit
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Run starts the Hub event loop and subscribes to Redis notifications.
// It blocks until ctx is cancelled.
func (h *Hub) Run() error {
	// Subscribe to Redis for real‑time notifications
	if err := h.broadcaster.Subscribe(h.ctx, redisChannel, h.handleRedisMessage); err != nil {
		return err
	}

	log.Println("WebSocket Hub started")
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.userID]; !ok {
				h.clients[client.userID] = make(map[*Client]bool)
			}
			h.clients[client.userID][client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected: user=%s total=%d", client.userID, len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.userID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
				}
				if len(clients) == 0 {
					delete(h.clients, client.userID)
				}
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected: user=%s", client.userID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			clients := h.clients[msg.userID]
			// Make a safe copy of clients slice to avoid locking during send
			snapshot := make([]*Client, 0, len(clients))
			for c := range clients {
				snapshot = append(snapshot, c)
			}
			h.mu.RUnlock()

			for _, c := range snapshot {
				h.sendToClient(c, msg.message)
			}

		case <-h.ctx.Done():
			log.Println("WebSocket Hub shutting down")
			return nil
		}
	}
}

// SendToUser enqueues a message to all connections of a given user.
func (h *Hub) SendToUser(userID string, message []byte) {
	h.broadcast <- broadcastMsg{userID: userID, message: message}
}

// sendToClient attempts to deliver a message to a single client.
// If the client's send buffer is full, the client is disconnected (slow consumer).
func (h *Hub) sendToClient(c *Client, msg []byte) {
	select {
	case c.send <- msg:
	default: // buffer full – treat as slow consumer
		log.Printf("Slow consumer detected, disconnecting user=%s", c.userID)
		close(c.send)
		// Remove from hub; connection will eventually be closed by readPump
		h.mu.Lock()
		if clients, ok := h.clients[c.userID]; ok {
			delete(clients, c)
			if len(clients) == 0 {
				delete(h.clients, c.userID)
			}
		}
		h.mu.Unlock()
	}
}

// handleRedisMessage is called for each message received on the Redis subscription.
func (h *Hub) handleRedisMessage(payload []byte) {
	// The payload is JSON of type RealtimeNotification (defined later)
	// We need to parse it to extract userId.
	// For now we'll parse a small struct.
	// (Definition below in the same package)
	var notif RealtimeNotification
	if err := json.Unmarshal(payload, &notif); err != nil {
		log.Printf("Hub: failed to unmarshal redis message: %v", err)
		return
	}
	h.SendToUser(notif.UserID, payload) // forward the raw message (or a formatted one)
}

// Shutdown gracefully stops the Hub.
func (h *Hub) Shutdown() {
	h.cancel()
}
