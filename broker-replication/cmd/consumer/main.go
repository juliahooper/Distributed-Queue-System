package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/distributed-queue-system/broker-replication/client"
)

func main() {
	brokerURL := getEnv("BROKER_URL", "http://localhost:8080")
	topic := getEnv("CONSUMER_TOPIC", "triage")
	consumerID := getEnv("CONSUMER_ID", "consumer")

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	consumer := client.NewHTTPConsumer(brokerURL, httpClient)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("[%s] consumer started: broker=%s topic=%s", consumerID, brokerURL, topic)

	for {
		msg, err := consumer.Consume(ctx, topic)
		if err != nil {
			if err == client.ErrNoMessageAvailable || isTimeoutError(err) {
				time.Sleep(500 * time.Millisecond)
				continue
			}

			log.Printf("[%s] consume error: %v", consumerID, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		log.Printf("[%s] received message id=%s topic=%s body=%s",
			consumerID,
			msg.ID,
			msg.Topic,
			string(msg.Body),
		)

		time.Sleep(500 * time.Millisecond)

		if err := consumer.Ack(ctx, msg.ID); err != nil {
			log.Printf("[%s] ack failed: %v", consumerID, err)
			continue
		}

		log.Printf("[%s] acked message id=%s", consumerID, msg.ID)
	}
}

func getEnv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return strings.Contains(err.Error(), "context deadline exceeded") ||
		strings.Contains(err.Error(), "Client.Timeout exceeded")
}
