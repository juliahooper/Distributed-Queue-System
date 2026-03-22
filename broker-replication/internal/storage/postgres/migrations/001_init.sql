-- Distributed Queue System - Initial Schema
-- Run this migration to create all tables.

-- Main queue messages (available, pending, completed)
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

-- Dead letter queue (messages that failed after max retries)
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

-- Activity log (publish, consume, ack, requeue, dlq)
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

-- Persistent counters for metrics
CREATE TABLE IF NOT EXISTS metrics (
    name VARCHAR(100) PRIMARY KEY,
    value BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
INSERT INTO metrics (name, value) VALUES ('messages_produced_total', 0), ('messages_consumed_total', 0)
ON CONFLICT (name) DO NOTHING;
