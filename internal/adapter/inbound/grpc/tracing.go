package grpc

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TracingUnaryInterceptor starts a span for each unary RPC.
func TracingUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		tracer := otel.Tracer("grpc-server")
		// Extract context from incoming metadata (carrier)
		ctx = otel.GetTextMapPropagator().Extract(ctx, metadataReaderWriter(mdFromIncoming(ctx)))
		ctx, span := tracer.Start(ctx, info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		resp, err := handler(ctx, req)
		if err != nil {
			span.RecordError(err)
		}
		return resp, err
	}
}

// TracingStreamInterceptor starts a span for each streaming RPC.
func TracingStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		tracer := otel.Tracer("grpc-server")
		ctx := otel.GetTextMapPropagator().Extract(ss.Context(), metadataReaderWriter(mdFromIncoming(ss.Context())))
		ctx, span := tracer.Start(ctx, info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		wrappedStream := &tracingServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}
		return handler(srv, wrappedStream)
	}
}

func mdFromIncoming(ctx context.Context) metadata.MD {
	md, _ := metadata.FromIncomingContext(ctx)
	return md
}

type metadataReaderWriter metadata.MD

func (m metadataReaderWriter) Get(key string) string {
	md := metadata.MD(m)
	v := md.Get(key)
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

func (m metadataReaderWriter) Set(key, val string) {
	md := metadata.MD(m)
	md.Set(key, val)
}

func (m metadataReaderWriter) Keys() []string {
	md := metadata.MD(m)
	out := make([]string, 0, len(md))
	for k := range md {
		out = append(out, k)
	}
	return out
}

type tracingServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *tracingServerStream) Context() context.Context {
	return s.ctx
}
