# Distributed Queue System

**Pull-based message broker + ER queue frontend**

A message broker with an ER (Emergency Room) queue demo app. Supports local in-memory mode or Azure (Service Bus + Blob Storage).

---

## What's in here

| Component | Description |
|-----------|-------------|
| **Broker** | Generic message queue: `POST /publish`, `GET /consume`, `POST /ack` |
| **ER Queue** | Hospital-style queue with priority (urgency 1–5), peek, and call. Powers the frontend. |
| **Frontend** | React app: register patients, view next 5 in queue, call next |
| **Storage** | ER queue persists to `data/er-queue.json`. Optional Azure Blob WAL for broker. |

---

## Quick start

### 1. Run the broker

```bash
cd broker-replication

# Load .env (PowerShell)
Get-Content .env | ForEach-Object {
  if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
    [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim().Trim('"'), 'Process')
  }
}

go run ./cmd/broker
```

Broker runs on **http://localhost:8080**.

### 2. Run the frontend

```bash
cd frontend
npm install
npm run dev
```

Frontend runs on **http://localhost:5173**.

### 3. Use the app

- **Register patient** – Add patients with urgency 1–5
- **View queue** – See next 5 patients, updates every 2 seconds
- **Call next** – Remove the first patient from the queue

---

## Modes

### Local (default)

No env vars needed. Uses in-memory queue. ER queue persists to `data/er-queue.json`.

### Azure

Set these in `.env` (copy from `broker-replication/.env.example`):

| Variable | Purpose |
|----------|---------|
| `AZURE_SERVICEBUS_CONNECTION_STRING` | Service Bus queue for broker |
| `AZURE_SERVICEBUS_QUEUE_NAME` | Queue name (default: `broker-queue`) |
| `AZURE_STORAGE_CONNECTION_STRING` | Blob Storage for broker WAL |
| `AZURE_STORAGE_CONTAINER_NAME` | Container name (default: `broker-logs`) |
| `BROKER_ADDR` | Listen address (default: `:8080`) |

---

## API

### Broker (generic queue)

- `POST /publish` – `{ "topic": "...", "body": "<base64>" }`
- `GET /consume?topic=...` – Returns one message or 404
- `POST /ack` – `{ "id": "..." }`

### ER Queue (frontend)

- `POST /er/register` – `{ "urgency": 1-5 }` → `{ "id", "patientId" }`
- `GET /er/next?limit=5` – Peek next N patients
- `POST /er/call` – `{ "id": "..." }` – Remove patient

---

## Project structure

```
broker-replication/     # Go backend
├── cmd/broker/          # Main entry point
├── internal/
│   ├── api/             # Broker HTTP handlers
│   ├── azure/           # Service Bus + Blob Storage
│   ├── er/              # ER queue + persistence
│   ├── queue/           # Queue interface + memory impl
│   └── service/         # Broker service
├── client/              # Producer client (standalone)
└── .env.example

frontend/                # React + Vite
├── src/
│   ├── api.js           # ER API client
│   └── views/           # Reception, Dashboard
└── vite.config.js
```

---

## Other commands

```bash
# Producer example (writes to local log)
go run ./cmd/producer_with_logcore

# Client tests
go test ./client
```
