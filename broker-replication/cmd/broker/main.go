// Package main is the entry point for the broker process.
// Supports two modes:
//   - Local mode (default): in-memory queue, no Azure dependencies
//   - Azure mode: set AZURE_SERVICEBUS_CONNECTION_STRING for Service Bus queue,
//     set AZURE_STORAGE_CONNECTION_STRING to also enable durable WAL to Blob Storage.
//
// Frontend uses broker API: /publish, /peek, /consume, /ack (topic er-queue).

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/distributed-queue-system/broker-replication/internal/api"
	"github.com/distributed-queue-system/broker-replication/internal/azure"
	"github.com/distributed-queue-system/broker-replication/internal/queue"
	"github.com/distributed-queue-system/broker-replication/internal/service"
	"github.com/distributed-queue-system/broker-replication/internal/storage"
	"github.com/distributed-queue-system/broker-replication/internal/storage/postgres"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("BROKER_ADDR"); v != "" {
		addr = v
	}

	cfg, _ := azure.LoadConfigFromEnv()
	postgresDSN := os.Getenv("POSTGRES_DSN")

	// ── Queue backend ────────────────────────────────────────────────────
	var broker service.Broker
	var queueCleanup func()

	if postgresDSN != "" {
		log.Println("queue backend: PostgreSQL")
		ctx := context.Background()
		store, err := postgres.NewStore(ctx, postgresDSN)
		if err != nil {
			log.Fatalf("failed to create Postgres store: %v", err)
		}
		if err := store.RunMigrations(ctx); err != nil {
			log.Fatalf("failed to run migrations: %v", err)
		}
		broker = service.NewPostgresBroker(store)
		queueCleanup = func() { store.Close() }
	} else if cfg.HasServiceBus() {
		log.Println("queue backend: Azure Service Bus")
		sbQueue, err := azure.NewServiceBusQueue(
			cfg.ServiceBusConnectionString,
			cfg.ServiceBusQueueName,
		)
		if err != nil {
			log.Fatalf("failed to create Service Bus queue: %v", err)
		}
		q := sbQueue
		broker = service.NewBrokerService(q)
		queueCleanup = func() {
			if err := sbQueue.Close(context.Background()); err != nil {
				log.Printf("service bus cleanup error: %v", err)
			}
		}
	} else {
		log.Println("queue backend: in-memory (local)")
		q := queue.NewMemoryQueue()
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
			log.Println("WAL backend: LogCore (internal/storage, data/logs/)")
			logCore, err := storage.NewLogCore(".")
			if err != nil {
				log.Fatalf("failed to create LogCore: %v", err)
			}
			brokerOpts = append(brokerOpts, service.WithWAL(storage.NewLogStorageClient(logCore)))
		}
		broker = service.NewBrokerService(q, brokerOpts...)
		queueCleanup = func() {}
	}
	apiServer := api.NewServer(broker)

	mux := http.NewServeMux()
	apiServer.Register(mux)

	handler := cors(mux)

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
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("broker exited: %v", err)
	}
}

// cors wraps h to add CORS headers for browser clients (e.g. frontend on another port).
func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
