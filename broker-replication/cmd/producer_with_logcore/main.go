// Example showing how Team Member 2 (Julia) can use Team Member 1's (Rosie's) log core.
// This replaces the StubStorageClient with the real LogStorageClient.

package main

import (
	"context"
	"log"
	"time"

	"github.com/distributed-queue-system/broker-replication/client"
	"github.com/distributed-queue-system/broker-replication/internal/storage"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create Team Member 1's log core
	logCore, err := storage.NewLogCore(".")
	if err != nil {
		log.Fatalf("Failed to create log core: %v", err)
	}
	defer logCore.Close()

	// Create Team Member 1's StorageClient adapter
	storageClient := storage.NewLogStorageClient(logCore)

	// Create Team Member 2's Producer using the real storage
	prod := client.NewProducer("example-producer", storageClient, client.DefaultRetryPolicy())

	// Produce messages
	offset1, err := prod.Produce(ctx, "example-topic", []byte("key-1"), []byte("hello, world"))
	if err != nil {
		log.Fatalf("produce failed: %v", err)
	}
	log.Printf("produced message 1 at offset %d", offset1)

	offset2, err := prod.Produce(ctx, "example-topic", []byte("key-2"), []byte("second message"))
	if err != nil {
		log.Fatalf("produce failed: %v", err)
	}
	log.Printf("produced message 2 at offset %d", offset2)

	// Verify we can read them back using the log core
	payload1, err := logCore.Read(ctx, "example-topic", offset1)
	if err != nil {
		log.Fatalf("read failed: %v", err)
	}
	log.Printf("read back message 1: %d bytes", len(payload1))

	payload2, err := logCore.Read(ctx, "example-topic", offset2)
	if err != nil {
		log.Fatalf("read failed: %v", err)
	}
	log.Printf("read back message 2: %d bytes", len(payload2))

	log.Println("Integration successful! Team Member 1's log core works with Team Member 2's producer.")
}
