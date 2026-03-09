# ER Queue Frontend

React + Vite app for the ER hospital queue: register patients (reception) and view/call next (dashboard).

## Setup

```bash
npm install
```

## Run

Start the backend first (from repo root):

```bash
cd broker-replication && go run ./cmd/broker
```

Then start the frontend:

```bash
npm run dev
```

Open http://localhost:5173. Use **Register patient** to add patients (ID + urgency 1–5). Use **View queue** to see next patient and up next five; the list polls every 2.5s. Click **Call next** to remove the current next patient from the queue.

## Env

- `VITE_API_URL` — API base URL (default: `http://localhost:8080`)
