package entity

import (
	"errors"
	"time"
)

type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeImage ContentType = "image"
)

func (ct ContentType) Validate() error {
	switch ct {
	case ContentTypeText, ContentTypeImage:
		return nil
	default:
		return errors.New("invalid content type: must be 'text' or 'image'")
	}
}

type ContentStatus string

const (
	ContentStatusPending  ContentStatus = "pending"
	ContentStatusApproved ContentStatus = "approved"
	ContentStatusRejected ContentStatus = "rejected"
)

type Content struct {
	ID           string        `json:"id"`
	UserID       string        `json:"user_id"`
	Type         ContentType   `json:"type"`
	Body         string        `json:"body"`
	Status       ContentStatus `json:"status"`
	ModerationID *string       `json:"moderation_id,omitempty"` // nullable
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

func NewContent(userID string, contentType ContentType, body string) (*Content, error) {
	if err := contentType.Validate(); err != nil {
		return nil, err
	}

	if body == "" {
		return nil, errors.New("content body cannot be empty")
	}

	// Validate body length (max 10000 characters as mentioned in docs)
	if len(body) > 10000 {
		return nil, errors.New("content body exceeds maximum length of 10000 characters")
	}

	now := time.Now()
	return &Content{
		ID:        "", // Will be set by repository
		UserID:    userID,
		Type:      contentType,
		Body:      body,
		Status:    ContentStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (c *Content) IsPending() bool {
	return c.Status == ContentStatusPending
}

func (c *Content) IsApproved() bool {
	return c.Status == ContentStatusApproved
}

func (c *Content) IsRejected() bool {
	return c.Status == ContentStatusRejected
}
