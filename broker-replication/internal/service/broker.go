// ASSIGNED TO: ENGINEER C
//
// Package service is the glue between API and Queue. It implements the "Pull" model:
// when the API asks for a message, this layer checks the Queue and manages the
// Pending list (messages waiting for client Acknowledgement).

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
	queue queue.Queue

	mu      sync.Mutex
	pending map[string]queue.Message // messageID -> message, waiting for ack
}

// NewBrokerService constructs a broker that uses the given queue.
func NewBrokerService(q queue.Queue) *BrokerService {
	return &BrokerService{
		queue: q,
		pending: make(map[string]queue.Message),
	}
}

// Publish enqueues a message for the given topic.
func (s *BrokerService) Publish(ctx context.Context, topic string, body []byte) error {
	if topic == "" {
		return errors.New("topic must not be empty")
	}
	if len(body) == 0 {
		return errors.New("body must not be empty")
	}

	// Check for cancellation early.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// For the single-node prototype, we generate a simple unique ID by using
	// the address of the message struct combined with the topic. This avoids
	// adding external dependencies while still giving each message a distinct ID.
	msg := queue.Message{
		ID:    "", // filled in below
		Topic: topic,
		Body:  body,
	}
	msg.ID = fmt.Sprintf("%s-%p", topic, &msg)

	s.queue.Enqueue(msg)
	return nil
}

// Consume pulls one message for the topic and moves it to Pending.
func (s *BrokerService) Consume(ctx context.Context, topic string) (*queue.Message, error) {
	if topic == "" {
		return nil, errors.New("topic must not be empty")
	}

	for {
		// Respect context cancellation.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		msg, ok := s.queue.Dequeue()
		if !ok {
			// No messages available at all.
			return nil, errors.New("no messages available")
		}

		// If this message is for a different topic, skip it for now by placing
		// it back at the end of the queue. For the single-queue prototype this
		// is simple and keeps behaviour correct enough for one node.
		if msg.Topic != topic {
			s.queue.Enqueue(msg)
			// Continue the loop to look for a message of the requested topic.
			continue
		}

		// Move to pending and return.
		s.mu.Lock()
		s.pending[msg.ID] = msg
		s.mu.Unlock()

		return &msg, nil
	}
}

// Ack removes the message from Pending by ID.
func (s *BrokerService) Ack(ctx context.Context, messageID string) error {
	if messageID == "" {
		return errors.New("message ID must not be empty")
	}

	// Respect context cancellation.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.pending[messageID]; !ok {
		return errors.New("message not found in pending")
	}

	delete(s.pending, messageID)
	return nil
}
