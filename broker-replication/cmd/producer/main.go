package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	brokerURL := getEnv("BROKER_URL", "http://localhost:8080")
	topic := getEnv("PRODUCER_TOPIC", "er-queue")
	producerID := getEnv("PRODUCER_ID", "producer")
	count := getEnvInt("COUNT", 0) // 0 = loop forever

	httpClient := &http.Client{Timeout: 10 * time.Second}

	log.Printf("[%s] producer started: broker=%s topic=%s", producerID, brokerURL, topic)

	i := 0
	for count == 0 || i < count {
		urgency := rand.Intn(5) + 1
		patientID := fmt.Sprintf("P-%s-%03d", strings.ToUpper(producerID[:min(4, len(producerID))]), i+1)

		body, _ := json.Marshal(map[string]interface{}{
			"patientId": patientID,
			"urgency":   urgency,
		})
		encoded := base64.StdEncoding.EncodeToString(body)

		payload, _ := json.Marshal(map[string]string{
			"topic": topic,
			"body":  encoded,
		})

		req, _ := http.NewRequest(http.MethodPost, brokerURL+"/publish", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Producer-Id", producerID)

		resp, err := httpClient.Do(req)
		if err != nil {
			log.Printf("[%s] publish error: %v", producerID, err)
		} else {
			resp.Body.Close()
			log.Printf("[%s] published patient=%s urgency=%d", producerID, patientID, urgency)
		}

		i++
		time.Sleep(2 * time.Second)
	}
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
