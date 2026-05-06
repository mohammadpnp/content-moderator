package grpc_test

import (
	"context"
	"net"
	"testing"

	"github.com/mohammadpnp/content-moderator/api/content"
	"github.com/mohammadpnp/content-moderator/api/moderation"
	grpcadapter "github.com/mohammadpnp/content-moderator/internal/adapter/inbound/grpc"
	"github.com/mohammadpnp/content-moderator/internal/service"
	"github.com/mohammadpnp/content-moderator/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// ============================================
// راه‌اندازی مشترک برای تست‌های gRPC با Auth
// ============================================
func setupAuthenticatedServer(t *testing.T) (content.ContentServiceClient, moderation.ModerationServiceClient, func()) {
	t.Helper()

	// ساخت mock ها
	repo := mock.NewMockContentRepository()
	broker := mock.NewMockMessageBroker()
	aiClient := mock.NewMockAIClient()
	cache := mock.NewMockCacheStore()

	contentSvc := service.NewContentService(repo, broker)
	moderationSvc := service.NewModerationService(repo, aiClient, cache, broker)

	// سرور gRPC با اینترسپتور احراز هویت
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(grpcadapter.AuthUnaryInterceptor()),
		grpc.StreamInterceptor(grpcadapter.AuthStreamInterceptor()),
	)
	content.RegisterContentServiceServer(srv, grpcadapter.NewContentServer(contentSvc, moderationSvc))
	moderation.RegisterModerationServiceServer(srv, grpcadapter.NewModerationServer(moderationSvc))

	lis := bufconn.Listen(1024 * 1024)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Server exited: %v", err)
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

	contentClient := content.NewContentServiceClient(conn)
	moderationClient := moderation.NewModerationServiceClient(conn)

	cleanup := func() {
		conn.Close()
		srv.GracefulStop()
	}
	return contentClient, moderationClient, cleanup
}

// ============================================
// تست‌های احراز هویت
// ============================================
func TestAuthInterceptor_Unary(t *testing.T) {
	client, _, cleanup := setupAuthenticatedServer(t)
	defer cleanup()

	req := &content.CreateContentRequest{
		UserId: "user-auth-test",
		Type:   content.ContentType_CONTENT_TYPE_TEXT,
		Body:   "Hello",
	}

	// بدون توکن
	_, err := client.CreateContent(context.Background(), req)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code(), "باید خطای Unauthenticated برگردد")

	// با توکن نامعتبر
	md := metadata.New(map[string]string{"authorization": "Bearer invalid-token"})
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	_, err = client.CreateContent(ctx, req)
	require.Error(t, err)
	st, ok = status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())

	// با توکن معتبر (پیشوند valid-)
	md = metadata.New(map[string]string{"authorization": "Bearer valid-testuser"})
	ctx = metadata.NewOutgoingContext(context.Background(), md)
	resp, err := client.CreateContent(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "user-auth-test", resp.Content.UserId) // user_id از درخواست اختیاری است
}

func TestAuthInterceptor_Stream(t *testing.T) {
	_, modClient, cleanup := setupAuthenticatedServer(t)
	defer cleanup()

	// سعی در باز کردن استریم بدون توکن
	stream, err := modClient.ModerateContent(context.Background())
	require.NoError(t, err)

	// ارسال یک پیام (اینجا ممکن است خطا بلافاصله ظاهر نشود)
	err = stream.Send(&moderation.ModerateContentRequest{
		ContentId: "some-id",
	})
	// اگر Send خطا داد همانجا بررسی می‌کنیم
	if err != nil {
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code(), "باید خطای احراز هویت دریافت شود")
		return
	}

	// در غیر این صورت، با Recv منتظر خطا از سمت سرور می‌مانیم
	_, err = stream.Recv()
	require.Error(t, err, "باید خطای احراز هویت هنگام دریافت پاسخ رخ دهد")
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

// ============================================
// تست Server Streaming: StreamModerationResults
// ============================================
func TestStreamModerationResults(t *testing.T) {
	contentClient, modClient, cleanup := setupAuthenticatedServer(t)
	defer cleanup()

	// توکن معتبر
	md := metadata.New(map[string]string{"authorization": "Bearer valid-streamuser"})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// ایجاد یک محتوا
	resp, err := contentClient.CreateContent(ctx, &content.CreateContentRequest{
		UserId: "streamuser",
		Type:   content.ContentType_CONTENT_TYPE_TEXT,
		Body:   "test message for stream",
	})
	require.NoError(t, err)
	contentID := resp.Content.Id

	// انجام moderation روی محتوا از طریق bidi stream
	modStream, err := modClient.ModerateContent(ctx)
	require.NoError(t, err)

	err = modStream.Send(&moderation.ModerateContentRequest{
		ContentId: contentID,
	})
	require.NoError(t, err)

	// دریافت پاسخ moderation
	_, err = modStream.Recv()
	require.NoError(t, err)

	err = modStream.CloseSend()
	require.NoError(t, err)

	// حالا StreamModerationResults را فراخوانی می‌کنیم
	stream, err := contentClient.StreamModerationResults(ctx, &content.StreamModerationResultsRequest{
		UserId: "streamuser",
	})
	require.NoError(t, err)

	// دریافت حداقل یک نتیجه
	msg, err := stream.Recv()
	require.NoError(t, err)
	assert.NotEmpty(t, msg.ContentId)
	assert.NotNil(t, msg.IsApproved)

	// ممکن است نتایج دیگری هم باشد
	for {
		msg, err := stream.Recv()
		if err != nil {
			break
		}
		assert.NotEmpty(t, msg.ContentId)
	}
}

// ============================================
// تست Bidirectional Streaming: ModerateContent
// ============================================
func TestModerateContentBidiStream(t *testing.T) {
	repo := mock.NewMockContentRepository()
	broker := mock.NewMockMessageBroker()
	aiClient := mock.NewMockAIClient()
	cache := mock.NewMockCacheStore()

	modSvc := service.NewModerationService(repo, aiClient, cache, broker)
	srv := grpc.NewServer(
		grpc.StreamInterceptor(grpcadapter.AuthStreamInterceptor()), // این تست احراز هویت هم دارد
	)
	moderation.RegisterModerationServiceServer(srv, grpcadapter.NewModerationServer(modSvc))

	lis := bufconn.Listen(1024 * 1024)
	go srv.Serve(lis)

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, _ := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	defer conn.Close()
	defer srv.GracefulStop()

	modClient := moderation.NewModerationServiceClient(conn)

	// توکن معتبر
	md := metadata.New(map[string]string{"authorization": "Bearer valid-bidiuser"})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// ابتدا یک محتوا در دیتابیس ذخیره می‌کنیم (با استفاده از سرویس content – برای این تست خودمان مستقیم repo می‌زنیم)
	c, _ := service.NewContentService(repo, broker).CreateContent(context.Background(), "bidiuser", "text", "Hello bidi test")
	require.NotNil(t, c)

	stream, err := modClient.ModerateContent(ctx)
	require.NoError(t, err)

	// ارسال یک درخواست
	err = stream.Send(&moderation.ModerateContentRequest{
		ContentId: c.ID,
	})
	require.NoError(t, err)

	// دریافت پاسخ
	resp, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, c.ID, resp.ContentId)
	assert.NotNil(t, resp.Result)
	assert.Greater(t, resp.Result.Score, float64(0))

	// بستن استریم
	err = stream.CloseSend()
	require.NoError(t, err)
}
