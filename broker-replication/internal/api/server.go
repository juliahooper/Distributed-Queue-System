// ASSIGNED TO: ENGINEER A
//
// Package api defines the HTTP/REST entry points for the broker: POST /publish,
// GET /consume. Responsibility: decode JSON, call the Service layer, return HTTP status codes.

package api

import (
	"encoding/json"
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

// AckRequest is the JSON body for POST /ack.
type AckRequest struct {
	ID string `json:"id"`
}

// Server is the HTTP server that exposes the broker API.
// ASSIGNED TO: ENGINEER A
type Server struct {
	broker service.Broker
}

// NewServer creates an API server that uses the given broker service.
func NewServer(broker service.Broker) *Server {
	return &Server{broker: broker}
}

// Register adds broker routes to the given mux (used for testing).
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("/publish", cors(s.handlePublish))
	mux.HandleFunc("/consume", cors(s.handleConsume))
	mux.HandleFunc("/ack", cors(s.handleAck))
}

// cors wraps a handler to add CORS headers for browser clients.
func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// Run starts the HTTP server and blocks until it exits.
func (s *Server) Run(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/publish", cors(s.handlePublish))
	mux.HandleFunc("/consume", cors(s.handleConsume))
	mux.HandleFunc("/ack", cors(s.handleAck))
	return http.ListenAndServe(addr, mux)
}

// handlePublish handles POST /publish.
func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Topic == "" {
		http.Error(w, "bad request: topic required", http.StatusBadRequest)
		return
	}

	if err := s.broker.Publish(r.Context(), req.Topic, req.Body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleConsume handles GET /consume.
func (s *Server) handleConsume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topic := r.URL.Query().Get("topic")
	if topic == "" {
		http.Error(w, "bad request: topic query required", http.StatusBadRequest)
		return
	}

	msg, err := s.broker.Consume(r.Context(), topic)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if msg == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ConsumeResponse{
		ID:    msg.ID,
		Topic: msg.Topic,
		Body:  msg.Body,
	})
}

// handleAck handles POST /ack (optional; for client acknowledgement).
func (s *Server) handleAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "bad request: id required", http.StatusBadRequest)
		return
	}

	if err := s.broker.Ack(r.Context(), req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}
