# Broker (Backend)

Go backend for the Distributed Queue System. Runs the generic message broker and the ER queue API.

---

## Run

```bash
# From broker-replication/
go run ./cmd/broker
```

Listens on `:8080` by default. Set `BROKER_ADDR` to change.

---

## Environment

Copy `.env.example` to `.env` and fill in values. For local dev, no env vars are required.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AZURE_SERVICEBUS_CONNECTION_STRING` | For Azure | — | Service Bus connection string |
| `AZURE_SERVICEBUS_QUEUE_NAME` | — | `broker-queue` | Queue name |
| `AZURE_STORAGE_CONNECTION_STRING` | For WAL | — | Blob Storage connection string |
| `AZURE_STORAGE_CONTAINER_NAME` | — | `broker-logs` | Container name |
| `BROKER_ADDR` | — | `:8080` | HTTP listen address |

---

## Endpoints

### Broker (generic)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/publish` | Enqueue message `{ topic, body }` |
| GET | `/consume?topic=X` | Dequeue one message |
| POST | `/ack` | Acknowledge `{ id }` |

### ER Queue (frontend)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/er/register` | Add patient `{ urgency }` |
| GET | `/er/next?limit=N` | Peek next N patients |
| POST | `/er/call` | Remove patient `{ id }` |

---

## Architecture

- **Broker**: `queue.Queue` → `service.BrokerService` → `api.Server`
- **ER Queue**: `er.Queue` (persisted to `data/er-queue.json`) → `er.Handlers`
- **Azure**: Optional Service Bus for broker queue, Blob Storage for WAL

---

## Packages

| Package | Role |
|---------|------|
| `internal/api` | Broker HTTP handlers (publish, consume, ack) |
| `internal/azure` | Service Bus queue, Blob Storage WAL |
| `internal/er` | ER queue, persistence, HTTP handlers |
| `internal/queue` | Queue interface, MemoryQueue |
| `internal/service` | BrokerService, pending/visibility logic |
| `client/` | Producer client (standalone, not used by broker) |
