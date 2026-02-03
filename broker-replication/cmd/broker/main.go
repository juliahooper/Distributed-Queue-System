// Package main is the entry point for the single-node broker process.
// Wiring is done here: queue -> service -> api -> Run.

package main

import (
	"github.com/distributed-queue-system/broker-replication/internal/api"
	"github.com/distributed-queue-system/broker-replication/internal/queue"
	"github.com/distributed-queue-system/broker-replication/internal/service"
)

func main() {
	// TODO: q := queue.NewMemoryQueue()
	// TODO: broker := service.NewBrokerService(q)
	// TODO: srv := api.NewServer(broker)
	// TODO: _ = srv.Run(":8080")
	_, _, _ = queue.NewMemoryQueue, service.NewBrokerService, api.NewServer
	panic("implement me")
}
