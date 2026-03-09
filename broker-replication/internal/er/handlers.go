// Package er provides HTTP handlers for the ER queue API.

package er

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// RegisterRequest is the JSON body for POST /er/register. Patient ID is assigned by the server.
type RegisterRequest struct {
	Urgency int `json:"urgency"`
}

// RegisterResponse is the JSON body returned from POST /er/register.
type RegisterResponse struct {
	ID        string `json:"id"`
	PatientID string `json:"patientId"`
}

// NextResponse is the JSON body for GET /er/next.
type NextResponse struct {
	Queue []Entry `json:"queue"`
}

// CallRequest is the JSON body for POST /er/call.
type CallRequest struct {
	ID string `json:"id"`
}

// Handlers holds the ER queue and provides HTTP handlers.
type Handlers struct {
	Queue *Queue
}

// NewHandlers returns handlers that use the given queue.
func NewHandlers(q *Queue) *Handlers {
	return &Handlers{Queue: q}
}

// Register registers ER routes on the given mux. Prefix is expected to be "/er".
func (h *Handlers) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/register", h.handleRegister)
	mux.HandleFunc(prefix+"/next", h.handleNext)
	mux.HandleFunc(prefix+"/call", h.handleCall)
}

func (h *Handlers) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: invalid JSON", http.StatusBadRequest)
		return
	}
	id, patientID, err := h.Queue.Register(r.Context(), req.Urgency)
	if err != nil {
		if err.Error() == "urgency must be 1-5" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(RegisterResponse{ID: id, PatientID: patientID})
}

func (h *Handlers) handleNext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 6
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}
	queue := h.Queue.PeekNext(limit)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(NextResponse{Queue: queue})
}

func (h *Handlers) handleCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: invalid JSON", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "bad request: id required", http.StatusBadRequest)
		return
	}
	err := h.Queue.Call(r.Context(), req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}
