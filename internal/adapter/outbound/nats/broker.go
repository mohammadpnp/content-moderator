package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
	"github.com/nats-io/nats.go"
)

const (
	// Subject names
	subjectModerationJob    = "moderation.job"
	subjectModerationResult = "moderation.result"
	subjectNotification     = "notification.send"

	// Connection settings
	maxReconnects  = 10
	reconnectWait  = 2 * time.Second
	connectTimeout = 10 * time.Second
	publishTimeout = 5 * time.Second
)

// NATSBroker implements outbound.MessageBroker using NATS.
type NATSBroker struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

// NewNATSBroker creates a new NATS broker connection.
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
			log.Printf("NATS disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("NATS reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Printf("NATS error: %v", err)
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

	// Ensure streams exist
	if err := ensureStreams(js); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ensure streams: %w", err)
	}

	log.Printf("Connected to NATS at %s", url)
	return &NATSBroker{conn: conn, js: js}, nil
}

// ensureStreams creates necessary JetStream streams if they don't exist.
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
	}

	for _, cfg := range streams {
		if _, err := js.AddStream(cfg); err != nil {
			if err != nats.ErrStreamNameAlreadyInUse {
				return fmt.Errorf("failed to create stream %s: %w", cfg.Name, err)
			}
			// Stream already exists, update it
			if _, err := js.UpdateStream(cfg); err != nil {
				return fmt.Errorf("failed to update stream %s: %w", cfg.Name, err)
			}
		}
	}
	return nil
}

// PublishModerationJob publishes a content moderation job.
func (b *NATSBroker) PublishModerationJob(ctx context.Context, content *entity.Content) error {
	data, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	msg := nats.NewMsg(subjectModerationJob)
	msg.Data = data
	msg.Header.Set("Content-Type", "application/json")

	// Use JetStream for persistence
	_, err = b.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("failed to publish moderation job: %w", err)
	}

	log.Printf("Published moderation job for content: %s", content.ID)
	return nil
}

// SubscribeModerationResults subscribes to moderation results.
func (b *NATSBroker) SubscribeModerationResults(ctx context.Context, handler func(result *entity.ModerationResult) error) error {
	sub, err := b.js.Subscribe(subjectModerationResult, func(msg *nats.Msg) {
		var result entity.ModerationResult
		if err := json.Unmarshal(msg.Data, &result); err != nil {
			log.Printf("WARNING: failed to unmarshal moderation result: %v", err)
			return
		}

		if err := handler(&result); err != nil {
			log.Printf("ERROR handling moderation result: %v", err)
		}

		msg.Ack()
	}, nats.ManualAck(), nats.Durable("moderation-result-consumer"))
	if err != nil {
		return fmt.Errorf("failed to subscribe to moderation results: %w", err)
	}

	// Clean up subscription when context is done
	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
		log.Println("Moderation results subscription closed")
	}()

	log.Println("Subscribed to moderation results")
	return nil
}

// PublishNotification publishes a notification to NATS.
func (b *NATSBroker) PublishNotification(ctx context.Context, notification *entity.Notification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	msg := nats.NewMsg(subjectNotification)
	msg.Data = data
	msg.Header.Set("Content-Type", "application/json")

	_, err = b.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("failed to publish notification: %w", err)
	}

	log.Printf("Published notification for user %s (content: %s, type: %s)",
		notification.UserID, notification.ContentID, notification.Type)
	return nil
}

// Close closes the NATS connection.
func (b *NATSBroker) Close() {
	if b.conn != nil {
		b.conn.Drain()
		b.conn.Close()
		log.Println("NATS connection closed")
	}
}

// PublishModerationResult publishes a moderation result back to NATS.
func (b *NATSBroker) PublishModerationResult(ctx context.Context, result *entity.ModerationResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal moderation result: %w", err)
	}

	msg := nats.NewMsg(subjectModerationResult)
	msg.Data = data
	msg.Header.Set("Content-Type", "application/json")

	_, err = b.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("failed to publish moderation result: %w", err)
	}

	log.Printf("Published moderation result for content: %s (approved: %v)", result.ContentID, result.IsApproved)
	return nil
}

// SubscribeModerationJobs subscribes to incoming moderation jobs.
func (b *NATSBroker) SubscribeModerationJobs(ctx context.Context, handler func(content *entity.Content) error) error {
	sub, err := b.js.QueueSubscribe(
		subjectModerationJob,
		"moderation-workers", // queue group for load balancing
		func(msg *nats.Msg) {
			var content entity.Content
			if err := json.Unmarshal(msg.Data, &content); err != nil {
				log.Printf("WARNING: failed to unmarshal job content: %v", err)
				msg.Nak()
				return
			}

			if err := handler(&content); err != nil {
				log.Printf("ERROR processing job for content %s: %v", content.ID, err)
				msg.Nak()
				return
			}

			msg.Ack()
		},
		nats.ManualAck(),
		nats.Durable("moderation-job-consumer"),
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to moderation jobs: %w", err)
	}

	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
		log.Println("Moderation jobs subscription closed")
	}()

	log.Println("Subscribed to moderation jobs (worker group 'moderation-workers')")
	return nil
}

// Verify interface compliance
var _ outbound.MessageBroker = (*NATSBroker)(nil)
