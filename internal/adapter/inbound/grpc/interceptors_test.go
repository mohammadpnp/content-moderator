package grpc_test

import (
	"context"
	"net"
	"testing"

	"github.com/mohammadpnp/content-moderator/api/content"
	grpcadapter "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type panickyServer struct {
	content.UnimplementedContentServiceServer
}

func (s *panickyServer) CreateContent(ctx context.Context, req *content.CreateContentRequest) (*content.CreateContentResponse, error) {
	panic("oops")
}

func TestRecoveryInterceptor(t *testing.T) {
	// Create gRPC server with recovery interceptor
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(grpcadapter.RecoveryUnaryInterceptor()),
	)
	content.RegisterContentServiceServer(srv, &panickyServer{})

	// Use bufconn for in-process communication
	lis := bufconn.Listen(1024 * 1024)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer srv.GracefulStop()

	// Dial to bufconn
	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := content.NewContentServiceClient(conn)

	// Call that panics on the server side
	_, err = client.CreateContent(context.Background(), &content.CreateContentRequest{
		UserId: "user-1",
		Type:   content.ContentType_CONTENT_TYPE_TEXT,
		Body:   "test",
	})

	// Should get an Internal error
	require.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code(), "expected Internal error from panic recovery")
}
