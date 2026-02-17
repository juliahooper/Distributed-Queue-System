package main

import (
	"context"
	"log"
	"time"

	"github.com/distributed-queue-system/broker-replication/client"
	"github.com/distributed-queue-system/broker-replication/internal/storage"
)

// This example shows how an application can use the Consumer client to read
// messages from Team Member 1's log core and process them with a worker pool.
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set up the log core (storage) as Team Member 1 provides.
	logCore, err := storage.NewLogCore(".")
	if err != nil {
		log.Fatalf("failed to create log core: %v", err)
	}
	defer logCore.Close()

	// Adapter implementing client.StorageReader using the log core.
	storageReader := &logCoreReader{core: logCore}

	// Build a consumer configuration.
	consumer, err := client.NewConsumer(client.ConsumerConfig{
		ID:          "example-consumer",
		Topic:       "example-topic",
		Storage:     storageReader,
		OffsetStore: client.NewInMemoryOffsetStore(),
		Handler: func(ctx context.Context, topic string, key, value []byte) error {
			log.Printf("processed message on topic=%s key=%s value=%s", topic, string(key), string(value))
			return nil
		},
		RetryPolicy: client.DefaultRetryPolicy(),
		WorkerCount: 4,
		// PollInterval left at zero to use default.
	})
	if err != nil {
		log.Fatalf("failed to create consumer: %v", err)
	}

	// Start the consumer. It will run until the context is cancelled or the
	// timeout elapses.
	if err := consumer.Start(ctx); err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		log.Fatalf("consumer stopped with error: %v", err)
	}
}

// logCoreReader is a tiny adapter that lets the consumer talk to the log
// core's Read API through the StorageReader interface.
type logCoreReader struct {
	core *storage.LogCore
}

func (r *logCoreReader) Read(ctx context.Context, topic string, offset int64) ([]byte, error) {
	return r.core.Read(ctx, topic, offset)
}

