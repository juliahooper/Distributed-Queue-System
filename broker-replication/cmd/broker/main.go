// Package main is the entry point for the single-node broker process.
// Wiring is done here: queue -> service -> api -> Run.

package main

import (
	"log"
	"os"

	"github.com/distributed-queue-system/broker-replication/internal/api"
	"github.com/distributed-queue-system/broker-replication/internal/queue"
	"github.com/distributed-queue-system/broker-replication/internal/service"
)

func main() {
	// Create the in-memory queue for this single-node broker.
	q := queue.NewMemoryQueue()

	// Create the broker service that glues the queue to the API layer.
	broker := service.NewBrokerService(q)

	// Create the HTTP server.
	srv := api.NewServer(broker)

	addr := ":8080"
	if fromEnv := os.Getenv("BROKER_HTTP_ADDR"); fromEnv != "" {
		addr = fromEnv
	}

	log.Printf("starting broker HTTP server on %s", addr)
	if err := srv.Run(addr); err != nil {
		log.Fatalf("broker server exited with error: %v", err)
	}
}
