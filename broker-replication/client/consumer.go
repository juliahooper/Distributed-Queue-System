package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrNoMessageAvailable = errors.New("no message available")

type ConsumedMessage struct {
	ID    string `json:"id"`
	Topic string `json:"topic"`
	Body  []byte `json:"body"`
}

type HTTPConsumer struct {
	baseURL    string
	client     *http.Client
	consumerID string
}

func NewHTTPConsumer(baseURL string, httpClient *http.Client) *HTTPConsumer {
	baseURL = strings.TrimRight(baseURL, "/")
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	return &HTTPConsumer{
		baseURL: baseURL,
		client:  httpClient,
	}
}

// WithConsumerID sets the X-Consumer-Id header for audit.
func (c *HTTPConsumer) WithConsumerID(id string) *HTTPConsumer {
	c.consumerID = id
	return c
}

func (c *HTTPConsumer) Consume(ctx context.Context, topic string) (*ConsumedMessage, error) {
	if strings.TrimSpace(topic) == "" {
		return nil, ErrInvalidTopic
	}

	u := c.baseURL + "/consume?topic=" + url.QueryEscape(topic)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build consume request: %w", err)
	}
	if c.consumerID != "" {
		req.Header.Set("X-Consumer-Id", c.consumerID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("consume request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var msg ConsumedMessage
		if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
			return nil, fmt.Errorf("decode consume response: %w", err)
		}
		return &msg, nil

	case http.StatusNoContent, http.StatusNotFound:
		return nil, ErrNoMessageAvailable

	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("consume failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func (c *HTTPConsumer) Ack(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("message id required")
	}

	payload, err := json.Marshal(map[string]string{
		"id": id,
	})
	if err != nil {
		return fmt.Errorf("marshal ack body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/ack",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("build ack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.consumerID != "" {
		req.Header.Set("X-Consumer-Id", c.consumerID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("ack request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ack failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}
