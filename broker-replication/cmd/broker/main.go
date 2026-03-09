// Package main is the entry point for the single-node broker process.
// Wiring: queue -> service -> api, plus ER queue and handlers; one mux with CORS.

package main

import (
	"log"
	"net/http"

	"github.com/distributed-queue-system/broker-replication/internal/api"
	"github.com/distributed-queue-system/broker-replication/internal/er"
	"github.com/distributed-queue-system/broker-replication/internal/queue"
	"github.com/distributed-queue-system/broker-replication/internal/service"
)

func main() {
	q := queue.NewMemoryQueue()
	broker := service.NewBrokerService(q)
	apiServer := api.NewServer(broker)

	erQueue, err := er.LoadQueue("")
	if err != nil {
		log.Fatalf("load ER queue: %v", err)
	}
	erHandlers := er.NewHandlers(erQueue)

	mux := http.NewServeMux()
	apiServer.Register(mux)
	erHandlers.Register(mux, "/er")

	handler := cors(mux)
	_ = http.ListenAndServe(":8080", handler)
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
