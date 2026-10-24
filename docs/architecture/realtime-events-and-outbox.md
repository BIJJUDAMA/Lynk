# Real-Time Events & Transactional Outbox - Lynk

> **Authoritative Specification: Event Streaming & Outbox Pattern**
> **Database Table:** `outbox_events` (Migration `000015_phase1_5_hardening`)
> **Streaming Protocol:** Server-Sent Events (SSE) via `/api/v1/events/stream`
> **Backend Subsystems:** `internal/outbox/` (dispatcher) & `internal/sse/` (hub)

---

## 1. Architectural Motivation

In high-concurrency marketplace platforms, business operations often produce side-effects: notifications, audit logging, search index invalidation, and real-time UI updates.

Directly publishing events to message brokers or external clients inside a database transaction creates two failure modes:
1. **Dual-Write Inconsistency:** The database transaction commits, but the event publish fails (or vice versa), leading to silent data loss or phantom notifications.
2. **Connection Starvation:** Network latency while publishing outside the database holds database connections open, rapidly exhausting the connection pool.

Lynk resolves both failure modes via the **Transactional Outbox Pattern** combined with a high-throughput **Server-Sent Events (SSE)** broadcast hub.

```text
[HTTP Request]
       |
       v
+--------------------------------------------------------+
| Database Transaction (ACID)                            |
|  1. INSERT INTO applications / contracts ...           |
|  2. INSERT INTO outbox_events (status = 'pending')     |
+--------------------------------------------------------+
       | (Committed atomically)
       v
+--------------------------------------------------------+
| Outbox Dispatcher (Background Loop)                   |
|  - SELECT ... FOR UPDATE SKIP LOCKED                   |
|  - Dispatches to SSE Hub & Cache Invalidation Bus       |
|  - UPDATE outbox_events SET status = 'processed'       |
+--------------------------------------------------------+
       |
       v
+--------------------------------------------------------+
| SSE Hub (internal/sse/hub.go)                          |
|  - Fan-out to connected campus member browser sockets  |
|  - GET /api/v1/events/stream (text/event-stream)       |
+--------------------------------------------------------+
```

---

## 2. Outbox Schema & Data Lifecycle

The `outbox_events` table (created in Migration `000015_phase1_5_hardening`) provides durable event storage:

```sql
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(128) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 5,
    last_error TEXT,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_pending_events
ON outbox_events (next_retry_at, created_at)
WHERE status IN ('pending', 'failed');
```

### 2.1 Event Types
Standardized domain events emitted across the platform include:
- `application.submitted`: Student applies to a job or project workspace.
- `application.accepted`: Lead accepts candidate proposal.
- `application.rejected`: Application proposal rejected.
- `contract.created`: Delivery contract formed.
- `contract.completed`: Deliverables approved and contract closed.
- `review.submitted`: Peer rating and qualitative feedback recorded.
- `workspace.member_invited`: Team invitation dispatched.

---

## 3. Dispatcher Polling & Retry Semantics

The outbox dispatcher daemon runs within the Go API backend:

1. **Non-Blocking Batch Fetch:**
   ```sql
   SELECT id, event_type, payload
   FROM outbox_events
   WHERE status IN ('pending', 'failed')
     AND next_retry_at <= NOW()
   ORDER BY created_at ASC
   LIMIT 50
   FOR UPDATE SKIP LOCKED;
   ```
   `FOR UPDATE SKIP LOCKED` allows multiple Go API replicas to process event batches concurrently without lock contention or duplicate deliveries.

2. **Exponential Backoff on Failure:**
   If event dispatch fails (e.g. transient Redis network failure):
   ```text
   next_retry_at = NOW() + (2 ^ retry_count * 500ms)
   ```
   - Maximum retries: 5 (`max_retries = 5`).
   - If `retry_count >= max_retries`, status transitions to `dead_letter` and an error is logged with distributed tracing context (`last_error`).

3. **Successful Completion:**
   Upon successful fan-out, the event is marked `processed`:
   ```sql
   UPDATE outbox_events
   SET status = 'processed', processed_at = NOW()
   WHERE id = $1;
   ```

---

## 4. Server-Sent Events (SSE) Hub (`internal/sse`)

The SSE subsystem provides low-overhead, unidirectionally streamed real-time updates to student browsers.

### 4.1 Endpoint Contract
`GET /api/v1/events/stream`
- **Auth Required:** Yes (Authenticated campus member session).
- **Response Headers:**
  ```http
  Content-Type: text/event-stream
  Cache-Control: no-cache
  Connection: keep-alive
  X-Accel-Buffering: no
  ```
  > `X-Accel-Buffering: no` instructs the Nginx reverse proxy gateway to bypass response buffering and deliver event bytes immediately to the client socket.

### 4.2 Stream Envelope
Each event adheres to standard SSE framing:
```text
id: 9b1c2d3e-4f5a-6b7c-8d9e-0f1a2b3c4d5e
event: application.submitted
data: {"application_id":"...","project_id":"...","applicant_name":"Jane Stanford"}

:keep-alive heartbeat
```

### 4.3 Heartbeats & Reconnections
- **Heartbeat Frequency:** The hub emits an empty comment line (`:keep-alive heartbeat`) every 15 seconds to prevent TCP socket timeouts across intermediate network proxies.
- **Client Reconnection:** The frontend `EventSource` automatically reconnects upon disconnection, transmitting the `Last-Event-ID` header.
- **Connection Capacity:** Each client connection uses a lightweight Go goroutine with minimal memory overhead (< 4KB), allowing thousands of concurrent students to maintain open event streams simultaneously.
