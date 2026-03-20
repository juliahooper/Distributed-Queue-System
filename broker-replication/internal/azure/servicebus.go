package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"github.com/distributed-queue-system/broker-replication/internal/queue"
)

// serviceBusMessage is the JSON structure stored in Service Bus message bodies.
type serviceBusMessage struct {
	ID    string `json:"id"`
	Topic string `json:"topic"`
	Body  []byte `json:"body"`
}

// ServiceBusQueue implements queue.Queue using Azure Service Bus.
//
// Design note on settlement:
//   - Enqueue sends a message to Service Bus.
//   - Dequeue receives a message and IMMEDIATELY completes (acks) it from
//     Service Bus. The broker's own pending/ack system handles re-delivery
//     if the consumer doesn't call /ack — the broker will re-enqueue.
//
// This keeps the interface simple and compatible with the in-memory MemoryQueue.
type ServiceBusQueue struct {
	client    *azservicebus.Client
	sender    *azservicebus.Sender
	queueName string
}

// NewServiceBusQueue creates a new queue backed by Azure Service Bus.
func NewServiceBusQueue(connectionString, queueName string) (*ServiceBusQueue, error) {
	client, err := azservicebus.NewClientFromConnectionString(connectionString, nil)
	if err != nil {
		return nil, fmt.Errorf("create service bus client: %w", err)
	}

	sender, err := client.NewSender(queueName, nil)
	if err != nil {
		return nil, fmt.Errorf("create service bus sender: %w", err)
	}

	return &ServiceBusQueue{
		client:    client,
		sender:    sender,
		queueName: queueName,
	}, nil
}

// Enqueue sends a message to Azure Service Bus.
func (q *ServiceBusQueue) Enqueue(m queue.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := json.Marshal(serviceBusMessage{
		ID:    m.ID,
		Topic: m.Topic,
		Body:  m.Body,
	})
	if err != nil {
		log.Printf("[servicebus] marshal error for message %s: %v", m.ID, err)
		return
	}

	sbMsg := &azservicebus.Message{
		Body: data,
		ApplicationProperties: map[string]interface{}{
			"topic": m.Topic,
		},
	}

	if err := q.sender.SendMessage(ctx, sbMsg, nil); err != nil {
		log.Printf("[servicebus] send error for message %s: %v", m.ID, err)
	} else {
		log.Printf("[servicebus] enqueued message %s (topic=%s)", m.ID, m.Topic)
	}
}

// Dequeue receives one message from Azure Service Bus.
// It completes (removes) the message from Service Bus immediately.
// The broker service's own pending/visibility-timeout system handles
// redelivery if the consumer misses the ack window.
// Returns (zero Message, false) if no message is available.
func (q *ServiceBusQueue) Dequeue() (queue.Message, bool) {
	// Create a fresh receiver each call so we're not holding an idle link.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	receiver, err := q.client.NewReceiverForQueue(q.queueName, nil)
	if err != nil {
		log.Printf("[servicebus] create receiver error: %v", err)
		return queue.Message{}, false
	}
	defer receiver.Close(ctx)

	messages, err := receiver.ReceiveMessages(ctx, 1, nil)
	if err != nil {
		// Context deadline is normal when queue is empty.
		if ctx.Err() != nil {
			return queue.Message{}, false
		}
		log.Printf("[servicebus] receive error: %v", err)
		return queue.Message{}, false
	}

	if len(messages) == 0 {
		return queue.Message{}, false
	}

	received := messages[0]

	var msg serviceBusMessage
	if err := json.Unmarshal(received.Body, &msg); err != nil {
		log.Printf("[servicebus] unmarshal error: %v — dead-lettering", err)
		_ = receiver.DeadLetterMessage(ctx, received, nil)
		return queue.Message{}, false
	}

	// Complete the message (remove from Service Bus).
	if err := receiver.CompleteMessage(ctx, received, nil); err != nil {
		log.Printf("[servicebus] complete error for message %s: %v", msg.ID, err)
		// Don't return false — we still got the message; the broker's
		// visibility timeout will handle stale pending entries.
	}

	log.Printf("[servicebus] dequeued message %s (topic=%s)", msg.ID, msg.Topic)

	return queue.Message{
		ID:    msg.ID,
		Topic: msg.Topic,
		Body:  msg.Body,
	}, true
}

// PeekN returns nil (not supported for Service Bus in current design).
func (q *ServiceBusQueue) PeekN(n int) []queue.Message {
	return nil
}

// Close releases the sender and client.
func (q *ServiceBusQueue) Close(ctx context.Context) error {
	var firstErr error
	if err := q.sender.Close(ctx); err != nil {
		firstErr = fmt.Errorf("close sender: %w", err)
	}
	if err := q.client.Close(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close client: %w", err)
	}
	return firstErr
}
