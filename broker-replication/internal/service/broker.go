// ASSIGNED TO: ENGINEER C
//
// Package service is the glue between API and Queue. It implements the "Pull" model:
// when the API asks for a message, this layer checks the Queue and manages the
// Pending list (messages waiting for client Acknowledgement).

package service

import (
	"context"

	"github.com/distributed-queue-system/broker-replication/internal/queue"
)

// Broker is the interface for the broker service. The API layer depends on this
// interface so it stays decoupled from the concrete implementation.
type Broker interface {
	// Publish enqueues a message for the given topic.
	Publish(ctx context.Context, topic string, body []byte) error
	// Consume pulls one message for the topic. The message is moved to Pending
	// until the client calls Ack. Returns error if no message available or context cancelled.
	Consume(ctx context.Context, topic string) (*queue.Message, error)
	// Ack acknowledges a message by ID, removing it from Pending.
	Ack(ctx context.Context, messageID string) error
}

// BrokerService is the concrete broker: it uses the Queue and maintains a Pending list.
// ASSIGNED TO: ENGINEER C
type BrokerService struct {
	queue   queue.Queue
	pending map[string]queue.Message // messageID -> message, waiting for ack
	// TODO: add mutex or sync for pending map; consider per-topic queues if needed.
}

// NewBrokerService constructs a broker that uses the given queue.
func NewBrokerService(q queue.Queue) *BrokerService {
	return &BrokerService{
		queue:   q,
		pending: make(map[string]queue.Message),
	}
}

// Publish enqueues a message for the given topic.
func (s *BrokerService) Publish(ctx context.Context, topic string, body []byte) error {
	// TODO: Generate message ID (e.g. UUID). Build queue.Message, call s.queue.Enqueue.
	_ = ctx
	_ = topic
	_ = body
	return nil
}

// Consume pulls one message for the topic and moves it to Pending.
func (s *BrokerService) Consume(ctx context.Context, topic string) (*queue.Message, error) {
	// TODO: Call s.queue.Dequeue in a loop (or single call if queue is per-topic) until
	// a message for topic is found or queue empty. If found, add to pending, return message.
	// If not found, return error (e.g. no message available).
	_ = ctx
	_ = topic
	return nil, nil
}

// Ack removes the message from Pending by ID.
func (s *BrokerService) Ack(ctx context.Context, messageID string) error {
	// TODO: Remove messageID from s.pending. Return error if not found.
	_ = ctx
	_ = messageID
	return nil
}
