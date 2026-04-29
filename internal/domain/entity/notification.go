package entity

import (
	"errors"
	"time"
)

type NotificationType string

const (
	NotificationApproved NotificationType = "approved"
	NotificationRejected NotificationType = "rejected"
)

func (nt NotificationType) Validate() error {
	switch nt {
	case NotificationApproved, NotificationRejected:
		return nil
	default:
		return errors.New("invalid notification type")
	}
}

type Notification struct {
	ID        string           `json:"id"`
	UserID    string           `json:"user_id"`
	ContentID string           `json:"content_id"`
	Type      NotificationType `json:"type"`
	Message   string           `json:"message"`
	CreatedAt time.Time        `json:"created_at"`
	ReadAt    *time.Time       `json:"read_at,omitempty"` // nullable
}

func NewNotification(userID, contentID string, notifType NotificationType, message string) (*Notification, error) {
	if userID == "" {
		return nil, errors.New("user ID cannot be empty")
	}

	if contentID == "" {
		return nil, errors.New("content ID cannot be empty")
	}

	if err := notifType.Validate(); err != nil {
		return nil, err
	}

	if message == "" {
		return nil, errors.New("message cannot be empty")
	}

	return &Notification{
		ID:        "", // Will be set by repository
		UserID:    userID,
		ContentID: contentID,
		Type:      notifType,
		Message:   message,
		CreatedAt: time.Now(),
	}, nil
}
