package websocket_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ws "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/websocket"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/test/mock"
)

func startTestServer(t *testing.T, app *fiber.App) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		if err := app.Listener(listener); err != nil {
			t.Logf("server exit: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	url := fmt.Sprintf("ws://127.0.0.1:%d", port)
	return url, func() {
		_ = app.Shutdown()
	}
}

func setupHub(t *testing.T) (*ws.Hub, *mock.MockRealtimeBroadcaster) {
	t.Helper()
	broadcaster := mock.NewMockRealtimeBroadcaster()
	hub := ws.NewHub(broadcaster)
	go func() {
		_ = hub.Run()
	}()
	time.Sleep(50 * time.Millisecond)
	return hub, broadcaster
}

func newTestApp(hub *ws.Hub) *fiber.App {
	app := fiber.New()
	app.Get("/ws", ws.NewWSHandler(hub))
	return app
}

func connectWS(t *testing.T, baseURL, token string) (*websocket.Conn, func()) {
	t.Helper()
	url := baseURL + "/ws?token=" + token
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(url, nil)
	require.NoError(t, err)
	return conn, func() { conn.Close() }
}

// ========== تست‌ها ==========

func TestHub_ClientConnection_ValidToken(t *testing.T) {
	hub, _ := setupHub(t)
	app := newTestApp(hub)
	baseURL, serverCleanup := startTestServer(t, app)
	defer serverCleanup()

	conn, connCleanup := connectWS(t, baseURL, "valid-testuser")
	defer connCleanup()
	time.Sleep(100 * time.Millisecond)

	err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second))
	assert.NoError(t, err)
}

func TestHub_ClientConnection_InvalidToken(t *testing.T) {
	hub, _ := setupHub(t)
	app := newTestApp(hub)
	baseURL, serverCleanup := startTestServer(t, app)
	defer serverCleanup()

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(baseURL+"/ws?token=bad-token", nil)
	if err == nil {
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, _, err = conn.ReadMessage()
	}
	assert.Error(t, err, "server should close connection after bad auth")
}

func TestHub_SendToUser_DeliversMessage(t *testing.T) {
	hub, _ := setupHub(t)
	app := newTestApp(hub)
	baseURL, serverCleanup := startTestServer(t, app)
	defer serverCleanup()

	conn, connCleanup := connectWS(t, baseURL, "valid-user123")
	defer connCleanup()
	time.Sleep(100 * time.Millisecond)

	hub.SendToUser("user123", []byte(`{"test":"hello"}`))

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, `{"test":"hello"}`, string(msg))
}

func TestHub_SlowConsumer_Disconnects(t *testing.T) {
	hub, _ := setupHub(t)
	app := newTestApp(hub)
	baseURL, serverCleanup := startTestServer(t, app)
	defer serverCleanup()

	conn, connCleanup := connectWS(t, baseURL, "valid-slow")
	defer connCleanup()
	time.Sleep(100 * time.Millisecond)

	// Fill buffer without reading
	for i := 0; i < 300; i++ {
		hub.SendToUser("slow", []byte("msg"))
	}

	// Read until we get an error (close frame or broken pipe)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var readErr error
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			readErr = err
			break
		}
	}
	assert.Error(t, readErr, "expected disconnection due to slow consumer")
}

// ========== تست جریان کامل بدون WebSocket واقعی ==========
func TestFullFlow_NotificationBridgeToBroadcaster(t *testing.T) {
	repo := mock.NewMockContentRepository()
	aiClient := mock.NewMockAIClient()
	aiClient.SetAsApproving()
	aiClient.SetDelay(0)
	cache := mock.NewMockCacheStore()
	broker := mock.NewMockMessageBroker()
	broadcaster := mock.NewMockRealtimeBroadcaster()

	contentSvc := service.NewContentService(repo, broker)
	moderationSvc := service.NewModerationService(repo, aiClient, cache, broker)

	hub := ws.NewHub(broadcaster)
	go hub.Run()
	defer hub.Shutdown()
	time.Sleep(50 * time.Millisecond)

	// استفاده از یک WaitGroup برای هماهنگی
	var wg sync.WaitGroup
	wg.Add(1)

	// جایگزین WebSocket Client: یک مصرف‌کننده که از Broadcaster شنود می‌کند
	received := make(chan ws.RealtimeNotification, 1)
	err := broadcaster.Subscribe(context.Background(), "moderation:notifications", func(payload []byte) {
		var notif ws.RealtimeNotification
		if err := json.Unmarshal(payload, &notif); err == nil {
			received <- notif
		}
		wg.Done()
	})
	require.NoError(t, err)

	bridge := service.NewNotificationBridge(broker, broadcaster)
	go bridge.Start(context.Background())
	time.Sleep(50 * time.Millisecond)

	content, err := contentSvc.CreateContent(context.Background(), "userXYZ", entity.ContentTypeText, "Hello, test!")
	require.NoError(t, err)

	result, err := moderationSvc.ModerateContent(context.Background(), content.ID)
	require.NoError(t, err)

	err = moderationSvc.HandleModerationResult(context.Background(), result)
	require.NoError(t, err)

	select {
	case notif := <-received:
		assert.Equal(t, "userXYZ", notif.UserID)
		assert.Equal(t, content.ID, notif.ContentID)
		assert.Equal(t, "approved", notif.Type)
		assert.Contains(t, notif.Message, "approved")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}
