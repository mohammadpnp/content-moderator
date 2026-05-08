package grpc

import (
	"context"
	"errors"
	"io"

	"github.com/mohammadpnp/content-moderator/api/content"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/inbound"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "metadata not found")
	}

	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id not found in metadata")
	}
	userID := userIDs[0]

	if !ok || userID == "" {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}
	c, err := s.svc.CreateContent(ctx, userID, contentTypeFromProto(req.Type), req.Body)
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
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	authUserID := ""
	if vals := md.Get("user-id"); len(vals) > 0 {
		authUserID = vals[0]
	}
	if authUserID == "" {
		return status.Error(codes.Unauthenticated, "user not authenticated")
	}

	targetUserID := req.UserId
	if targetUserID == "" {
		targetUserID = authUserID
	} else if targetUserID != authUserID {
		return status.Error(codes.PermissionDenied, "cannot view results of another user")
	}

	const pageSize = 50
	offset := 0
	for {
		contents, err := s.svc.ListUserContents(stream.Context(), targetUserID, pageSize, offset)
		if err != nil {
			return status.Errorf(codes.Internal, "listing contents: %v", err)
		}
		if len(contents) == 0 {
			break
		}

		for _, c := range contents {
			mod, err := s.modSvc.GetModerationResult(stream.Context(), c.ID)
			if err != nil {
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
				CreatedAt:  nil,
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

		if len(contents) < pageSize {
			break
		}
		offset += pageSize
	}

	return nil
}
