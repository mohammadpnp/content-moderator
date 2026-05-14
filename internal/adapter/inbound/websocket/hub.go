package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
	"github.com/mohammadpnp/content-moderator/internal/pkg/metrics"
	"github.com/rs/zerolog/log"
)

const (
	sendBufferSize = 256
	redisChannel   = "moderation:notifications"
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
		broadcast:   make(chan broadcastMsg, 100),
		broadcaster: broadcaster,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Warn().Err(err).Msg("WebSocket read error")
			}
			break
		}
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
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Warn().Err(err).Msg("WebSocket write error")
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

func (h *Hub) Run() error {
	if err := h.broadcaster.Subscribe(h.ctx, redisChannel, h.handleRedisMessage); err != nil {
		return err
	}

	log.Info().Msg("WebSocket Hub started")
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			metrics.ActiveWsConnections.Inc()
			if _, ok := h.clients[client.userID]; !ok {
				h.clients[client.userID] = make(map[*Client]bool)
			}
			h.clients[client.userID][client] = true
			h.mu.Unlock()
			log.Info().Str("user_id", client.userID).Int("total_clients", len(h.clients)).Msg("WebSocket client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			metrics.ActiveWsConnections.Dec()
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
			log.Info().Str("user_id", client.userID).Msg("WebSocket client disconnected")

		case msg := <-h.broadcast:
			h.mu.RLock()
			clients := h.clients[msg.userID]
			snapshot := make([]*Client, 0, len(clients))
			for c := range clients {
				snapshot = append(snapshot, c)
			}
			h.mu.RUnlock()

			for _, c := range snapshot {
				h.sendToClient(c, msg.message)
			}

		case <-h.ctx.Done():
			log.Info().Msg("WebSocket Hub shutting down")
			return nil
		}
	}
}

func (h *Hub) SendToUser(userID string, message []byte) {
	h.broadcast <- broadcastMsg{userID: userID, message: message}
}

func (h *Hub) sendToClient(c *Client, msg []byte) {
	select {
	case c.send <- msg:
	default:
		log.Warn().Str("user_id", c.userID).Msg("slow consumer detected, disconnecting")
		close(c.send)
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

func (h *Hub) handleRedisMessage(payload []byte) {
	var notif RealtimeNotification
	if err := json.Unmarshal(payload, &notif); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal redis message")
		return
	}
	h.SendToUser(notif.UserID, payload)
}

func (h *Hub) Shutdown() {
	h.cancel()
}
