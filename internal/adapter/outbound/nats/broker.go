package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	subjectModerationJob    = "moderation.job"
	subjectModerationResult = "moderation.result"
	subjectNotification     = "notification.send"

	maxReconnects  = 10
	reconnectWait  = 2 * time.Second
	connectTimeout = 10 * time.Second

	subjectModerationJobDLQ = "moderation.job.dlq"
)

// natsHeaderCarrier implements propagation.TextMapCarrier for nats.Header
type natsHeaderCarrier nats.Header

func (c natsHeaderCarrier) Get(key string) string {
	return nats.Header(c).Get(key)
}
func (c natsHeaderCarrier) Set(key, val string) {
	nats.Header(c).Set(key, val)
}
func (c natsHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(nats.Header(c)))
	for k := range nats.Header(c) {
		keys = append(keys, k)
	}
	return keys
}

type NATSBroker struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

func NewNATSBroker() (*NATSBroker, error) {
	host := os.Getenv("NATS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("NATS_PORT")
	if port == "" {
		port = "4222"
	}

	url := fmt.Sprintf("nats://%s:%s", host, port)

	opts := []nats.Option{
		nats.Name("content-moderator"),
		nats.MaxReconnects(maxReconnects),
		nats.ReconnectWait(reconnectWait),
		nats.Timeout(connectTimeout),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Warn().Err(err).Msg("NATS disconnected")
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info().Str("url", nc.ConnectedUrl()).Msg("NATS reconnected")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Error().Err(err).Msg("NATS error")
		}),
	}

	conn, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to get JetStream context: %w", err)
	}

	if err := ensureStreams(js); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ensure streams: %w", err)
	}
	if err := ensureConsumer(js); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ensure consumer: %w", err)
	}

	log.Info().Str("url", url).Msg("Connected to NATS")
	return &NATSBroker{conn: conn, js: js}, nil
}

func ensureStreams(js nats.JetStreamContext) error {
	streams := []*nats.StreamConfig{
		{
			Name:     "MODERATION_JOBS",
			Subjects: []string{subjectModerationJob},
			MaxAge:   24 * time.Hour,
			Storage:  nats.FileStorage,
		},
		{
			Name:     "MODERATION_RESULTS",
			Subjects: []string{subjectModerationResult},
			MaxAge:   24 * time.Hour,
			Storage:  nats.FileStorage,
		},
		{
			Name:     "NOTIFICATIONS",
			Subjects: []string{subjectNotification},
			MaxAge:   7 * 24 * time.Hour,
			Storage:  nats.FileStorage,
		},
		{
			Name:     "MODERATION_DLQ",
			Subjects: []string{subjectModerationJobDLQ},
			MaxAge:   30 * 24 * time.Hour,
			Storage:  nats.FileStorage,
		},
	}

	for _, cfg := range streams {
		if _, err := js.AddStream(cfg); err != nil {
			if err != nats.ErrStreamNameAlreadyInUse {
				return fmt.Errorf("failed to create stream %s: %w", cfg.Name, err)
			}
			if _, err := js.UpdateStream(cfg); err != nil {
				return fmt.Errorf("failed to update stream %s: %w", cfg.Name, err)
			}
		}
	}
	return nil
}

func ensureConsumer(js nats.JetStreamContext) error {
	consumerCfg := &nats.ConsumerConfig{
		Durable:       "moderation-job-consumer",
		DeliverGroup:  "moderation-workers",
		AckPolicy:     nats.AckExplicitPolicy,
		MaxDeliver:    3,
		MaxAckPending: 100,
		BackOff:       []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond},
	}
	_, err := js.AddConsumer("MODERATION_JOBS", consumerCfg)
	if err != nil && err != nats.ErrConsumerNameAlreadyInUse {
		return fmt.Errorf("failed to add consumer: %w", err)
	}
	if err == nats.ErrConsumerNameAlreadyInUse {
		_, err = js.UpdateConsumer("MODERATION_JOBS", consumerCfg)
		if err != nil {
			return fmt.Errorf("failed to update consumer: %w", err)
		}
	}
	return nil
}

func (b *NATSBroker) PublishModerationJob(ctx context.Context, content *entity.Content) error {
	tr := otel.Tracer("nats")
	ctx, span := tr.Start(ctx, "NATSBroker.PublishModerationJob", trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()
	span.SetAttributes(attribute.String("content.id", content.ID))

	data, err := json.Marshal(content)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal content: %w", err)
	}
	msg := nats.NewMsg(subjectModerationJob)
	msg.Data = data
	msg.Header.Set("Content-Type", "application/json")

	carrier := natsHeaderCarrier(msg.Header)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	_, err = b.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to publish moderation job: %w", err)
	}
	log.Debug().Str("content_id", content.ID).Msg("published moderation job")
	return nil
}

func (b *NATSBroker) PublishModerationResult(ctx context.Context, result *entity.ModerationResult) error {
	tr := otel.Tracer("nats")
	ctx, span := tr.Start(ctx, "NATSBroker.PublishModerationResult", trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()
	span.SetAttributes(attribute.String("content.id", result.ContentID))

	data, err := json.Marshal(result)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal moderation result: %w", err)
	}
	msg := nats.NewMsg(subjectModerationResult)
	msg.Data = data
	msg.Header.Set("Content-Type", "application/json")

	carrier := natsHeaderCarrier(msg.Header)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	_, err = b.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to publish moderation result: %w", err)
	}
	log.Debug().Str("content_id", result.ContentID).Bool("approved", result.IsApproved).Msg("published moderation result")
	return nil
}

func (b *NATSBroker) PublishNotification(ctx context.Context, notification *entity.Notification) error {
	tr := otel.Tracer("nats")
	ctx, span := tr.Start(ctx, "NATSBroker.PublishNotification", trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()
	span.SetAttributes(attribute.String("user_id", notification.UserID), attribute.String("content_id", notification.ContentID))

	data, err := json.Marshal(notification)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	msg := nats.NewMsg(subjectNotification)
	msg.Data = data
	msg.Header.Set("Content-Type", "application/json")

	carrier := natsHeaderCarrier(msg.Header)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	_, err = b.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to publish notification: %w", err)
	}
	log.Debug().Str("user_id", notification.UserID).Str("content_id", notification.ContentID).Msg("published notification")
	return nil
}

func (b *NATSBroker) SubscribeModerationJobs(ctx context.Context, handler func(content *entity.Content) error) error {
	sub, err := b.js.QueueSubscribe(
		subjectModerationJob,
		"moderation-workers",
		func(msg *nats.Msg) {
			// Extract trace context from headers
			carrier := natsHeaderCarrier(msg.Header)
			ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
			tr := otel.Tracer("nats")
			ctx, span := tr.Start(ctx, "NATSBroker.SubscribeModerationJobs", trace.WithSpanKind(trace.SpanKindConsumer))
			defer span.End()

			var content entity.Content
			if err := json.Unmarshal(msg.Data, &content); err != nil {
				log.Warn().Err(err).Msg("unmarshal job content")
				span.RecordError(err)
				msg.Nak()
				return
			}
			span.SetAttributes(attribute.String("content.id", content.ID))

			if err := handler(&content); err != nil {
				log.Error().Err(err).Str("content_id", content.ID).Msg("processing job")
				span.RecordError(err)
				msg.Nak()
				return
			}
			msg.Ack()
		},
		nats.ManualAck(),
		nats.Bind("MODERATION_JOBS", "moderation-job-consumer"),
	)
	if err != nil {
		return fmt.Errorf("subscribe to jobs: %w", err)
	}
	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
		log.Info().Msg("Jobs subscription closed")
	}()
	return nil
}

func (b *NATSBroker) SubscribeModerationResults(ctx context.Context, handler func(result *entity.ModerationResult) error) error {
	sub, err := b.js.Subscribe(subjectModerationResult, func(msg *nats.Msg) {
		carrier := natsHeaderCarrier(msg.Header)
		ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
		tr := otel.Tracer("nats")
		ctx, span := tr.Start(ctx, "NATSBroker.SubscribeModerationResults", trace.WithSpanKind(trace.SpanKindConsumer))
		defer span.End()

		var result entity.ModerationResult
		if err := json.Unmarshal(msg.Data, &result); err != nil {
			log.Warn().Err(err).Msg("unmarshal moderation result")
			span.RecordError(err)
			return
		}
		span.SetAttributes(attribute.String("content.id", result.ContentID))

		if err := handler(&result); err != nil {
			log.Error().Err(err).Str("content_id", result.ContentID).Msg("handling moderation result")
			span.RecordError(err)
		}
		msg.Ack()
	}, nats.ManualAck(), nats.Durable("moderation-result-consumer"))
	if err != nil {
		return fmt.Errorf("failed to subscribe to moderation results: %w", err)
	}
	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
		log.Info().Msg("Moderation results subscription closed")
	}()
	return nil
}

func (b *NATSBroker) SubscribeNotifications(ctx context.Context, handler func(notification *entity.Notification) error) error {
	sub, err := b.js.Subscribe(subjectNotification, func(msg *nats.Msg) {
		carrier := natsHeaderCarrier(msg.Header)
		ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
		tr := otel.Tracer("nats")
		ctx, span := tr.Start(ctx, "NATSBroker.SubscribeNotifications", trace.WithSpanKind(trace.SpanKindConsumer))
		defer span.End()

		var notif entity.Notification
		if err := json.Unmarshal(msg.Data, &notif); err != nil {
			log.Warn().Err(err).Msg("unmarshal notification")
			span.RecordError(err)
			msg.Nak()
			return
		}
		span.SetAttributes(attribute.String("user_id", notif.UserID), attribute.String("content_id", notif.ContentID))

		if err := handler(&notif); err != nil {
			log.Error().Err(err).Str("user_id", notif.UserID).Msg("handling notification")
			span.RecordError(err)
			msg.Nak()
			return
		}
		msg.Ack()
	}, nats.ManualAck(), nats.Durable("notification-consumer"))
	if err != nil {
		return fmt.Errorf("subscribe to notifications: %w", err)
	}
	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
		log.Info().Msg("Notifications subscription closed")
	}()
	return nil
}

func (b *NATSBroker) StartDLQMonitor(ctx context.Context) error {
	advisorySubject := "$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.MODERATION_JOBS.moderation-job-consumer"
	sub, err := b.js.Subscribe(advisorySubject, func(msg *nats.Msg) {
		var advisory struct {
			Stream string `json:"stream"`
			Seq    uint64 `json:"stream_seq"`
		}
		if err := json.Unmarshal(msg.Data, &advisory); err != nil {
			log.Warn().Err(err).Msg("DLQ: failed to parse advisory")
			msg.Nak()
			return
		}
		if advisory.Stream == "" || advisory.Seq == 0 {
			log.Warn().Msg("DLQ: incomplete advisory data")
			msg.Nak()
			return
		}
		rawMsg, err := b.js.GetMsg(advisory.Stream, advisory.Seq)
		if err != nil {
			log.Warn().Err(err).Str("stream", advisory.Stream).Uint64("seq", advisory.Seq).Msg("DLQ: failed to fetch original message")
			msg.Nak()
			return
		}
		dlqMsg := nats.NewMsg(subjectModerationJobDLQ)
		dlqMsg.Data = rawMsg.Data
		if rawMsg.Header != nil {
			dlqMsg.Header = rawMsg.Header
		}
		dlqMsg.Header.Set("X-Original-Stream", advisory.Stream)
		dlqMsg.Header.Set("X-Original-Seq", fmt.Sprintf("%d", advisory.Seq))
		_, err = b.js.PublishMsg(dlqMsg, nats.Context(ctx))
		if err != nil {
			log.Warn().Err(err).Msg("DLQ: error publishing to DLQ")
			msg.Nak()
			return
		}
		log.Info().Str("stream", advisory.Stream).Uint64("seq", advisory.Seq).Msg("DLQ: moved message to DLQ")
		msg.Ack()
	}, nats.ManualAck())
	if err != nil {
		return fmt.Errorf("failed to subscribe to DLQ advisory: %w", err)
	}
	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
		log.Info().Msg("DLQ monitor subscription closed")
	}()
	return nil
}

func (b *NATSBroker) Close() {
	if b.conn != nil {
		b.conn.Drain()
		b.conn.Close()
		log.Info().Msg("NATS connection closed")
	}
}

var _ outbound.MessageBroker = (*NATSBroker)(nil)
