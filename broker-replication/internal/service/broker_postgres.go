// Package service provides Postgres-backed broker implementation.
package service

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/distributed-queue-system/broker-replication/internal/queue"
	"github.com/distributed-queue-system/broker-replication/internal/storage/postgres"
)

// PostgresBroker implements Broker using PostgreSQL storage with DLQ, activity log, and metrics.
type PostgresBroker struct {
	store *postgres.Store
}

// NewPostgresBroker creates a broker backed by PostgreSQL.
func NewPostgresBroker(store *postgres.Store) *PostgresBroker {
	b := &PostgresBroker{store: store}
	go b.requeueExpiredLoop()
	return b
}

// Publish enqueues a message. producerID is optional (from X-Producer-Id header).
func (b *PostgresBroker) Publish(ctx context.Context, topic string, body []byte) error {
	return b.PublishWithProducer(ctx, topic, body, "")
}

// PublishWithProducer enqueues a message with producer ID for audit.
func (b *PostgresBroker) PublishWithProducer(ctx context.Context, topic string, body []byte, producerID string) error {
	if topic == "" {
		return errors.New("topic required")
	}
	if len(body) == 0 {
		return errors.New("body required")
	}
	_, err := b.store.Publish(ctx, topic, body, producerID)
	return err
}

// Consume pulls one message for the topic. consumerID is optional.
func (b *PostgresBroker) Consume(ctx context.Context, topic string) (*queue.Message, error) {
	return b.ConsumeWithConsumer(ctx, topic, "")
}

// ConsumeWithConsumer pulls one message with consumer ID for audit.
func (b *PostgresBroker) ConsumeWithConsumer(ctx context.Context, topic string, consumerID string) (*queue.Message, error) {
	if topic == "" {
		return nil, errors.New("topic required")
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		msg, err := b.store.Consume(ctx, topic, consumerID)
		if err != nil {
			if err == postgres.ErrNoMessageAvailable {
				select {
				case <-ctx.Done():
					return nil, errors.New("no message available")
				case <-time.After(200 * time.Millisecond):
					continue
				}
			}
			return nil, err
		}
		return msg, nil
	}
}

// Ack acknowledges a message. consumerID is optional.
func (b *PostgresBroker) Ack(ctx context.Context, messageID string) error {
	return b.AckWithConsumer(ctx, messageID, "")
}

// AckWithConsumer acknowledges with consumer ID for audit.
func (b *PostgresBroker) AckWithConsumer(ctx context.Context, messageID string, consumerID string) error {
	return b.store.Ack(ctx, messageID, consumerID)
}

// Peek returns up to n messages without consuming.
func (b *PostgresBroker) Peek(ctx context.Context, topic string, n int) []*queue.Message {
	if topic == "" || n <= 0 {
		return nil
	}
	msgs, err := b.store.Peek(ctx, topic, n)
	if err != nil {
		log.Printf("[postgres] peek error: %v", err)
		return nil
	}
	return msgs
}

func (b *PostgresBroker) requeueExpiredLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if err := b.store.RequeueExpired(context.Background()); err != nil {
			log.Printf("[postgres] requeueExpired error: %v", err)
		}
	}
}

// GetMetrics returns queue metrics (queue_depth, pending_count, produced/consumed totals, dead_letter_count).
func (b *PostgresBroker) GetMetrics(ctx context.Context) (map[string]interface{}, error) {
	return b.store.GetMetrics(ctx)
}

// DeadLetterCount returns the count of messages in the dead letter table.
func (b *PostgresBroker) DeadLetterCount(ctx context.Context) (int, error) {
	return b.store.DeadLetterCount(ctx)
}

// DeadLetterRetry re-publishes all DLQ messages to the queue.
func (b *PostgresBroker) DeadLetterRetry(ctx context.Context) (retried, failed int, err error) {
	return b.store.DeadLetterRetry(ctx)
}

// GetActivityLog returns activity log entries.
func (b *PostgresBroker) GetActivityLog(ctx context.Context, limit, offset int, eventType string) ([]ActivityEntry, error) {
	entries, err := b.store.GetActivityLog(ctx, limit, offset, eventType)
	if err != nil {
		return nil, err
	}
	out := make([]ActivityEntry, len(entries))
	for i, e := range entries {
		out[i] = ActivityEntry{
			EventType:  e.EventType,
			MessageID:  e.MessageID,
			Topic:      e.Topic,
			ProducerID: e.ProducerID,
			ConsumerID: e.ConsumerID,
			CreatedAt:  e.CreatedAt,
			Details:    e.Details,
		}
	}
	return out, nil
}
