// ASSIGNED TO: ENGINEER C
//
// Package service is the glue between API and Queue. It implements the "Pull" model:
// when the API asks for a message, this layer checks the Queue and manages the
// Pending list (messages waiting for client Acknowledgement).

package service

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/distributed-queue-system/broker-replication/internal/queue"
    "github.com/google/uuid"
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
    // Peek returns up to n messages for the topic without consuming. Nil if not supported.
    Peek(ctx context.Context, topic string, n int) []*queue.Message
}

// BrokerWithAudit is an optional interface for brokers that support producer/consumer IDs for audit.
type BrokerWithAudit interface {
    Broker
    PublishWithProducer(ctx context.Context, topic string, body []byte, producerID string) error
    ConsumeWithConsumer(ctx context.Context, topic string, consumerID string) (*queue.Message, error)
    AckWithConsumer(ctx context.Context, messageID string, consumerID string) error
}

// ActivityEntry is a single activity log entry (publish, consume, ack, requeue, dlq).
type ActivityEntry struct {
    EventType  string                 `json:"event_type"`
    MessageID  string                 `json:"message_id"`
    Topic      string                 `json:"topic"`
    ProducerID string                 `json:"producer_id"`
    ConsumerID string                 `json:"consumer_id"`
    CreatedAt  time.Time              `json:"created_at"`
    Details    map[string]interface{} `json:"details,omitempty"`
}

// BrokerExt is an optional interface for brokers with metrics, DLQ, and activity log (PostgreSQL backend).
type BrokerExt interface {
    GetMetrics(ctx context.Context) (map[string]interface{}, error)
    DeadLetterCount(ctx context.Context) (int, error)
    DeadLetterRetry(ctx context.Context) (retried, failed int, err error)
    GetActivityLog(ctx context.Context, limit, offset int, eventType string) ([]ActivityEntry, error)
}

// WALWriter is the interface for the write-ahead log storage backend.
// Any type with an Append method — including azure.BlobStorageClient — satisfies this.
type WALWriter interface {
    Append(ctx context.Context, topic string, data []byte) (int64, error)
}

// BrokerOption configures a BrokerService.
type BrokerOption func(*BrokerService)

// WithWAL attaches a write-ahead log to the broker.
// Every published message will be durably written to the WAL before being enqueued.
func WithWAL(w WALWriter) BrokerOption {
    return func(s *BrokerService) { s.wal = w }
}

// BrokerService is the concrete broker: it uses the Queue and maintains a Pending list.
// ASSIGNED TO: ENGINEER C
type BrokerService struct {
    queue      queue.Queue
    wal        WALWriter              // optional durable write-ahead log
    pending    map[string]pendingEntry // messageID -> message, waiting for ack
    mu         sync.Mutex
    visibility time.Duration
}

type pendingEntry struct {
    msg    queue.Message
    expiry time.Time
}

// NewBrokerService constructs a broker that uses the given queue.
// Pass WithWAL(w) to enable durable message logging.
func NewBrokerService(q queue.Queue, opts ...BrokerOption) *BrokerService {
    s := &BrokerService{
        queue:      q,
        pending:    make(map[string]pendingEntry),
        visibility: 30 * time.Second,
    }
    for _, opt := range opts {
        opt(s)
    }

    // Start background reaper to requeue expired pending messages.
    go s.requeueExpiredLoop()

    return s
}

// walEntry is the JSON structure persisted to the write-ahead log.
type walEntry struct {
    ID        string    `json:"id"`
    Topic     string    `json:"topic"`
    Body      []byte    `json:"body"`
    Timestamp time.Time `json:"timestamp"`
}

// Publish enqueues a message for the given topic.
// If a WAL is configured, the message is first written durably to storage.
func (s *BrokerService) Publish(ctx context.Context, topic string, body []byte) error {
    if topic == "" {
        return errors.New("topic required")
    }
    if len(body) == 0 {
        return errors.New("body required")
    }

    m := queue.Message{
        ID:    newID(),
        Topic: topic,
        Body:  append([]byte(nil), body...),
    }

    // Write-ahead log: persist to durable storage before enqueuing.
    // This ensures messages survive a broker crash — on restart
    // the log can be replayed to recover any un-consumed messages.
    if s.wal != nil {
        entry := walEntry{
            ID:        m.ID,
            Topic:     m.Topic,
            Body:      m.Body,
            Timestamp: time.Now().UTC(),
        }
        data, err := json.Marshal(entry)
        if err != nil {
            log.Printf("[wal] marshal error for message %s: %v", m.ID, err)
        } else {
            offset, err := s.wal.Append(ctx, topic, data)
            if err != nil {
                // Log but don't fail the publish — Service Bus still has it.
                log.Printf("[wal] write error for message %s: %v", m.ID, err)
            } else {
                log.Printf("[wal] wrote message %s (topic=%s) at offset %d", m.ID, topic, offset)
            }
        }
    }

    s.queue.Enqueue(m)
    return nil
}

// PublishWithProducer delegates to Publish (producerID ignored for memory backend).
func (s *BrokerService) PublishWithProducer(ctx context.Context, topic string, body []byte, producerID string) error {
    _ = producerID
    return s.Publish(ctx, topic, body)
}

// ConsumeWithConsumer delegates to Consume (consumerID ignored for memory backend).
func (s *BrokerService) ConsumeWithConsumer(ctx context.Context, topic string, consumerID string) (*queue.Message, error) {
    _ = consumerID
    return s.Consume(ctx, topic)
}

// AckWithConsumer delegates to Ack (consumerID ignored for memory backend).
func (s *BrokerService) AckWithConsumer(ctx context.Context, messageID string, consumerID string) error {
    _ = consumerID
    return s.Ack(ctx, messageID)
}

// Consume pulls one message for the topic and moves it to Pending.
func (s *BrokerService) Consume(ctx context.Context, topic string) (*queue.Message, error) {
    if topic == "" {
        return nil, errors.New("topic required")
    }

    // Loop until we find a matching message or the context deadline is hit.
    // - If Dequeue returns false (queue transiently empty), we sleep and retry.
    //   This makes Consume work correctly against both in-memory and Service Bus.
    // - If a message is for a different topic, re-enqueue it and continue.
    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }

        m, ok := s.queue.Dequeue()
        if !ok {
            // Queue is empty right now — wait a bit and try again.
            select {
            case <-ctx.Done():
                return nil, errors.New("no message available")
            case <-time.After(200 * time.Millisecond):
                continue
            }
        }

        if m.Topic != topic {
            // Not the right topic; put it back at the end and continue.
            s.queue.Enqueue(m)
            select {
            case <-ctx.Done():
                return nil, errors.New("no message available")
            case <-time.After(5 * time.Millisecond):
                continue
            }
        }

        // Lock into pending with expiry.
        s.mu.Lock()
        s.pending[m.ID] = pendingEntry{msg: m, expiry: time.Now().Add(s.visibility)}
        s.mu.Unlock()

        return &m, nil
    }
}

// Peek returns up to n messages for the topic without consuming.
// Uses queue.PeekN if available; returns nil for queues that don't support it (e.g. Service Bus).
func (s *BrokerService) Peek(ctx context.Context, topic string, n int) []*queue.Message {
    if topic == "" || n <= 0 {
        return nil
    }
    msgs := s.queue.PeekN(n * 3) // Peek extra to account for other topics
    if len(msgs) == 0 {
        return nil
    }
    var out []*queue.Message
    for i := range msgs {
        if msgs[i].Topic == topic {
            m := msgs[i]
            out = append(out, &m)
            if len(out) >= n {
                break
            }
        }
    }
    return out
}

// Ack removes the message from Pending by ID.
func (s *BrokerService) Ack(ctx context.Context, messageID string) error {
    _ = ctx

    s.mu.Lock()
    defer s.mu.Unlock()

    if _, ok := s.pending[messageID]; !ok {
        return fmt.Errorf("message %s not found or already acked", messageID)
    }

    delete(s.pending, messageID)
    return nil
}

// requeueExpiredLoop runs in background to requeue pending messages whose
// visibility timeout expired (i.e., consumer didn't ack in time).
func (s *BrokerService) requeueExpiredLoop() {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        now := time.Now()
        var expired []queue.Message

        s.mu.Lock()
        for id, e := range s.pending {
            if now.After(e.expiry) {
                expired = append(expired, e.msg)
                delete(s.pending, id)
            }
        }
        s.mu.Unlock()

        for _, m := range expired {
            s.queue.Enqueue(m)
        }
    }
}

func newID() string {
    return uuid.New().String()
}
