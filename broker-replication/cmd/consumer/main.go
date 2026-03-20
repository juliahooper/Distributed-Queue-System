package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"math/rand"
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
	skipAck := flag.Bool("skip-ack", false, "never send ack (simulate crash/failure for DLQ demo)")
	failRate := flag.Float64("fail-rate", 0, "randomly skip ack with this probability (0-1, e.g. 0.3 = 30%%)")
	flag.Parse()

	brokerURL := getEnv("BROKER_URL", "http://localhost:8080")
	topic := getEnv("CONSUMER_TOPIC", "er-queue")
	consumerID := getEnv("CONSUMER_ID", "consumer")

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	consumer := client.NewHTTPConsumer(brokerURL, httpClient).WithConsumerID(consumerID)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if *skipAck {
		log.Printf("[%s] FAILURE SIMULATION: --skip-ack enabled, will never ack (messages will requeue and move to DLQ after 3 retries)", consumerID)
	}
	if *failRate > 0 {
		log.Printf("[%s] FAILURE SIMULATION: --fail-rate=%.2f enabled, will randomly skip ack", consumerID, *failRate)
	}
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

		shouldAck := !*skipAck && (*failRate <= 0 || rand.Float64() >= *failRate)
		if !shouldAck {
			log.Printf("[%s] SIMULATED FAILURE: skipping ack for message id=%s (will requeue)", consumerID, msg.ID)
			continue
		}

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
