// Package postgres provides PostgreSQL-backed broker storage with row-level locking,
// dead letter table, activity log, and persistent counters.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/distributed-queue-system/broker-replication/internal/queue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	visibilityTimeout = 30 * time.Second
	maxRetries        = 3
)

// Store implements PostgreSQL-backed broker storage with DLQ, activity log, and metrics.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new PostgreSQL store. Caller must run migrations first.
func NewStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close closes the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// RunMigrations runs the init migration.
func (s *Store) RunMigrations(ctx context.Context) error {
	// Read migration from embed or file - for simplicity we inline the key statements
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS messages (
			id SERIAL PRIMARY KEY,
			message_id VARCHAR(64) UNIQUE NOT NULL,
			topic VARCHAR(255) NOT NULL,
			body BYTEA NOT NULL,
			producer_id VARCHAR(255),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			status VARCHAR(20) DEFAULT 'available',
			consumer_id VARCHAR(255),
			expires_at TIMESTAMPTZ,
			retry_count INT DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_messages_topic_status ON messages(topic, status);
		CREATE INDEX IF NOT EXISTS idx_messages_expires ON messages(expires_at) WHERE status = 'pending';

		CREATE TABLE IF NOT EXISTS dead_letter (
			id SERIAL PRIMARY KEY,
			message_id VARCHAR(64) NOT NULL,
			topic VARCHAR(255) NOT NULL,
			body BYTEA NOT NULL,
			producer_id VARCHAR(255),
			failed_at TIMESTAMPTZ DEFAULT NOW(),
			reason VARCHAR(255) DEFAULT 'max_retries',
			retry_count INT
		);
		CREATE INDEX IF NOT EXISTS idx_dead_letter_topic ON dead_letter(topic);

		CREATE TABLE IF NOT EXISTS activity_log (
			id SERIAL PRIMARY KEY,
			event_type VARCHAR(50) NOT NULL,
			message_id VARCHAR(64),
			topic VARCHAR(255),
			producer_id VARCHAR(255),
			consumer_id VARCHAR(255),
			created_at TIMESTAMPTZ DEFAULT NOW(),
			details JSONB
		);
		CREATE INDEX IF NOT EXISTS idx_activity_log_created ON activity_log(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_activity_log_event ON activity_log(event_type);

		CREATE TABLE IF NOT EXISTS metrics (
			name VARCHAR(100) PRIMARY KEY,
			value BIGINT NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ DEFAULT NOW()
		);
		INSERT INTO metrics (name, value) VALUES ('messages_produced_total', 0), ('messages_consumed_total', 0)
		ON CONFLICT (name) DO NOTHING;
	`)
	return err
}

// Publish adds a message to the queue. message_id is a random UUID.
func (s *Store) Publish(ctx context.Context, topic string, body []byte, producerID string) (messageID string, err error) {
	messageID = uuid.New().String()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO messages (message_id, topic, body, producer_id, status)
		VALUES ($1, $2, $3, $4, 'available')
	`, messageID, topic, body, producerID)
	if err != nil {
		return "", err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE metrics SET value = value + 1, updated_at = NOW() WHERE name = 'messages_produced_total'`)
	_ = s.logActivity(ctx, "publish", messageID, topic, producerID, "", nil)
	return messageID, nil
}

// Consume claims one message for the topic using SELECT FOR UPDATE SKIP LOCKED.
// Only one consumer gets each message (row-level locking).
func (s *Store) Consume(ctx context.Context, topic string, consumerID string) (*queue.Message, error) {
	var msg queue.Message
	var body []byte
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	err = func() error {
		var producerID *string
		row := tx.QueryRow(ctx, `
			SELECT message_id, topic, body, producer_id, retry_count
			FROM messages
			WHERE topic = $1 AND status = 'available'
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		`, topic)
		if err := row.Scan(&msg.ID, &msg.Topic, &body, &producerID, &msg.RetryCount); err != nil {
			if err == pgx.ErrNoRows {
				return errNoMessage
			}
			return err
		}
		msg.Body = append([]byte(nil), body...)
		if producerID != nil {
			msg.ProducerID = *producerID
		}
		expires := time.Now().Add(visibilityTimeout)
		_, err := tx.Exec(ctx, `
			UPDATE messages
			SET status = 'pending', consumer_id = $1, expires_at = $2
			WHERE message_id = $3
		`, consumerID, expires, msg.ID)
		if err != nil {
			return err
		}
		prodID := ""
		if producerID != nil {
			prodID = *producerID
		}
		_ = s.logActivityTx(ctx, tx, "consume", msg.ID, topic, prodID, consumerID, nil)
		return nil
	}()
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if err != nil {
		if err == errNoMessage {
			return nil, err
		}
		return nil, err
	}
	return &msg, nil
}

var errNoMessage = fmt.Errorf("no message available")

// ErrNoMessageAvailable is returned when no message is available.
var ErrNoMessageAvailable = errNoMessage

// Ack acknowledges a message by ID, removing it from pending.
func (s *Store) Ack(ctx context.Context, messageID string, consumerID string) error {
	result, err := s.pool.Exec(ctx, `
		DELETE FROM messages WHERE message_id = $1 AND status = 'pending'
	`, messageID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("message %s not found or already acked", messageID)
	}
	_, _ = s.pool.Exec(ctx, `UPDATE metrics SET value = value + 1, updated_at = NOW() WHERE name = 'messages_consumed_total'`)
	_ = s.logActivity(ctx, "ack", messageID, "", "", consumerID, nil)
	return nil
}

// Peek returns up to n messages for the topic without consuming.
func (s *Store) Peek(ctx context.Context, topic string, n int) ([]*queue.Message, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT message_id, topic, body, producer_id, retry_count
		FROM messages
		WHERE topic = $1 AND status = 'available'
		ORDER BY created_at ASC
		LIMIT $2
	`, topic, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*queue.Message
	for rows.Next() {
		var m queue.Message
		var body []byte
		var producerID *string
		if err := rows.Scan(&m.ID, &m.Topic, &body, &producerID, &m.RetryCount); err != nil {
			return nil, err
		}
		m.Body = append([]byte(nil), body...)
		if producerID != nil {
			m.ProducerID = *producerID
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// RequeueExpired moves expired pending messages back to available or to dead_letter.
func (s *Store) RequeueExpired(ctx context.Context) error {
	// Find expired pending
	rows, err := s.pool.Query(ctx, `
		SELECT message_id, topic, body, producer_id, retry_count
		FROM messages
		WHERE status = 'pending' AND expires_at < NOW()
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var toRequeue, toDLQ []messageRow
	for rows.Next() {
		var r messageRow
		if err := rows.Scan(&r.messageID, &r.topic, &r.body, &r.producerID, &r.retryCount); err != nil {
			return err
		}
		if r.retryCount >= maxRetries {
			toDLQ = append(toDLQ, r)
		} else {
			toRequeue = append(toRequeue, r)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range toRequeue {
		if err := s.requeueOne(ctx, r); err != nil {
			log.Printf("[postgres] requeue error: %v", err)
		}
	}
	for _, r := range toDLQ {
		if err := s.moveToDLQ(ctx, r); err != nil {
			log.Printf("[postgres] moveToDLQ error: %v", err)
		}
	}
	return nil
}

type messageRow struct {
	messageID  string
	topic      string
	body       []byte
	producerID *string
	retryCount int
}

func (s *Store) requeueOne(ctx context.Context, r messageRow) error {
	producerID := ""
	if r.producerID != nil {
		producerID = *r.producerID
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE messages
		SET status = 'available', consumer_id = NULL, expires_at = NULL, retry_count = retry_count + 1
		WHERE message_id = $1
	`, r.messageID)
	if err != nil {
		return err
	}
	_ = s.logActivity(ctx, "requeue", r.messageID, r.topic, producerID, "", map[string]interface{}{"retry_count": r.retryCount + 1})
	return nil
}

func (s *Store) moveToDLQ(ctx context.Context, r messageRow) error {
	producerID := ""
	if r.producerID != nil {
		producerID = *r.producerID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO dead_letter (message_id, topic, body, producer_id, reason, retry_count)
		VALUES ($1, $2, $3, $4, 'max_retries', $5)
	`, r.messageID, r.topic, r.body, producerID, r.retryCount)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM messages WHERE message_id = $1`, r.messageID)
	if err != nil {
		return err
	}
	_ = s.logActivity(ctx, "dlq", r.messageID, r.topic, producerID, "", map[string]interface{}{"reason": "max_retries"})
	return nil
}

// DeadLetterCount returns the count of messages in the dead letter table.
func (s *Store) DeadLetterCount(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM dead_letter`).Scan(&n)
	return n, err
}

// DeadLetterEntry is a single entry in the dead letter table.
type DeadLetterEntry struct {
	MessageID  string    `json:"message_id"`
	Topic      string    `json:"topic"`
	Body       []byte    `json:"body"`
	ProducerID string    `json:"producer_id"`
	FailedAt   time.Time `json:"failed_at"`
	Reason     string    `json:"reason"`
	RetryCount int       `json:"retry_count"`
}

// ListDeadLetter returns all messages in the dead letter table.
func (s *Store) ListDeadLetter(ctx context.Context) ([]DeadLetterEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT message_id, topic, body, COALESCE(producer_id, ''), failed_at, reason, retry_count
		FROM dead_letter
		ORDER BY failed_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeadLetterEntry
	for rows.Next() {
		var e DeadLetterEntry
		if err := rows.Scan(&e.MessageID, &e.Topic, &e.Body, &e.ProducerID, &e.FailedAt, &e.Reason, &e.RetryCount); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteDeadLetter removes a single message from the dead letter table by message_id.
func (s *Store) DeleteDeadLetter(ctx context.Context, messageID string) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM dead_letter WHERE message_id = $1`, messageID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("message %s not found in dead letter queue", messageID)
	}
	_ = s.logActivity(ctx, "dlq_deleted", messageID, "", "", "", map[string]interface{}{"reason": "manually_discarded"})
	return nil
}

// DeadLetterRetry re-publishes all DLQ messages to the queue and removes them from dead_letter.
func (s *Store) DeadLetterRetry(ctx context.Context) (retried, failed int, err error) {
	rows, err := s.pool.Query(ctx, `
		SELECT message_id, topic, body, producer_id, retry_count
		FROM dead_letter
		ORDER BY id ASC
	`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var items []messageRow
	for rows.Next() {
		var r messageRow
		if err := rows.Scan(&r.messageID, &r.topic, &r.body, &r.producerID, &r.retryCount); err != nil {
			return 0, 0, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	for _, r := range items {
		producerID := ""
		if r.producerID != nil {
			producerID = *r.producerID
		}
		_, err := s.pool.Exec(ctx, `
			INSERT INTO messages (message_id, topic, body, producer_id, status, retry_count)
			VALUES ($1, $2, $3, $4, 'available', 0)
		`, r.messageID, r.topic, r.body, producerID)
		if err != nil {
			failed++
			continue
		}
		_, _ = s.pool.Exec(ctx, `DELETE FROM dead_letter WHERE message_id = $1`, r.messageID)
		retried++
		_ = s.logActivity(ctx, "dlq_retry", r.messageID, r.topic, producerID, "", nil)
	}
	return retried, failed, nil
}

// GetMetrics returns queue metrics.
func (s *Store) GetMetrics(ctx context.Context) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	var produced, consumed int64
	_ = s.pool.QueryRow(ctx, `SELECT value FROM metrics WHERE name = 'messages_produced_total'`).Scan(&produced)
	_ = s.pool.QueryRow(ctx, `SELECT value FROM metrics WHERE name = 'messages_consumed_total'`).Scan(&consumed)
	metrics["messages_produced_total"] = produced
	metrics["messages_consumed_total"] = consumed

	var dlqCount int
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM dead_letter`).Scan(&dlqCount)
	metrics["dead_letter_count"] = dlqCount

	var queueDepth int
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE status = 'available' AND topic = 'er-queue'`).Scan(&queueDepth)
	metrics["queue_depth"] = queueDepth

	var pendingCount int
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE status = 'pending'`).Scan(&pendingCount)
	metrics["pending_count"] = pendingCount

	return metrics, nil
}

// GetActivityLog returns the activity log with pagination.
func (s *Store) GetActivityLog(ctx context.Context, limit, offset int, eventType string) ([]ActivityEntry, error) {
	query := `
		SELECT event_type, message_id, topic, producer_id, consumer_id, created_at, details
		FROM activity_log
	`
	args := []interface{}{}
	argNum := 1
	if eventType != "" {
		query += fmt.Sprintf(" WHERE event_type = $%d", argNum)
		args = append(args, eventType)
		argNum++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argNum, argNum+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ActivityEntry
	for rows.Next() {
		var e ActivityEntry
		var details []byte
		if err := rows.Scan(&e.EventType, &e.MessageID, &e.Topic, &e.ProducerID, &e.ConsumerID, &e.CreatedAt, &details); err != nil {
			return nil, err
		}
		if len(details) > 0 {
			_ = json.Unmarshal(details, &e.Details)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ActivityEntry is a single activity log entry.
type ActivityEntry struct {
	EventType  string                 `json:"event_type"`
	MessageID  string                 `json:"message_id"`
	Topic      string                 `json:"topic"`
	ProducerID string                 `json:"producer_id"`
	ConsumerID string                 `json:"consumer_id"`
	CreatedAt  time.Time              `json:"created_at"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

func (s *Store) logActivity(ctx context.Context, eventType, messageID, topic, producerID, consumerID string, details map[string]interface{}) error {
	return s.logActivityTx(ctx, nil, eventType, messageID, topic, producerID, consumerID, details)
}

func (s *Store) logActivityTx(ctx context.Context, tx pgx.Tx, eventType, messageID, topic, producerID, consumerID string, details map[string]interface{}) error {
	var detailsJSON []byte
	if details != nil {
		detailsJSON, _ = json.Marshal(details)
	}
	args := []interface{}{eventType, messageID, topic, nullIfEmpty(producerID), nullIfEmpty(consumerID), detailsJSON}
	if tx != nil {
		_, err := tx.Exec(ctx, `
			INSERT INTO activity_log (event_type, message_id, topic, producer_id, consumer_id, details)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, args...)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO activity_log (event_type, message_id, topic, producer_id, consumer_id, details)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, args...)
	return err
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
