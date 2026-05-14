package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type ContentRepository struct {
	db *sqlx.DB
}

func NewContentRepository(db *sqlx.DB) *ContentRepository {
	return &ContentRepository{db: db}
}

var _ outbound.ContentRepository = (*ContentRepository)(nil)

func (r *ContentRepository) Save(ctx context.Context, c *entity.Content) error {
	tr := otel.Tracer("postgres")
	ctx, span := tr.Start(ctx, "ContentRepository.Save")
	defer span.End()
	span.SetAttributes(attribute.String("content.id", c.ID), attribute.String("user_id", c.UserID))

	query := `
		INSERT INTO contents (id, user_id, type, body, status, moderation_id, created_at, updated_at)
		VALUES (:id, :user_id, :type, :body, :status, :moderation_id, :created_at, :updated_at)
	`
	c.CreatedAt = time.Now()
	c.UpdatedAt = c.CreatedAt

	_, err := r.db.NamedExecContext(ctx, query, map[string]interface{}{
		"id":            c.ID,
		"user_id":       c.UserID,
		"type":          c.Type,
		"body":          c.Body,
		"status":        c.Status,
		"moderation_id": c.ModerationID,
		"created_at":    c.CreatedAt,
		"updated_at":    c.UpdatedAt,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to insert content: %w", err)
	}
	return nil
}

func (r *ContentRepository) FindByID(ctx context.Context, id string) (*entity.Content, error) {
	tr := otel.Tracer("postgres")
	ctx, span := tr.Start(ctx, "ContentRepository.FindByID")
	defer span.End()
	span.SetAttributes(attribute.String("content.id", id))

	var c entity.Content
	err := r.db.GetContext(ctx, &c, "SELECT * FROM contents WHERE id = $1", id)
	if err == sql.ErrNoRows {
		span.RecordError(err)
		return nil, fmt.Errorf("content not found: %s", id)
	}
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("query error: %w", err)
	}
	return &c, nil
}

func (r *ContentRepository) FindByUserID(ctx context.Context, userID string, limit, offset int) ([]*entity.Content, error) {
	tr := otel.Tracer("postgres")
	ctx, span := tr.Start(ctx, "ContentRepository.FindByUserID")
	defer span.End()
	span.SetAttributes(attribute.String("user_id", userID), attribute.Int("limit", limit), attribute.Int("offset", offset))

	var contents []*entity.Content
	err := r.db.SelectContext(ctx, &contents,
		"SELECT * FROM contents WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		userID, limit, offset)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("query error: %w", err)
	}
	return contents, nil
}

func (r *ContentRepository) UpdateStatus(ctx context.Context, id string, status entity.ContentStatus, moderationID *string) error {
	tr := otel.Tracer("postgres")
	ctx, span := tr.Start(ctx, "ContentRepository.UpdateStatus")
	defer span.End()
	span.SetAttributes(attribute.String("content.id", id), attribute.String("status", string(status)))

	query := "UPDATE contents SET status = $1, moderation_id = $2, updated_at = $3 WHERE id = $4"
	result, err := r.db.ExecContext(ctx, query, status, moderationID, time.Now(), id)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("update error: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		err := fmt.Errorf("content not found: %s", id)
		span.RecordError(err)
		return err
	}
	return nil
}

func (r *ContentRepository) Delete(ctx context.Context, id string) error {
	tr := otel.Tracer("postgres")
	ctx, span := tr.Start(ctx, "ContentRepository.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("content.id", id))

	result, err := r.db.ExecContext(ctx, "DELETE FROM contents WHERE id = $1", id)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("delete error: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		err := fmt.Errorf("content not found: %s", id)
		span.RecordError(err)
		return err
	}
	return nil
}

func (r *ContentRepository) Stats() sql.DBStats {
	return r.db.Stats()
}
