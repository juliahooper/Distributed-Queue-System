package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// StorageReader is the minimal interface the consumer needs from the storage
// / log-core layer. Team 1 can provide a concrete implementation that calls
// LogCore.Read.
type StorageReader interface {
	// Read returns the raw payload bytes for the given topic and offset.
	Read(ctx context.Context, topic string, offset int64) ([]byte, error)
}

// OffsetStore abstracts how the consumer tracks committed offsets so it can
// resume after a restart. Implementations can be in-memory, file-based, DB,
// etc. For at-least-once semantics, offsets should be committed only after the
// message has been successfully processed.
type OffsetStore interface {
	// LoadOffset returns the last committed offset for the given consumer and
	// topic. If no offset has been stored yet, it should return 0 and a nil
	// error.
	LoadOffset(ctx context.Context, consumerID, topic string) (int64, error)

	// SaveOffset persists the last successfully processed offset for the given
    // consumer and topic.
	SaveOffset(ctx context.Context, consumerID, topic string, offset int64) error
}

// InMemoryOffsetStore is a simple OffsetStore implementation that keeps
// offsets in memory only. It is suitable for local development and tests.
// Offsets are not durable across process restarts, but messages are still not
// lost because the underlying log is append-only.
type InMemoryOffsetStore struct {
	mu      sync.Mutex
	offsets map[string]int64 // key: consumerID + "|" + topic
}

// NewInMemoryOffsetStore constructs a new in-memory offset store.
func NewInMemoryOffsetStore() *InMemoryOffsetStore {
	return &InMemoryOffsetStore{
		offsets: make(map[string]int64),
	}
}

// LoadOffset returns the last committed offset for this consumer/topic pair.
func (s *InMemoryOffsetStore) LoadOffset(_ context.Context, consumerID, topic string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := consumerID + "|" + topic
	return s.offsets[key], nil
}

// SaveOffset records the last committed offset for this consumer/topic pair.
func (s *InMemoryOffsetStore) SaveOffset(_ context.Context, consumerID, topic string, offset int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := consumerID + "|" + topic
	s.offsets[key] = offset
	return nil
}

// HandlerFunc is the application-provided function that processes each
// message. If it returns an error, the consumer will retry according to the
// configured RetryPolicy.
type HandlerFunc func(ctx context.Context, topic string, key, value []byte) error

// ConsumerConfig configures a Consumer instance.
type ConsumerConfig struct {
	// ID uniquely identifies this consumer instance. It is used as part of the
	// key for offset tracking so multiple consumers can read the same topic
	// independently.
	ID string

	// Topic is the topic this consumer will read from.
	Topic string

	// Storage is the underlying storage/log client used to read messages by
	// offset.
	Storage StorageReader

	// OffsetStore is where committed offsets are stored. If nil,
	// NewInMemoryOffsetStore() is used.
	OffsetStore OffsetStore

	// Handler is the application callback invoked for each message.
	Handler HandlerFunc

	// RetryPolicy controls how the consumer retries handler failures. If zero,
	// DefaultRetryPolicy() is used.
	RetryPolicy RetryPolicy

	// WorkerCount is the size of the worker pool processing messages. If zero
	// or negative, a default of 1 is used.
	WorkerCount int

	// PollInterval is how long the poll loop waits before re-checking for a
	// new message when none is available. If zero, a small default is used.
	PollInterval time.Duration
}

// Consumer reads messages for a single topic from storage using a poll loop
// and fan-outs them to a worker pool for processing.
type Consumer struct {
	cfg ConsumerConfig

	workCh chan delivery
	wg     sync.WaitGroup
}

type delivery struct {
	offset int64
	data   []byte
}

// NewConsumer constructs a Consumer with the given configuration. It validates
// required fields and fills in reasonable defaults where needed.
func NewConsumer(cfg ConsumerConfig) (*Consumer, error) {
	if cfg.ID == "" {
		return nil, errors.New("consumer: ID must not be empty")
	}
	if cfg.Topic == "" {
		return nil, errors.New("consumer: Topic must not be empty")
	}
	if cfg.Storage == nil {
		return nil, errors.New("consumer: Storage must not be nil")
	}
	if cfg.Handler == nil {
		return nil, errors.New("consumer: Handler must not be nil")
	}
	if cfg.OffsetStore == nil {
		cfg.OffsetStore = NewInMemoryOffsetStore()
	}
	if cfg.RetryPolicy.MaxAttempts == 0 && cfg.RetryPolicy.BaseBackoff == 0 && cfg.RetryPolicy.MaxBackoff == 0 {
		cfg.RetryPolicy = DefaultRetryPolicy()
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 1
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 50 * time.Millisecond
	}

	return &Consumer{
		cfg:    cfg,
		workCh: make(chan delivery, cfg.WorkerCount*2),
	}, nil
}

// Start begins the consumer's poll loop and worker pool. It blocks until the
// provided context is cancelled or an unrecoverable error occurs. Clean
// shutdown is achieved by cancelling the context; Start will wait for all in-
// flight messages to finish processing.
func (c *Consumer) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("consumer: context must not be nil")
	}

	// Determine starting offset (at-least-once): we start at lastCommitted+1.
	lastCommitted, err := c.cfg.OffsetStore.LoadOffset(ctx, c.cfg.ID, c.cfg.Topic)
	if err != nil {
		return fmt.Errorf("consumer: load offset: %w", err)
	}
	nextOffset := lastCommitted + 1
	if nextOffset <= 0 {
		nextOffset = 1
	}

	// Start workers.
	for i := 0; i < c.cfg.WorkerCount; i++ {
		c.wg.Add(1)
		go c.worker(ctx)
	}

	// Poll loop (single goroutine).
	pollErr := c.pollLoop(ctx, nextOffset)

	// Signal workers to stop and wait for in-flight processing.
	close(c.workCh)
	c.wg.Wait()

	return pollErr
}

// pollLoop continuously reads from storage starting at the given offset and
// pushes deliveries into the work channel. It runs in a single goroutine to
// preserve the natural ordering of offsets, while workers can process in
// parallel.
func (c *Consumer) pollLoop(ctx context.Context, startOffset int64) error {
	current := startOffset

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Attempt to read the next offset. Any error is treated as "no message
		// available yet" for now; Team 1 can refine error types to distinguish
		// between transient failures and end-of-log conditions.
		payload, err := c.cfg.Storage.Read(ctx, c.cfg.Topic, current)
		if err != nil {
			// Back off briefly before trying again.
			timer := time.NewTimer(c.cfg.PollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}

		d := delivery{
			offset: current,
			data:   payload,
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case c.workCh <- d:
			// Successfully queued this message for processing; move to next offset.
			current++
		}
	}
}

// worker processes messages from the work channel using the configured
// HandlerFunc. It uses the shared RetryPolicy to implement at-least-once
// semantics (best-effort within a single process run) and commits offsets only
// after successful handling.
func (c *Consumer) worker(ctx context.Context) {
	defer c.wg.Done()

	for d := range c.workCh {
		// Decode frame into logical message.
		key, value, err := decodeMessage(d.data)
		if err != nil {
			// If we cannot decode the message, we can't meaningfully process it.
			// Skip it but still advance the committed offset to avoid an
			// infinite decode loop on a corrupted entry.
			_ = c.cfg.OffsetStore.SaveOffset(ctx, c.cfg.ID, c.cfg.Topic, d.offset)
			continue
		}

		// Apply retry policy around the handler execution.
		handlerErr := withRetry(ctx, c.cfg.RetryPolicy, func() error {
			return c.cfg.Handler(ctx, c.cfg.Topic, key, value)
		})
		if handlerErr != nil {
			// At-least-once within this run: we've retried. We deliberately do
			// NOT advance the committed offset so that a future restart can
			// re-process this message from the log.
			continue
		}

		// Handler succeeded; commit offset.
		_ = c.cfg.OffsetStore.SaveOffset(ctx, c.cfg.ID, c.cfg.Topic, d.offset)
	}
}

