package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/distributed-queue-system/broker-replication/internal/queue"
)

func TestBrokerService_MultipleConsumersGetUniqueMessages(t *testing.T) {
	q := queue.NewMemoryQueue()
	broker := NewBrokerService(q)

	const topic = "triage"
	const messageCount = 12
	const consumerCount = 3

	for i := 0; i < messageCount; i++ {
		body := []byte(fmt.Sprintf("patient-%d", i+1))
		if err := broker.Publish(context.Background(), topic, body); err != nil {
			t.Fatalf("Publish failed: %v", err)
		}
	}

	var seenMu sync.Mutex
	seen := make(map[string]bool)

	var consumed atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < consumerCount; i++ {
		wg.Add(1)

		go func(worker int) {
			defer wg.Done()

			for {
				if int(consumed.Load()) >= messageCount {
					return
				}

				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				msg, err := broker.Consume(ctx, topic)
				cancel()

				if err != nil {
					if int(consumed.Load()) >= messageCount {
						return
					}
					continue
				}

				if msg == nil {
					continue
				}

				seenMu.Lock()
				if seen[msg.ID] {
					seenMu.Unlock()
					t.Errorf("worker %d got duplicate message id %s", worker, msg.ID)
					return
				}
				seen[msg.ID] = true
				seenMu.Unlock()

				if err := broker.Ack(context.Background(), msg.ID); err != nil {
					t.Errorf("worker %d failed to ack message %s: %v", worker, msg.ID, err)
					return
				}

				consumed.Add(1)
			}
		}(i + 1)
	}

	wg.Wait()

	if got := int(consumed.Load()); got != messageCount {
		t.Fatalf("expected %d consumed messages, got %d", messageCount, got)
	}

	seenMu.Lock()
	defer seenMu.Unlock()

	if len(seen) != messageCount {
		t.Fatalf("expected %d unique messages, got %d", messageCount, len(seen))
	}
}

func TestBrokerService_AckRemovesPending(t *testing.T) {
	q := queue.NewMemoryQueue()
	broker := NewBrokerService(q)

	if err := broker.Publish(context.Background(), "triage", []byte("patient-1")); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	msg, err := broker.Consume(ctx, "triage")
	cancel()
	if err != nil {
		t.Fatalf("Consume failed: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message, got nil")
	}

	if err := broker.Ack(context.Background(), msg.ID); err != nil {
		t.Fatalf("Ack failed: %v", err)
	}

	if err := broker.Ack(context.Background(), msg.ID); err == nil {
		t.Fatal("expected second Ack to fail, got nil")
	}
}
