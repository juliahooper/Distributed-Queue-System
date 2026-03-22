# Distributed Queue System - Implementation Plan

> Editable plan for team collaboration. Last updated for spec-aligned redo.

## Overview

Strip Azure queue/storage services, implement PostgreSQL-backed queue, add console producer/consumer apps, Azure Entra ID auth, dead letter queue (retry 3x then dead_letter table), monitoring dashboard, and deploy on a single Azure Linux VM.

---

## Goals (from spec)

- **Scalability**: Handle many messages efficiently
- **Reliability**: Messages survive failures (PostgreSQL persistence)
- **Ordering**: FIFO per topic
- **Decoupling**: Producers and consumers independent
- **Multiple producers/consumers**: Console apps + frontend
- **Pull notifications**: Consumers poll the broker
- **At-most-once delivery**: One consumer reads each message
- **No Azure queue services**: Only an Azure Linux VM for hosting

---

## Architecture

```mermaid
flowchart TB
    subgraph vm [Azure Linux VM]
        Broker[Broker HTTP API :8080]
        Pg[(PostgreSQL)]
        DLQ[(dead_letter table)]
        Static[Static Frontend]
    end

    subgraph producers [Producers]
        P1[Console Producer 1]
        P2[Console Producer 2]
        P3[Frontend Receptionist 1]
        P4[Frontend Receptionist 2]
    end

    subgraph consumers [Consumers]
        C1[Console Consumer 1]
        C2[Console Consumer 2]
        C3[Frontend Nurse 1]
        C4[Frontend Nurse 2]
    end

    P1 -->|POST /publish| Broker
    P2 -->|POST /publish| Broker
    P3 -->|POST /publish| Broker
    P4 -->|POST /publish| Broker

    C1 -->|GET /consume POST /ack| Broker
    C2 -->|GET /consume POST /ack| Broker
    C3 -->|GET /peek consume ack| Broker
    C4 -->|GET /peek consume ack| Broker

    Broker --> Pg
    Broker -->|after 3 retries ~30s visibility each| DLQ
    Static -->|served by nginx/broker| Users
```

**Components:**

| Component             | Role                                                                                          |
| --------------------- | --------------------------------------------------------------------------------------------- |
| **Broker**            | HTTP API (publish, peek, consume, ack, dead-letter, metrics). Uses PostgreSQL.                |
| **PostgreSQL**        | Queue storage. Our own schema.                                                                |
| **Dead letter table** | PostgreSQL `dead_letter` table. Messages after 3 failed deliveries. Retry via POST /dead-letter/retry. |
| **Console Producers** | CLI apps that POST to broker. Run multiple terminals to demo multiple producers.              |
| **Console Consumers** | CLI apps that poll broker (consume + ack). Run multiple terminals to demo multiple consumers. |
| **Frontend**          | Web UI: receptionists, nurses, admin (monitor). Azure Entra ID auth. Served from VM.          |
| **Azure VM**          | Hosts broker, PostgreSQL, and static frontend. No Service Bus, no Blob.                       |

---

## 1. Remove Azure Dependencies

**Delete or isolate:**

- `broker-replication/internal/azure/` — Service Bus, Blob Storage
- `broker-replication/cmd/producer_azure/`
- Azure SDK deps from `go.mod`

**Broker:** Use only PostgreSQL (or in-memory for local dev). No Service Bus, no Blob WAL.

---

## 2. PostgreSQL Queue

**Schema** (`internal/queue/postgres.go` + migration):

```sql
CREATE TABLE messages (
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
CREATE INDEX idx_messages_topic_status ON messages(topic, status);
```

**Message ID:** Generate `message_id` as a random GUID (UUID v4) on publish. Ensures uniqueness across producers and restarts.

**Dequeue locking:** Use row-level locking (e.g., `SELECT ... FOR UPDATE SKIP LOCKED`) so only one consumer gets each message. Ensures at-most-once delivery; no two consumers receive the same message.

**Queue interface:** `Enqueue`, `Dequeue`, `PeekN` backed by PostgreSQL. Visibility timeout (~30s) and requeue for unacked messages. `Message` struct includes `RetryCount`; broker increments on requeue and moves to DLQ when `RetryCount >= 3`.

---

## 3. Console Producer

**File:** `broker-replication/cmd/producer/main.go`

- CLI that POSTs to broker `/publish`
- Env: `BROKER_URL`, `PRODUCER_ID`, `TOPIC` (default `er-queue`)
- Loop: prompt or auto-generate patient entries, POST, log
- Run multiple instances (different `PRODUCER_ID`) to demo multiple producers

**Example:**

```bash
PRODUCER_ID=receptionist-1 BROKER_URL=http://vm-ip:8080 go run cmd/producer/main.go
```

---

## 4. Console Consumer

**Existing:** `broker-replication/cmd/consumer/main.go` already polls broker.

**Changes:**

- Default topic: `er-queue` (match frontend)
- Ensure `CONSUMER_ID` is passed to broker (header or query) for audit
- Run multiple instances to demo multiple consumers

**Failure simulation (for demo):** Add CLI flags to intentionally skip acknowledgments and demonstrate DLQ flow:
- `--skip-ack` — never send ack for consumed messages (simulates crash/failure)
- `--fail-rate 0.3` — randomly skip ack with given probability (e.g., 30% of messages)
- Use when demoing: run consumer with `--skip-ack` to show messages requeue and eventually move to DLQ after 3 retries

---

## 5. Broker Updates

**File:** `broker-replication/cmd/broker/main.go`

- Remove Azure Service Bus and Blob branches
- Use PostgreSQL when `POSTGRES_DSN` is set; otherwise in-memory for local dev
- Listen on `0.0.0.0:8080` on VM so external clients can connect
- Optionally serve static frontend from `/` (or rely on nginx)

---

## 6. Frontend: Multiple Producers/Consumers

**Login + roles:**

- Login: name + role (receptionist | nurse | admin)
- Receptionists: Register patient (producer)
- Nurses: View queue, Call next (consumer)
- Admin/IT: Access monitoring dashboard (see Section 6c)

**Producer/consumer IDs:**

- API sends `X-Producer-Id` / `X-Consumer-Id` with requests
- Broker stores in `messages` for audit
- UI shows "Added by Receptionist X", "Called by Nurse Y"

**Off localhost:**

- Build: `npm run build`
- Serve from VM (nginx or broker static handler)
- `VITE_API_URL` points to broker (e.g. `http://vm-ip:8080` or relative `/api`)

---

## 6a. Authentication (Azure Entra ID)

**Approach:** Microsoft Entra ID (formerly Azure AD) for sign-in. Frontend uses MSAL.js.

**Setup:**

- Azure Portal: App registration (single-page app, redirect URIs)
- Frontend: `@azure/msal-browser` for login, acquire token
- API calls: attach Bearer token; broker validates (optional for MVP) or frontend just uses token for role/identity

**Flow:**

- User visits app → redirect to Microsoft login if not authenticated
- After login, get user profile (name, email) and map to role (receptionist/nurse/admin) via app role or simple config
- Token stored in MSAL cache; sent with API requests

**Files:**

- `frontend/src/auth/` — MSAL config, login/logout, useAuth hook
- `frontend/src/App.jsx` — wrap with MsalProvider, protect routes

---

## 6b. Dead Letter Queue

**Flow:** When a consumed message is not acked before visibility timeout (~30s), broker requeues it. Track `retry_count` per message. After 3 failed delivery attempts, move to dead letter instead of requeueing. *The queue is not stalled—other messages keep processing.*

**Retry count conflation:** A slow consumer can cause multiple requeues without actual processing failures. `retry_count` may conflate "slow" vs "failed" scenarios. Consider distinguishing slow-consumer requeues from true failures (e.g., separate counters or semantics) if needed.

**Schema change:** Add `retry_count INT DEFAULT 0` to `messages`. On requeue, increment. If `retry_count >= 3`, dead-letter instead.

**Dead letter storage:** PostgreSQL `dead_letter` table (not JSONL file). Benefits: querying, indexing, transactions, durability.

```sql
CREATE TABLE dead_letter (
    id SERIAL PRIMARY KEY,
    message_id VARCHAR(64) NOT NULL,
    topic VARCHAR(255) NOT NULL,
    body BYTEA NOT NULL,
    producer_id VARCHAR(255),
    failed_at TIMESTAMPTZ DEFAULT NOW(),
    reason VARCHAR(255) DEFAULT 'max_retries',
    retry_count INT
);
CREATE INDEX idx_dead_letter_topic ON dead_letter(topic);
```

**Broker logic:**

- `requeueExpiredLoop`: for expired pending, if `retry_count < 3` → requeue (increment retry_count); else → insert into `dead_letter` table, remove from pending.

**Broker endpoints:**

- `GET /dead-letter/count` — returns `{ count: N }` (query `dead_letter` table)
- `POST /dead-letter/retry` — reads from `dead_letter` table, re-publishes each message to queue, deletes from table on success; returns `{ retried: N, failed: M }`

**Frontend:**

- Poll or fetch `GET /dead-letter/count` periodically (or include in metrics)
- If `count > 3`: show alert banner (e.g. "Dead letter queue has N items. [Retry] [Dismiss]")
- "Retry" button calls `POST /dead-letter/retry`, then refreshes

---

## 6c. Monitoring Dashboard (3rd Page)

**Audience:** Admin/IT role. Route: `/monitor`.

**Metrics (broker endpoints):**

- `GET /metrics` — returns JSON, e.g.:
  - `queue_depth` — count of available messages per topic
  - `pending_count` — messages awaiting ack
  - `messages_produced_total` — counter (increment on publish); **persisted in DB** so it survives broker restarts
  - `messages_consumed_total` — counter (increment on ack); **persisted in DB**
  - `dead_letter_count` — from `dead_letter` table

**Persistent counters:** Store `messages_produced_total` and `messages_consumed_total` in PostgreSQL (e.g., `metrics` table or derived from `activity_log`). Counters must survive broker restarts.

**Activity log:** Add `activity_log` table; broker logs every publish, consume, ack, requeue, and DLQ event. Display in admin dashboard (filterable, paginated).

```sql
CREATE TABLE activity_log (
    id SERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    message_id VARCHAR(64),
    topic VARCHAR(255),
    producer_id VARCHAR(255),
    consumer_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    details JSONB
);
CREATE INDEX idx_activity_log_created ON activity_log(created_at DESC);
CREATE INDEX idx_activity_log_event ON activity_log(event_type);
```

**Frontend:** Simple dashboard with cards/gauges:

- Queue depth (er-queue)
- Pending count
- Produced / consumed totals (or rate if we track timestamps)
- Dead letter count with link to alert/retry
- **Activity log view** — filterable, paginated list of publish/consume/ack/requeue/DLQ events

Poll every 5–10 seconds. Basic styling consistent with rest of app.

---

## 7. Azure VM Deployment

**Stack on VM:**

- PostgreSQL (install via apt)
- Broker binary (built from repo)
- Frontend static files (from `frontend/dist`)

**Options:**

- **A)** Nginx: reverse proxy for broker (`/api` → `:8080`), serve frontend from `/`
- **B)** Broker serves static: mount `frontend/dist` at `/`, API at `/publish`, `/consume`, etc.

**Env on VM:**

```
POSTGRES_DSN=postgres://user:pass@localhost:5432/queue?sslmode=disable
BROKER_ADDR=0.0.0.0:8080
```

**Azure Entra ID:** App registration client ID and tenant for frontend auth.

**Demo:**

- Open frontend at `http://vm-ip/` (or port 80)
- Run 2–3 producer consoles, 2–3 consumer consoles from dev machines pointing at `http://vm-ip:8080`
- Show multiple receptionists adding patients, multiple nurses calling next
- **Failure simulation:** Run a consumer with `--skip-ack` or `--fail-rate 0.3` to intentionally skip acknowledgments; show messages requeue and move to DLQ after 3 retries

---

## 8. File Summary

| Action   | File |
| -------- | ---- |
| Create   | `internal/queue/postgres.go` — PostgreSQL queue impl |
| Create   | `internal/storage/postgres/migrations/001_init.sql` — messages, dead_letter, activity_log, metrics |
| Create   | `cmd/producer/main.go` — HTTP producer console |
| Modify   | `cmd/broker/main.go` — Remove Azure, add Postgres, bind 0.0.0.0 |
| Modify   | `cmd/consumer/main.go` — Default topic er-queue, consumer ID, `--skip-ack` / `--fail-rate` for failure simulation |
| Modify   | `internal/api/server.go` — X-Producer-Id, X-Consumer-Id |
| Modify   | `internal/queue/memory.go` — Add RetryCount to Message |
| Modify   | `internal/service/broker.go` — DLQ logic (retry 3x, then dead_letter table), GET /dead-letter/count, POST /dead-letter/retry, activity log, persistent counters |
| Create   | `internal/metrics/` or broker — GET /metrics for monitoring |
| Modify   | `frontend/src/` — Login (MSAL), role-based views, producer/consumer IDs |
| Create   | `frontend/src/auth/` — MSAL config, useAuth |
| Create   | `frontend/src/views/Monitor.jsx` — Monitoring dashboard, activity log view |
| Create   | `frontend/src/views/DeadLetterAlert.jsx` — Alert banner when DLQ > 3 |
| Modify   | `frontend/vite.config.js` — API proxy, build config |
| Delete   | `internal/azure/*`, `cmd/producer_azure` |
| Update   | `.env.example` — POSTGRES_DSN, Azure Entra app config |

---

## 9. Spec Checklist

**In scope:**

- Multiple producers and consumers (console + frontend)
- In-order queue operations (FIFO via PostgreSQL)
- Producers produce messages (POST /publish)
- Consumers read messages (GET /consume, POST /ack)
- Pull notifications (consumers poll)
- At-most-once delivery (one consumer per message)
- Frontend and backend
- Message persistence (PostgreSQL)
- No Azure queue services (only VM)
- Database from our own programs (PostgreSQL schema)
- Auth (Azure Entra ID), dead letter queue, monitoring dashboard

**Out of scope for now:** Load simulation and benchmarking.

---

## 10. Implementation Notes & Open Items

| Item | Current State | Action |
|------|---------------|--------|
| **Dequeue locking** | No row-level locking specified; only one consumer should get each message | Design and document locking (e.g., `SELECT ... FOR UPDATE SKIP LOCKED`); ensure single-consumer semantics |
| **DLQ storage** | JSONL file | Migrate to PostgreSQL `dead_letter` table for querying, indexing, transactions |
| **Retry count conflation** | `retry_count` increments on any requeue | Consider slow consumer vs true failure; may need separate semantics or counters |
| **Activity log** | None | Add `activity_log` table; broker logs publish/consume/ack/requeue/DLQ; display in admin dashboard |
| **Counters** | In-memory | Persist `messages_produced_total`, `messages_consumed_total` in DB |
| **Failure simulation** | Console consumer cannot skip ack | Add `--skip-ack` or `--fail-rate` to demo intentional failures |
| **Message ID** | Not specified | Use random GUID (UUID v4) for `message_id`; document in schema |
