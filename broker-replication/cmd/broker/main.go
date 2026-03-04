// Package main is the entry point for the broker process.
// Supports two modes:
//   - Local mode (default): in-memory queue, no Azure dependencies
//   - Azure mode: set AZURE_SERVICEBUS_CONNECTION_STRING to use Azure Service Bus
//
// Wiring: queue -> service -> api -> Run.

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/distributed-queue-system/broker-replication/internal/api"
	"github.com/distributed-queue-system/broker-replication/internal/azure"
	"github.com/distributed-queue-system/broker-replication/internal/queue"
	"github.com/distributed-queue-system/broker-replication/internal/service"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("BROKER_ADDR"); v != "" {
		addr = v
	}

	cfg, _ := azure.LoadConfigFromEnv()

	var q queue.Queue
	var cleanup func()

	if cfg.HasServiceBus() {
		// Azure mode: use Azure Service Bus as the queue backend.
		log.Println("mode: Azure Service Bus")
		sbQueue, err := azure.NewServiceBusQueue(
			cfg.ServiceBusConnectionString,
			cfg.ServiceBusQueueName,
		)
		if err != nil {
			log.Fatalf("failed to create Service Bus queue: %v", err)
		}
		q = sbQueue
		cleanup = func() {
			if err := sbQueue.Close(context.Background()); err != nil {
				log.Printf("service bus cleanup error: %v", err)
			}
		}
	} else {
		// Local mode: in-memory queue.
		log.Println("mode: in-memory (local)")
		q = queue.NewMemoryQueue()
		cleanup = func() {}
	}

	broker := service.NewBrokerService(q)
	srv := api.NewServer(broker)

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("received %s, shutting down...", sig)
		cleanup()
		os.Exit(0)
	}()

	log.Printf("broker starting on %s", addr)
	if err := srv.Run(addr); err != nil {
		log.Fatalf("broker exited: %v", err)
	}
}
