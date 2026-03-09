package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPConsumer_ConsumeSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/consume" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("topic"); got != "triage" {
			t.Fatalf("expected topic=triage, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ConsumedMessage{
			ID:    "msg-1",
			Topic: "triage",
			Body:  []byte("patient-A"),
		})
	}))
	defer ts.Close()

	c := NewHTTPConsumer(ts.URL, ts.Client())

	msg, err := c.Consume(context.Background(), "triage")
	if err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message, got nil")
	}
	if msg.ID != "msg-1" {
		t.Fatalf("expected id msg-1, got %q", msg.ID)
	}
	if msg.Topic != "triage" {
		t.Fatalf("expected topic triage, got %q", msg.Topic)
	}
	if string(msg.Body) != "patient-A" {
		t.Fatalf("expected body patient-A, got %q", string(msg.Body))
	}
}

func TestHTTPConsumer_ConsumeNoMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no message available", http.StatusNotFound)
	}))
	defer ts.Close()

	c := NewHTTPConsumer(ts.URL, ts.Client())

	msg, err := c.Consume(context.Background(), "triage")
	if err != ErrNoMessageAvailable {
		t.Fatalf("expected ErrNoMessageAvailable, got %v", err)
	}
	if msg != nil {
		t.Fatalf("expected nil message, got %+v", msg)
	}
}

func TestHTTPConsumer_AckSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ack" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["id"] != "msg-1" {
			t.Fatalf("expected id msg-1, got %q", body["id"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewHTTPConsumer(ts.URL, ts.Client())

	if err := c.Ack(context.Background(), "msg-1"); err != nil {
		t.Fatalf("Ack returned error: %v", err)
	}
}
