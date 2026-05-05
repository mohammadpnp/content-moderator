package grpc_test

import (
	"context"
	"testing"

	"github.com/mohammadpnp/content-moderator/api/content"
	grpcadapter "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/grpc"
	"google.golang.org/grpc"
)

type panickyServer struct {
	content.UnimplementedContentServiceServer
}

func (s *panickyServer) CreateContent(ctx context.Context, req *content.CreateContentRequest) (*content.CreateContentResponse, error) {
	panic("oops")
}

func TestRecoveryInterceptor(t *testing.T) {
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(grpcadapter.RecoveryUnaryInterceptor()),
	)
	content.RegisterContentServiceServer(srv, &panickyServer{})

	// همان setup قبلی با bufconn
	// تست کنیم که در اثر panic خطای Internal برمی‌گردد
	// ...
	// کد کامل شبیه تست gRPC قبلی، اما اینجا انتظار خطا داریم
}
