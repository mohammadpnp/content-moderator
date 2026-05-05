
package grpc

import (
	"github.com/mohammadpnp/content-moderator/api/content"
	"github.com/mohammadpnp/content-moderator/api/moderation"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func contentToProto(c *entity.Content) *content.Content {
	if c == nil {
		return nil
	}
	ct := content.ContentType_CONTENT_TYPE_UNSPECIFIED
	switch c.Type {
	case entity.ContentTypeText:
		ct = content.ContentType_CONTENT_TYPE_TEXT
	case entity.ContentTypeImage:
		ct = content.ContentType_CONTENT_TYPE_IMAGE
	}
	cs := content.ContentStatus_CONTENT_STATUS_UNSPECIFIED
	switch c.Status {
	case entity.ContentStatusPending:
		cs = content.ContentStatus_CONTENT_STATUS_PENDING
	case entity.ContentStatusApproved:
		cs = content.ContentStatus_CONTENT_STATUS_APPROVED
	case entity.ContentStatusRejected:
		cs = content.ContentStatus_CONTENT_STATUS_REJECTED
	}
	var modID *string
	if c.ModerationID != nil {
		modID = c.ModerationID
	}
	return &content.Content{
		Id:           c.ID,
		UserId:       c.UserID,
		Type:         ct,
		Body:         c.Body,
		Status:       cs,
		ModerationId: modID,
		CreatedAt:    timestamppb.New(c.CreatedAt),
		UpdatedAt:    timestamppb.New(c.UpdatedAt),
	}
}

func contentTypeFromProto(ct content.ContentType) entity.ContentType {
	switch ct {
	case content.ContentType_CONTENT_TYPE_TEXT:
		return entity.ContentTypeText
	case content.ContentType_CONTENT_TYPE_IMAGE:
		return entity.ContentTypeImage
	default:
		return entity.ContentTypeText
	}
}

func moderationResultToProto(r *entity.ModerationResult) *moderation.ModerationResult {
	if r == nil {
		return nil
	}
	cats := make([]string, len(r.Categories))
	for i, c := range r.Categories {
		cats[i] = string(c)
	}
	return &moderation.ModerationResult{
		Id:         r.ID,
		ContentId:  r.ContentID,
		IsApproved: r.IsApproved,
		Score:      r.Score,
		Categories: cats,
		ModelName:  r.ModelName,
		DurationMs: r.DurationMs,
		CreatedAt:  timestamppb.New(r.CreatedAt),
	}
}

func moderationResultFromProto(r *moderation.ModerationResult) *entity.ModerationResult {
	if r == nil {
		return nil
	}
	cats := make([]entity.ModerationCategory, len(r.Categories))
	for i, cat := range r.Categories {
		cats[i] = entity.ModerationCategory(cat)
	}
	return &entity.ModerationResult{
		ID:         r.Id,
		ContentID:  r.ContentId,
		IsApproved: r.IsApproved,
		Score:      r.Score,
		Categories: cats,
		ModelName:  r.ModelName,
		DurationMs: r.DurationMs,
		CreatedAt:  r.CreatedAt.AsTime(),
	}
}
