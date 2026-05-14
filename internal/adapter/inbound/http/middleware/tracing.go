package middleware

import (
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

func TracingMiddleware() fiber.Handler {
	tracer := otel.Tracer("http-server")
	return func(c *fiber.Ctx) error {
		ctx := otel.GetTextMapPropagator().Extract(c.UserContext(), propagation.HeaderCarrier(c.GetReqHeaders()))
		ctx, span := tracer.Start(ctx, c.Method()+" "+c.Route().Path, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		spanCtx := span.SpanContext()
		if spanCtx.IsValid() {
			c.Set("X-Trace-ID", spanCtx.TraceID().String())
		}

		c.SetUserContext(ctx)

		err := c.Next()
		if err != nil {
			span.RecordError(err)
		}
		span.SetAttributes(
			semconv.HTTPStatusCode(c.Response().StatusCode()),
		)
		return err
	}
}
