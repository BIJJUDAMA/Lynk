# Real-Time Events & Transactional Outbox - Lynk

> **Authoritative Specification: Event Streaming & Outbox Pattern**
> **Database Tables:** `outbox_events` (Migrations `000015` & `000023`), `audit_logs` (Migration `000023`)
> **Message Broker:** RabbitMQ 3.13 (`lynk.events` exchange, `lynk.dlx` dead letter exchange)
> **Streaming Protocol:** Server-Sent Events (SSE) via `/api/v1/events/stream`
> **Backend Subsystems:** `internal/outbox/` (relay), `internal/queue/` (AMQP), `internal/audit/` (auditor), `internal/sse/` (hub)

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
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
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


---

## 5. RabbitMQ Topology & Dead Letter Queues (`internal/queue`)

To decouple heavy background processing and cross-service audit requirements from client HTTP lifecycles, outbox events are published to RabbitMQ 3.13.

### 5.1 Exchange & Queue Architecture

```text
Outbox Dispatcher (PublishTx)
       | (ch.Confirm publisher confirm)
       v
[Exchange: lynk.events (topic, durable)]
       |
       +---> routing_key: event.audit.* ------------> [Queue: events.audit] ----------> Audit Consumer
       +---> routing_key: task.ai.embedding --------> [Queue: tasks.ai.embeddings] ---> AI Worker
       +---> routing_key: task.notification --------> [Queue: tasks.notifications] ---> Notification Worker
       |
  (Consumer Nack / Max Retries Exceeded)
       |
       v
[Dead Letter Exchange: lynk.dlx (topic, durable)]
       |
       +---> routing_key: # ------------------------> [Queue: tasks.dead_letter] (Dead Letter Queue)
```

### 5.2 Topology Specifications

| Component | Name | Type | Durability | DLX Association | Routing Keys |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Primary Exchange** | `lynk.events` | `topic` | Durable | N/A | `event.*`, `task.*` |
| **Dead Letter Exchange** | `lynk.dlx` | `topic` | Durable | N/A | `#` (catch-all) |
| **Audit Queue** | `events.audit` | Queue | Durable | `lynk.dlx` | `event.audit.*` |
| **Embedding Queue** | `tasks.ai.embeddings` | Queue | Durable | `lynk.dlx` | `task.ai.embedding` |
| **Notification Queue**| `tasks.notifications` | Queue | Durable | `lynk.dlx` | `task.notification` |
| **Dead Letter Queue** | `tasks.dead_letter` | Queue | Durable | None | `#` |

---

## 6. Reliability, Publisher Confirms & Exponential Jittered Retries

### 6.1 Transactional Outbox + Publisher Confirms
Publishing is executed in two guaranteed stages:
1. **Zero Dual-Write Invariant:** Events are recorded into `outbox_events` in the exact same PostgreSQL transaction as the state change via `TxRecorder.RecordEventTx(ctx, tx, event)`.
2. **Synchronous Publisher Confirms:** The outbox worker polls pending events and publishes them to RabbitMQ using channel publisher confirms (`ch.Confirm(false)`). The outbox record is marked `processed` only after RabbitMQ confirms broker persistence.

### 6.2 Consumer Retries with Full Jitter
Consumer execution failures follow randomized exponential backoff to avoid thundering herd conditions:
```text
T_backoff = min(MaxInterval, BaseInterval * 2^attempt)
T_wait    = rand(0, T_backoff)
```
- **Base Interval:** 500ms
- **Max Interval:** 30s
- **Max Retries:** 5 attempts
- Once `attempt >= MaxRetries`, the consumer rejects the message with `requeue = false`, automatically routing it to `lynk.dlx` -> `tasks.dead_letter` with complete error headers for investigation.

---

## 7. Audit Subsystem (`internal/audit`)

The platform implements an append-only, tamper-resistant audit infrastructure for institutional compliance.

### 7.1 Schema & Immutable Trigger

```sql
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id VARCHAR(64) NOT NULL,
    action VARCHAR(128) NOT NULL,
    entity_type VARCHAR(64) NOT NULL,
    entity_id VARCHAR(64) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address VARCHAR(45) NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    trace_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Immutable Trigger: Prohibits UPDATE or DELETE operations on audit_logs
CREATE TRIGGER trg_prevent_audit_log_modification
BEFORE UPDATE OR DELETE ON audit_logs
FOR EACH ROW
EXECUTE FUNCTION prevent_audit_log_modification();
```

### 7.2 Administrative Query Endpoint Contract

`GET /api/v1/audit/logs`
- **Role Required:** Admin (`RequireRole("admin")`).
- **Query Parameters:**
  - `actor_id` (string): Filter by member ID.
  - `action` (string): Filter by event action (e.g. `contract.completed`).
  - `entity_type` (string): Filter by target domain entity.
  - `limit` (int, default: 50, max: 200): Keyset page size.
  - `cursor_time` (RFC3339 timestamp): Keyset timestamp cursor.
  - `cursor_id` (UUID): Keyset ID tie-breaker cursor.
- **Keyset Pagination Predicate:**
  ```sql
  WHERE (created_at, id) < ($cursor_time, $cursor_id)
  ORDER BY created_at DESC, id DESC
  LIMIT $limit;
  ```
