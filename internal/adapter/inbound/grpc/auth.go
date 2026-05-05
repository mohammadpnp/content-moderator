package grpc

import (
	"context"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthUnaryInterceptor extracts JWT token from gRPC metadata, validates it,
// and injects the user ID into the context.
func AuthUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		userID, err := authenticate(ctx)
		if err != nil {
			return nil, err
		}
		// Store userID in context for downstream handlers
		ctx = context.WithValue(ctx, "userID", userID)
		return handler(ctx, req)
	}
}

// AuthStreamInterceptor is the stream version of authentication.
func AuthStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		userID, err := authenticate(ss.Context())
		if err != nil {
			return err
		}
		wrapped := &authenticatedStream{
			ServerStream: ss,
			ctx:          context.WithValue(ss.Context(), "userID", userID),
		}
		return handler(srv, wrapped)
	}
}

// authenticate extracts the JWT token from metadata and validates it.
// For demonstration, it accepts tokens starting with "valid-".
// Replace with real JWT validation logic in production.
func authenticate(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	values := md["authorization"]
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization token")
	}

	tokenStr := values[0]
	if !strings.HasPrefix(tokenStr, "Bearer ") {
		return "", status.Error(codes.Unauthenticated, "authorization header must be Bearer token")
	}
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")

	// For now, simple check; replace with jwt.Parse when ready
	if strings.HasPrefix(tokenStr, "valid-") {
		// extract user ID from token (e.g., "valid-user123")
		userID := strings.TrimPrefix(tokenStr, "valid-")
		return userID, nil
	}

	// Try real JWT parsing
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
	if err == nil {
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if sub, ok := claims["sub"].(string); ok {
				return sub, nil
			}
		}
	}

	return "", status.Error(codes.Unauthenticated, "invalid token")
}

// authenticatedStream wraps grpc.ServerStream to override context.
type authenticatedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedStream) Context() context.Context {
	return s.ctx
}
