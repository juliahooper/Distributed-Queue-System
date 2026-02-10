# Single-Node Message Broker (Barebones)

**Team 1 · Pull-based queue · Go**

This is a **skeleton only**—no real logic, just packages, structs, interfaces, and stubs.

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

## Client Producer (Prototype)

There is a **producer client** in `client/` that applications can use to send messages. It is wired to a pluggable storage interface so we can later hook it up to the real log core / broker.

- **Files**
  - `client/producer.go` – `Producer` interface + concrete implementation using retries.
  - `client/message.go` – message framing: `[length][payload]` with JSON payload `{key,value}`.
  - `client/retry.go` – `RetryPolicy` + `withRetry` helper (simple exponential backoff).
  - `client/storage_stub.go` – in-memory `StubStorageClient` that assigns monotonically increasing offsets.
  - `cmd/producer_example/main.go` – small demo that constructs a producer and sends one message.

- **Storage integration**
  - `StorageClient` interface defines `Append(ctx, topic, data) (offset, err)` and is the only dependency on the future log core.
  - `StubStorageClient` is **temporary**; it does not talk to the broker yet and is clearly marked with `// TODO(team1): replace with real log core append`.

- **Running the example**

From the `broker-replication` directory:

```bash
go run ./cmd/producer_example
```

This will construct a stub storage client, send a single message on `example-topic`, and log the returned offset. Once the real append/log-core implementation exists, we can swap `StubStorageClient` for a concrete adapter without changing the producer API.

---

## How to run a quick demo today

Right now the **storage/log core and full broker service logic are not implemented yet**, so the HTTP broker process (`cmd/broker`) is still a stub. You can still demo what exists using the producer client and its tests.

- **1. Run producer/client tests**

From the `broker-replication` directory:

```bash
go test ./client
```

This runs:

- Serialization tests for the `[length][payload]` framing and JSON `{key,value}` payload.
- Retry behavior tests using a flaky in-memory storage implementation.

- **2. Run the producer example (stub storage)**

Also from the `broker-replication` directory:

```bash
go run ./cmd/producer_example
```

This:

- Creates a `StubStorageClient` (no real broker/storage yet).
- Constructs a `Producer` with a default retry policy.
- Produces a single message to topic `example-topic` and logs the returned offset.

---

## TL;DR

| Engineer | File | Job |
|----------|------|-----|
| **A** | `internal/api/server.go` | HTTP: decode JSON, call broker, return status codes. |
| **B** | `internal/queue/memory.go` | Thread-safe queue: `Enqueue` / `Dequeue` only. |
| **C** | `internal/service/broker.go` | Pull + pending: use queue, expose `Publish` / `Consume` / `Ack` to API. |

