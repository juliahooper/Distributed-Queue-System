// Package main is the entry point for the broker process.
// Supports two modes:
//   - Local mode (default): in-memory queue, no Azure dependencies
//   - Azure mode: set AZURE_SERVICEBUS_CONNECTION_STRING for Service Bus queue,
//     set AZURE_STORAGE_CONNECTION_STRING to also enable durable WAL to Blob Storage.
//
// Wiring: queue -> service (+ optional WAL) -> api -> Run.

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

	// ── Queue backend ────────────────────────────────────────────────────
	var q queue.Queue
	var queueCleanup func()

	if cfg.HasServiceBus() {
		log.Println("queue backend: Azure Service Bus")
		sbQueue, err := azure.NewServiceBusQueue(
			cfg.ServiceBusConnectionString,
			cfg.ServiceBusQueueName,
		)
		if err != nil {
			log.Fatalf("failed to create Service Bus queue: %v", err)
		}
		q = sbQueue
		queueCleanup = func() {
			if err := sbQueue.Close(context.Background()); err != nil {
				log.Printf("service bus cleanup error: %v", err)
			}
		}
	} else {
		log.Println("queue backend: in-memory (local)")
		q = queue.NewMemoryQueue()
		queueCleanup = func() {}
	}

	// ── WAL (write-ahead log) ────────────────────────────────────────────
	var brokerOpts []service.BrokerOption

	if cfg.HasBlobStorage() {
		log.Println("WAL backend: Azure Blob Storage")
		walClient, err := azure.NewBlobStorageClient(
			cfg.StorageConnectionString,
			cfg.StorageContainerName,
		)
		if err != nil {
			log.Fatalf("failed to create Blob Storage WAL client: %v", err)
		}
		brokerOpts = append(brokerOpts, service.WithWAL(walClient))
	} else {
		log.Println("WAL backend: disabled (no AZURE_STORAGE_CONNECTION_STRING set)")
	}

	// ── Wire broker + API ────────────────────────────────────────────────
	broker := service.NewBrokerService(q, brokerOpts...)
	srv := api.NewServer(broker)

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("received %s, shutting down...", sig)
		queueCleanup()
		os.Exit(0)
	}()

	log.Printf("broker starting on %s", addr)
	if err := srv.Run(addr); err != nil {
		log.Fatalf("broker exited: %v", err)
	}
}

