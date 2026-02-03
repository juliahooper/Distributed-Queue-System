// ASSIGNED TO: ENGINEER A
//
// Package api defines the HTTP/REST entry points for the broker: POST /publish,
// GET /consume. Responsibility: decode JSON, call the Service layer, return HTTP status codes.

package api

import (
	"net/http"

	"github.com/distributed-queue-system/broker-replication/internal/service"
)

// PublishRequest is the JSON body for POST /publish.
type PublishRequest struct {
	Topic string `json:"topic"`
	Body  []byte `json:"body"`
}

// ConsumeResponse is the JSON body returned for GET /consume.
type ConsumeResponse struct {
	ID    string `json:"id"`
	Topic string `json:"topic"`
	Body  []byte `json:"body"`
}

// Server is the HTTP server that exposes the broker API.
// ASSIGNED TO: ENGINEER A
type Server struct {
	broker service.Broker
	// TODO: optional config (addr, timeouts, etc.)
}

// NewServer creates an API server that uses the given broker service.
func NewServer(broker service.Broker) *Server {
	return &Server{broker: broker}
}

// Run starts the HTTP server and blocks until it exits.
func (s *Server) Run(addr string) error {
	// TODO: Register routes (POST /publish, GET /consume, optionally POST /ack).
	// TODO: Call http.ListenAndServe(addr, handler).
	_ = addr
	return nil
}

// handlePublish handles POST /publish.
func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	// TODO: Ensure method is POST. Decode JSON body into PublishRequest.
	// TODO: Call s.broker.Publish(r.Context(), req.Topic, req.Body).
	// TODO: Return 201 on success, 400 on bad request, 500 on server error.
	_ = w
	_ = r
	return
}

// handleConsume handles GET /consume.
func (s *Server) handleConsume(w http.ResponseWriter, r *http.Request) {
	// TODO: Parse query (e.g. ?topic=foo). Call s.broker.Consume(r.Context(), topic).
	// TODO: If message returned, encode as ConsumeResponse and write 200.
	// TODO: If no message, return 204 or 404 as per spec.
	_ = w
	_ = r
	return
}

// handleAck handles POST /ack (optional; for client acknowledgement).
func (s *Server) handleAck(w http.ResponseWriter, r *http.Request) {
	// TODO: Decode JSON body (e.g. {"id": "message-id"}). Call s.broker.Ack(r.Context(), id).
	// TODO: Return 200 on success, 400/404/500 as appropriate.
	_ = w
	_ = r
	return
}
