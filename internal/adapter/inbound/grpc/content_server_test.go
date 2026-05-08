package grpc_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/mohammadpnp/content-moderator/api/content"
	grpcadapter "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/grpc"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

func setupContentServer(t *testing.T) (content.ContentServiceClient, func()) {
	t.Helper()

	repo := mock.NewMockContentRepository()
	broker := mock.NewMockMessageBroker()
	contentSvc := service.NewContentService(repo, broker)
	moderationSvc := service.NewModerationService(repo, mock.NewMockAIClient(), mock.NewMockCacheStore(), broker)

	srv := grpc.NewServer()
	content.RegisterContentServiceServer(srv, grpcadapter.NewContentServer(contentSvc, moderationSvc))

	lis := bufconn.Listen(1024 * 1024)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := content.NewContentServiceClient(conn)

	cleanup := func() {
		conn.Close()
		srv.GracefulStop()
	}
	return client, cleanup
}

func TestCreateContent_gRPC(t *testing.T) {
	client, cleanup := setupContentServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	md := metadata.Pairs("user-id", "user-1")
	ctx = metadata.NewOutgoingContext(ctx, md)

	resp, err := client.CreateContent(ctx, &content.CreateContentRequest{
		UserId: "user-1",
		Type:   content.ContentType_CONTENT_TYPE_TEXT,
		Body:   "Hello gRPC",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp.Content)
	assert.Equal(t, "user-1", resp.Content.UserId)
	assert.Equal(t, content.ContentStatus_CONTENT_STATUS_PENDING, resp.Content.Status)
}

func TestGetContent_gRPC(t *testing.T) {
	client, cleanup := setupContentServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	md := metadata.Pairs("user-id", "user-1")
	ctx = metadata.NewOutgoingContext(ctx, md)

	createResp, err := client.CreateContent(ctx, &content.CreateContentRequest{
		UserId: "user-2",
		Type:   content.ContentType_CONTENT_TYPE_TEXT,
		Body:   "Get test",
	})
	require.NoError(t, err)

	getResp, err := client.GetContent(ctx, &content.GetContentRequest{
		Id: createResp.Content.Id,
	})
	require.NoError(t, err)
	assert.Equal(t, "Get test", getResp.Content.Body)
}
