// ASSIGNED TO: ENGINEER A
//
// Package api defines the HTTP/REST entry points for the broker: POST /publish,
// GET /consume. Responsibility: decode JSON, call the Service layer, return HTTP status codes.

package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/distributed-queue-system/broker-replication/internal/queue"
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
	mux.HandleFunc("/peek", cors(s.handlePeek))
	mux.HandleFunc("/ack", cors(s.handleAck))
	if ext, ok := s.broker.(service.BrokerExt); ok {
		mux.HandleFunc("/metrics", cors(s.handleMetrics(ext)))
		mux.HandleFunc("/dead-letter/count", cors(s.handleDeadLetterCount(ext)))
		mux.HandleFunc("/dead-letter/retry", cors(s.handleDeadLetterRetry(ext)))
		mux.HandleFunc("/activity-log", cors(s.handleActivityLog(ext)))
	}
}

// cors wraps a handler to add CORS headers for browser clients.
func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Producer-Id, X-Consumer-Id")
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
	mux.HandleFunc("/peek", cors(s.handlePeek))
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

	producerID := r.Header.Get("X-Producer-Id")
	if audit, ok := s.broker.(service.BrokerWithAudit); ok {
		if err := audit.PublishWithProducer(r.Context(), req.Topic, req.Body, producerID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if err := s.broker.Publish(r.Context(), req.Topic, req.Body); err != nil {
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

	consumerID := r.Header.Get("X-Consumer-Id")
	var msg *queue.Message
	var err error
	if audit, ok := s.broker.(service.BrokerWithAudit); ok {
		msg, err = audit.ConsumeWithConsumer(r.Context(), topic, consumerID)
	} else {
		msg, err = s.broker.Consume(r.Context(), topic)
	}
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

// PeekResponse is the JSON body for GET /peek.
type PeekResponse struct {
	Queue []ConsumeResponse `json:"queue"`
}

// handlePeek handles GET /peek?topic=X&limit=N.
func (s *Server) handlePeek(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	topic := r.URL.Query().Get("topic")
	if topic == "" {
		http.Error(w, "bad request: topic query required", http.StatusBadRequest)
		return
	}
	limit := 5
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 20 {
			limit = n
		}
	}
	msgs := s.broker.Peek(r.Context(), topic, limit)
	resp := PeekResponse{Queue: make([]ConsumeResponse, len(msgs))}
	for i, m := range msgs {
		resp.Queue[i] = ConsumeResponse{ID: m.ID, Topic: m.Topic, Body: m.Body}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
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

	consumerID := r.Header.Get("X-Consumer-Id")
	if audit, ok := s.broker.(service.BrokerWithAudit); ok {
		if err := audit.AckWithConsumer(r.Context(), req.ID, consumerID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
	} else if err := s.broker.Ack(r.Context(), req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleMetrics returns a handler for GET /metrics.
func (s *Server) handleMetrics(ext service.BrokerExt) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		metrics, err := ext.GetMetrics(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(metrics)
	}
}

// handleDeadLetterCount returns a handler for GET /dead-letter/count.
func (s *Server) handleDeadLetterCount(ext service.BrokerExt) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		count, err := ext.DeadLetterCount(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]int{"count": count})
	}
}

// handleDeadLetterRetry returns a handler for POST /dead-letter/retry.
func (s *Server) handleDeadLetterRetry(ext service.BrokerExt) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		retried, failed, err := ext.DeadLetterRetry(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]int{"retried": retried, "failed": failed})
	}
}

// handleActivityLog returns a handler for GET /activity-log?limit=N&offset=N&event_type=X.
func (s *Server) handleActivityLog(ext service.BrokerExt) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limit := 50
		if s := r.URL.Query().Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		offset := 0
		if s := r.URL.Query().Get("offset"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 {
				offset = n
			}
		}
		eventType := r.URL.Query().Get("event_type")
		entries, err := ext.GetActivityLog(r.Context(), limit, offset, eventType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"entries": entries})
	}
}
