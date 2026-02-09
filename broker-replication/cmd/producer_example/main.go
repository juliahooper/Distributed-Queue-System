package main

import (
	"context"
	"log"
	"time"

	"github.com/distributed-queue-system/broker-replication/client"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	storage := client.NewStubStorageClient()
	prod := client.NewProducer("example-producer", storage, client.DefaultRetryPolicy())

	offset, err := prod.Produce(ctx, "example-topic", []byte("key-1"), []byte("hello, world"))
	if err != nil {
		log.Fatalf("produce failed: %v", err)
	}

	log.Printf("produced message at offset %d", offset)
}

