package client

import (
	"context"
	"errors"
)

// Producer is the public interface for sending messages to the broker.
// It hides serialization, retries, and the underlying storage/broker details.
type Producer interface {
	// Produce sends a message for the given topic. It returns the assigned
	// offset from the storage layer (if available) or 0 if the implementation
	// does not track offsets yet.
	Produce(ctx context.Context, topic string, key, value []byte) (offset int64, err error)
}

// StorageClient is the integration point between the producer and the
// underlying log core / broker. Team 1 will provide a concrete implementation
// later; for now we define the minimal interface we need.
//
// TODO(team1): Wire this interface to the real log core append API.
type StorageClient interface {
	// Append sends a single serialized record for the given topic to storage.
	// It returns the offset assigned to the record, if known.
	Append(ctx context.Context, topic string, data []byte) (offset int64, err error)
}

// producer is the concrete implementation of Producer.
type producer struct {
	id          string
	conn        StorageClient
	retryPolicy RetryPolicy
}

// ErrInvalidTopic is returned when a Produce call is made with an empty topic.
var ErrInvalidTopic = errors.New("topic must not be empty")

// NewProducer constructs a new Producer with the given identifier, storage
// client connection, and retry policy.
func NewProducer(id string, conn StorageClient, retry RetryPolicy) Producer {
	return &producer{
		id:          id,
		conn:        conn,
		retryPolicy: retry,
	}
}

// Produce serializes the message and appends it to the underlying storage
// using the configured retry policy.
func (p *producer) Produce(ctx context.Context, topic string, key, value []byte) (int64, error) {
	if topic == "" {
		return 0, ErrInvalidTopic
	}

	framed, err := encodeMessage(key, value)
	if err != nil {
		return 0, err
	}

	var offset int64
	err = withRetry(ctx, p.retryPolicy, func() error {
		o, appendErr := p.conn.Append(ctx, topic, framed)
		if appendErr != nil {
			return appendErr
		}
		offset = o
		return nil
	})
	if err != nil {
		return 0, err
	}
	return offset, nil
}


