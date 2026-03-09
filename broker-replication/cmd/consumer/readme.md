# Consumer Runner

This folder contains the consumer worker for the distributed queue system.

The consumer connects to the broker over HTTP, polls for messages on a topic, processes them, and sends an acknowledgement after processing.

## Project Structure

```text
cmd/
├── broker/
│   └── main.go
└── consumer/
    ├── main.go
    └── README.md
```

## Open the Project

From the root of `broker-replication`:

```powershell
cd "C:\Users\murta\Desktop\Microsoft Queue System\Distributed-Queue-System\broker-replication"
```

## Start the Broker

Open Terminal 1 and run:

```powershell
go run ./cmd/broker
```

Expected output:

```text
queue backend: in-memory (local)
WAL backend: disabled (no AZURE_STORAGE_CONNECTION_STRING set)
broker starting on :8080
```

Leave this terminal running.

## Start Consumer 1

Open Terminal 2 and run:

```powershell
cd "C:\Users\murta\Desktop\Microsoft Queue System\Distributed-Queue-System\broker-replication"
$env:CONSUMER_ID="consumer-1"
$env:CONSUMER_TOPIC="triage"
go run ./cmd/consumer
```

Expected startup output:

```text
[consumer-1] consumer started: broker=http://localhost:8080 topic=triage
```

## Start Consumer 2

Open Terminal 3 and run:

```powershell
cd "C:\Users\murta\Desktop\Microsoft Queue System\Distributed-Queue-System\broker-replication"
$env:CONSUMER_ID="consumer-2"
$env:CONSUMER_TOPIC="triage"
go run ./cmd/consumer
```

Expected startup output:

```text
[consumer-2] consumer started: broker=http://localhost:8080 topic=triage
```

## Publish Test Messages

Open Terminal 4 and run:

```powershell
cd "C:\Users\murta\Desktop\Microsoft Queue System\Distributed-Queue-System\broker-replication"
```

Then run these commands:

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/publish" -ContentType "application/json" -Body '{"topic":"triage","body":"cGF0aWVudC0x"}'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/publish" -ContentType "application/json" -Body '{"topic":"triage","body":"cGF0aWVudC0y"}'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/publish" -ContentType "application/json" -Body '{"topic":"triage","body":"cGF0aWVudC0z"}'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/publish" -ContentType "application/json" -Body '{"topic":"triage","body":"cGF0aWVudC00"}'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/publish" -ContentType "application/json" -Body '{"topic":"triage","body":"cGF0aWVudC01"}'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/publish" -ContentType "application/json" -Body '{"topic":"triage","body":"cGF0aWVudC02"}'
```

These decode to:

```text
patient-1
patient-2
patient-3
patient-4
patient-5
patient-6
```

## Expected Result

The messages should be split across the two consumers.

Example:

### Consumer 1

```text
[consumer-1] received message id=... topic=triage body=patient-1
[consumer-1] acked message id=...
[consumer-1] received message id=... topic=triage body=patient-4
[consumer-1] acked message id=...
```

### Consumer 2

```text
[consumer-2] received message id=... topic=triage body=patient-2
[consumer-2] acked message id=...
[consumer-2] received message id=... topic=triage body=patient-3
[consumer-2] acked message id=...
```

The exact split may vary, but each message should only be processed once.

## What This Proves

- Multiple consumers can run at the same time
- Both consumers can poll the same broker
- Messages are distributed across consumers
- Messages are acknowledged after processing
- No duplicate processing occurred during testing

## Optional Build and Test Commands

Run from the `broker-replication` root:

```powershell
go build ./...
go test ./...
```
