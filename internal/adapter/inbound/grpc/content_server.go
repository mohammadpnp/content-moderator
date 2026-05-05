package grpc

import (
	"context"
	"errors"
	"io"

	"github.com/mohammadpnp/content-moderator/api/content"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/inbound"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ContentServer struct {
	content.UnimplementedContentServiceServer
	svc inbound.ContentService
	// For streaming we may need ModerationService to fetch results
	modSvc inbound.ModerationService
}

func NewContentServer(svc inbound.ContentService, modSvc inbound.ModerationService) *ContentServer {
	return &ContentServer{svc: svc, modSvc: modSvc}
}

func (s *ContentServer) CreateContent(ctx context.Context, req *content.CreateContentRequest) (*content.CreateContentResponse, error) {
	c, err := s.svc.CreateContent(ctx, req.UserId, contentTypeFromProto(req.Type), req.Body)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating content: %v", err)
	}
	return &content.CreateContentResponse{Content: contentToProto(c)}, nil
}

func (s *ContentServer) GetContent(ctx context.Context, req *content.GetContentRequest) (*content.GetContentResponse, error) {
	c, err := s.svc.GetContent(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "content not found: %v", err)
	}
	return &content.GetContentResponse{Content: contentToProto(c)}, nil
}

func (s *ContentServer) StreamModerationResults(req *content.StreamModerationResultsRequest, stream content.ContentService_StreamModerationResultsServer) error {
	// For now, simply send a static message.
	// In later phases, this will be replaced with real-time subscription.
	if req.UserId == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}
	// Use GetUserContents to get all contents for the user, then for each send its moderation result if available
	// This is a simple polling approach.
	const pageSize = 50
	offset := 0
	for {
		contents, err := s.svc.ListUserContents(stream.Context(), req.UserId, pageSize, offset)
		if err != nil {
			return status.Errorf(codes.Internal, "listing contents: %v", err)
		}
		if len(contents) == 0 {
			break
		}
		for _, c := range contents {
			mod, err := s.modSvc.GetModerationResult(stream.Context(), c.ID)
			if err != nil {
				// Skip content without moderation result yet
				continue
			}
			msg := &content.ModerationResult{
				Id:         mod.ID,
				ContentId:  mod.ContentID,
				IsApproved: mod.IsApproved,
				Score:      mod.Score,
				Categories: make([]string, len(mod.Categories)),
				ModelName:  mod.ModelName,
				DurationMs: mod.DurationMs,
				CreatedAt:  nil, // conversion later if needed
			}
			for i, cat := range mod.Categories {
				msg.Categories[i] = string(cat)
			}
			if err := stream.Send(msg); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return status.Errorf(codes.Internal, "sending result: %v", err)
			}
		}
		offset += pageSize
	}
	return nil
}
