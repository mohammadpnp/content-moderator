package entity

import (
	"errors"
	"time"
)

type ModerationCategory string

const (
	CategoryHate     ModerationCategory = "hate"
	CategorySpam     ModerationCategory = "spam"
	CategoryAdult    ModerationCategory = "adult"
	CategoryViolence ModerationCategory = "violence"
)

type ModerationResult struct {
	ID         string               `json:"id"`
	ContentID  string               `json:"content_id"`
	IsApproved bool                 `json:"is_approved"`
	Score      float64              `json:"score"`
	Categories []ModerationCategory `json:"categories"`
	ModelName  string               `json:"model_name"`
	DurationMs int64                `json:"duration_ms"`
	CreatedAt  time.Time            `json:"created_at"`
}

func NewModerationResult(contentID string, isApproved bool, score float64, categories []ModerationCategory, modelName string, durationMs int64) (*ModerationResult, error) {
	if contentID == "" {
		return nil, errors.New("content ID cannot be empty")
	}

	if score < 0 || score > 1 {
		return nil, errors.New("score must be between 0 and 1")
	}

	if modelName == "" {
		return nil, errors.New("model name cannot be empty")
	}

	if durationMs < 0 {
		return nil, errors.New("duration cannot be negative")
	}

	return &ModerationResult{
		ID:         "", // Will be set by repository
		ContentID:  contentID,
		IsApproved: isApproved,
		Score:      score,
		Categories: categories,
		ModelName:  modelName,
		DurationMs: durationMs,
		CreatedAt:  time.Now(),
	}, nil
}
