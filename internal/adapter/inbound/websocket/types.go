package websocket

type RealtimeNotification struct {
	UserID    string `json:"user_id"`
	ContentID string `json:"content_id"`
	Type      string `json:"type"` // "approved" or "rejected"
	Message   string `json:"message"`
}
