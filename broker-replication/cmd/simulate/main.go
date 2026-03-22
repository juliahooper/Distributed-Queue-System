// simulate spins up multiple producer and consumer goroutines to simulate
// realistic ER queue load. Producers publish patients at random intervals;
// consumers process them with configurable failure rates for DLQ demos.
//
// Usage:
//
//	BROKER_URL=http://<vm-ip>:8080 go run ./cmd/simulate
//
// Env vars:
//
//	BROKER_URL        broker address (default http://localhost:8080)
//	PRODUCERS         number of producer goroutines (default 10)
//	CONSUMERS         number of consumer goroutines (default 8)
//	FAILING_CONSUMERS number of consumers that skip acks for DLQ demo (default 2)
//	FAIL_RATE         probability a failing consumer skips ack (default 1.0 = always)
//	TOPIC             queue topic (default er-queue)
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	brokerURL := getEnv("BROKER_URL", "http://localhost:8080")
	topic := getEnv("TOPIC", "er-queue")
	numProducers := getEnvInt("PRODUCERS", 10)
	numConsumers := getEnvInt("CONSUMERS", 8)
	numFailing := getEnvInt("FAILING_CONSUMERS", 2)
	failRate := getEnvFloat("FAIL_RATE", 1.0)

	log.Printf("simulation starting: %d producers, %d consumers (%d failing at rate %.0f%%), broker=%s topic=%s",
		numProducers, numConsumers, numFailing, failRate*100, brokerURL, topic)
	log.Printf("press Ctrl+C to stop")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	httpClient := &http.Client{Timeout: 10 * time.Second}

	var wg sync.WaitGroup

	// Start producers
	for i := 1; i <= numProducers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			producerID := fmt.Sprintf("sim-receptionist-%02d", id)
			runProducer(ctx, httpClient, brokerURL, topic, producerID)
		}(i)
	}

	// Start normal consumers
	for i := 1; i <= numConsumers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			consumerID := fmt.Sprintf("sim-nurse-%02d", id)
			runConsumer(ctx, httpClient, brokerURL, topic, consumerID, 0)
		}(i)
	}

	// Start failing consumers (for DLQ demo)
	for i := 1; i <= numFailing; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			consumerID := fmt.Sprintf("sim-failing-%02d", id)
			runConsumer(ctx, httpClient, brokerURL, topic, consumerID, failRate)
		}(i)
	}

	wg.Wait()
	log.Println("simulation stopped")
}

func runProducer(ctx context.Context, client *http.Client, brokerURL, topic, producerID string) {
	counter := 0
	for {
		// Wait a realistic interval before publishing (3–12 seconds)
		wait := time.Duration(3000+rand.Intn(9000)) * time.Millisecond
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		counter++
		urgency := rand.Intn(5) + 1
		patientID := fmt.Sprintf("SIM-%s-%03d", strings.ToUpper(producerID[len(producerID)-4:]), counter)

		body, _ := json.Marshal(map[string]interface{}{
			"patientId": patientID,
			"urgency":   urgency,
		})
		encoded := base64.StdEncoding.EncodeToString(body)
		payload, _ := json.Marshal(map[string]string{"topic": topic, "body": encoded})

		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, brokerURL+"/publish", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Producer-Id", producerID)

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[%s] publish error: %v", producerID, err)
			continue
		}
		resp.Body.Close()
		log.Printf("[%s] registered %s (urgency %d)", producerID, patientID, urgency)
	}
}

func runConsumer(ctx context.Context, client *http.Client, brokerURL, topic, consumerID string, failRate float64) {
	if failRate > 0 {
		log.Printf("[%s] FAILING consumer started (fail rate %.0f%%)", consumerID, failRate*100)
	}
	for {
		// Wait before polling (2–6 seconds between consume attempts)
		wait := time.Duration(2000+rand.Intn(4000)) * time.Millisecond
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		msg, err := consume(ctx, client, brokerURL, topic, consumerID)
		if err != nil || msg == nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}

		// Simulate processing time (1–5 seconds)
		processingTime := time.Duration(1000+rand.Intn(4000)) * time.Millisecond
		select {
		case <-ctx.Done():
			return
		case <-time.After(processingTime):
		}

		// Decide whether to ack
		shouldFail := failRate > 0 && rand.Float64() < failRate
		if shouldFail {
			log.Printf("[%s] SIMULATED FAILURE: not acking %s (will requeue → DLQ)", consumerID, msg.ID)
			continue
		}

		if err := ack(ctx, client, brokerURL, consumerID, msg.ID); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[%s] ack error: %v", consumerID, err)
			continue
		}
		log.Printf("[%s] processed %s", consumerID, msg.ID)
	}
}

type consumeMsg struct {
	ID    string `json:"id"`
	Topic string `json:"topic"`
	Body  string `json:"body"`
}

func consume(ctx context.Context, client *http.Client, brokerURL, topic, consumerID string) (*consumeMsg, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/consume?topic=%s", brokerURL, topic), nil)
	req.Header.Set("X-Consumer-Id", consumerID)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var msg consumeMsg
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func ack(ctx context.Context, client *http.Client, brokerURL, consumerID, messageID string) error {
	payload, _ := json.Marshal(map[string]string{"id": messageID})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, brokerURL+"/ack", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Consumer-Id", consumerID)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func getEnv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvFloat(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
