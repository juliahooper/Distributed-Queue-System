# Single-Node Message Broker (Barebones)

**Team 1 · Pull-based queue · Go**

This is a **skeleton only**—no real logic, just packages, structs, interfaces, and stubs. Had an AI whip it up so the three of us can implement in parallel without stepping on each other’s toes.

---

## What’s in here

- **API layer** – HTTP entry points (decode JSON, call service, return status codes).
- **Queue layer** – Thread-safe in-memory queue (enqueue/dequeue).
- **Service layer** – Glue between API and queue: pull logic + pending list until clients ack.

Everything compiles; the rest is `panic("implement me")` and TODOs.

---

## Engineer A — API & Networking

**File:** `internal/api/server.go`

You own the HTTP/REST surface:

- **POST /publish** – Read JSON body (`topic`, `body`), call the broker’s `Publish`, return 201/400/500.
- **GET /consume** – Read `topic` from query, call `Consume`, return the message as JSON or 204/404.
- **POST /ack** (optional) – Read message ID from body, call `Ack`, return 200/400/404/500.

You **don’t** touch the queue or pending logic. You only decode requests, call the `service.Broker` interface, and write responses.

---

## Engineer B — The Data Structure

**File:** `internal/queue/memory.go`

You own the queue storage:

- **`Message`** – The thing we store (ID, Topic, Body).
- **`Queue` interface** – `Enqueue(Message)` and `Dequeue() (Message, bool)`.
- **`MemoryQueue`** – Mutex + slice (or list); implement `Enqueue` and `Dequeue` in a thread-safe way.

No HTTP, no topics semantics, no “pending” list—just a generic FIFO that the service layer will use.

---

## Engineer C — The Broker Service

**File:** `internal/service/broker.go`

You own the pull + ack logic:

- **`Broker`** – Interface: `Publish`, `Consume`, `Ack`. The API talks only to this.
- **`BrokerService`** – Holds a `queue.Queue` and a **pending** map (messages out but not yet acked).
- **Publish** – Build a message (e.g. with an ID), call `queue.Enqueue`.
- **Consume** – Pull from the queue (for the right topic if needed), put it in pending, return it to the client.
- **Ack** – Remove the message from pending by ID.

You’re the glue: API and queue stay decoupled; you orchestrate who gets what and what’s “in flight.”

---

## Wiring

`cmd/broker/main.go` is where we’ll wire it: `queue.NewMemoryQueue()` → `service.NewBrokerService(q)` → `api.NewServer(broker)` → `server.Run(":8080")`. Right now it’s just a stub so the tree builds.

---

## TL;DR

| Engineer | File | Job |
|----------|------|-----|
| **A** | `internal/api/server.go` | HTTP: decode JSON, call broker, return status codes. |
| **B** | `internal/queue/memory.go` | Thread-safe queue: `Enqueue` / `Dequeue` only. |
| **C** | `internal/service/broker.go` | Pull + pending: use queue, expose `Publish` / `Consume` / `Ack` to API. |

Barebones, AI-generated scaffold—fill in the rest.
