package client

import (
	"context"
	"errors"
	"sync"
)

// StubStorageClient is a temporary in-memory StorageClient implementation
// that assigns monotonically increasing offsets. This is only for local
// testing until the real log core is available.
//
// TODO(team1): replace this stub with a real implementation that calls the
// log core's append function.
type StubStorageClient struct {
	mu     sync.Mutex
	offset int64
}

// NewStubStorageClient constructs a new StubStorageClient.
func NewStubStorageClient() *StubStorageClient {
	return &StubStorageClient{}
}

// Append increments the internal offset counter and returns the new offset.
// It ignores the topic and data for now but validates that some data exists.
func (s *StubStorageClient) Append(ctx context.Context, topic string, data []byte) (int64, error) {
	_ = ctx

	if topic == "" {
		return 0, ErrInvalidTopic
	}
	if len(data) == 0 {
		return 0, errors.New("cannot append empty payload")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.offset++
	return s.offset, nil
}

