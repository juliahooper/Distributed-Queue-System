// Example showing how to use the producer with Azure Blob Storage
// instead of the local file-based log core or stub storage.
//
// Requires: AZURE_STORAGE_CONNECTION_STRING and AZURE_STORAGE_CONTAINER_NAME

package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/distributed-queue-system/broker-replication/client"
	"github.com/distributed-queue-system/broker-replication/internal/azure"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	container := os.Getenv("AZURE_STORAGE_CONTAINER_NAME")
	if connStr == "" {
		log.Fatal("AZURE_STORAGE_CONNECTION_STRING must be set")
	}
	if container == "" {
		container = "broker-logs"
	}

	storageClient, err := azure.NewBlobStorageClient(connStr, container)
	if err != nil {
		log.Fatalf("Failed to create blob storage client: %v", err)
	}

	prod := client.NewProducer("azure-producer", storageClient, client.DefaultRetryPolicy())

	offset, err := prod.Produce(ctx, "example-topic", []byte("key-1"), []byte("hello from Azure!"))
	if err != nil {
		log.Fatalf("produce failed: %v", err)
	}
	log.Printf("produced message at offset %d (stored in Azure Blob Storage)", offset)

	// Read it back
	payload, err := storageClient.Read(ctx, "example-topic", offset)
	if err != nil {
		log.Fatalf("read back failed: %v", err)
	}
	log.Printf("read back %d bytes from blob storage", len(payload))
}
