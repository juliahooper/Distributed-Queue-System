package client

import (
	"context"
	"errors"
	"testing"
	"time"
)

type flakyStorage struct {
	failuresRemaining int
	calls             int
	offset            int64
}

func (f *flakyStorage) Append(ctx context.Context, topic string, data []byte) (int64, error) {
	_ = ctx
	f.calls++
	if f.failuresRemaining > 0 {
		f.failuresRemaining--
		return 0, errors.New("temporary failure")
	}
	f.offset++
	return f.offset, nil
}

func TestProducer_RetrySuccess(t *testing.T) {
	ctx := context.Background()

	storage := &flakyStorage{
		failuresRemaining: 2,
	}
	policy := RetryPolicy{
		MaxAttempts: 5,
		BaseBackoff: 1 * time.Millisecond,
		MaxBackoff:  5 * time.Millisecond,
	}

	p := NewProducer("test", storage, policy)

	offset, err := p.Produce(ctx, "topic", []byte("k"), []byte("v"))
	if err != nil {
		t.Fatalf("Produce returned error: %v", err)
	}
	if offset != 1 {
		t.Fatalf("expected offset 1, got %d", offset)
	}
	if storage.calls < 3 {
		t.Fatalf("expected at least 3 calls (2 failures + 1 success), got %d", storage.calls)
	}
}

func TestProducer_InvalidTopic(t *testing.T) {
	ctx := context.Background()
	p := NewProducer("test", NewStubStorageClient(), DefaultRetryPolicy())

	if _, err := p.Produce(ctx, "", []byte("k"), []byte("v")); err == nil {
		t.Fatalf("expected error for empty topic, got nil")
	}
}

