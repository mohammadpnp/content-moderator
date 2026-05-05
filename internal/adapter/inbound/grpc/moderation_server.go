// moderation_server.go

package grpc

import (
	"context"
	"errors"
	"io"

	"github.com/mohammadpnp/content-moderator/api/moderation"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/inbound"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ModerationServer struct {
	moderation.UnimplementedModerationServiceServer
	svc inbound.ModerationService
}

func NewModerationServer(svc inbound.ModerationService) *ModerationServer {
	return &ModerationServer{svc: svc}
}

func (s *ModerationServer) ModerateContent(stream moderation.ModerationService_ModerateContentServer) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "recv error: %v", err)
		}
		// Call the service's ModerateContent
		result, err := s.svc.ModerateContent(stream.Context(), req.ContentId)
		if err != nil {
			return status.Errorf(codes.Internal, "moderation failed: %v", err)
		}
		// Convert to proto and send
		if err := stream.Send(&moderation.ModerateContentResponse{
			ContentId: result.ContentID,
			Result:    moderationResultToProto(result),
		}); err != nil {
			return status.Errorf(codes.Internal, "send error: %v", err)
		}
	}
}

func (s *ModerationServer) GetModelHealth(ctx context.Context, req *moderation.GetModelHealthRequest) (*moderation.GetModelHealthResponse, error) {
	// We can call AIClient.IsHealthy via ModerationService? But ModerationService doesn't expose that.
	// For now return true. In future we can inject AIClient into this server.
	return &moderation.GetModelHealthResponse{Healthy: true}, nil
}
