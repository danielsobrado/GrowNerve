# 14 — Observability and Operations

## Goals

GrowNerve must make operational failures visible without requiring developers to inspect raw logs or MQTT manually.

## Structured logging

Use structured logs with consistent fields:

```text
level
time
message
correlation_id
component
facility_id
zone_id
device_id
command_id
rule_id
```

Avoid logging secrets, raw tokens, or sensitive credential material.

## Correlation

Generate/propagate a correlation ID across:

```text
HTTP request
 -> application service
 -> database operation
 -> command creation
 -> MQTT publish
 -> acknowledgement processing
```

This is particularly valuable when troubleshooting control actions.

## Health endpoints

Expose at least:

```text
/health/live
/health/ready
```

Readiness should reflect required dependencies such as PostgreSQL. MQTT readiness semantics should distinguish whether the API can serve read-only farm data while broker control is unavailable.

## Device health

Each device should expose projected health including:

```text
online/offline
last heartbeat
firmware version
active config version
RSSI
uptime
reset reason
sensor faults
command acknowledgement health
```

## Operational dashboards

Admin/operations views should include:

- devices offline
- stale channels
- commands timed out/rejected
- automation faults
- MQTT connection status
- database/outbox backlog
- alert counts
- calibration overdue
- storage/media errors

## Metrics

Useful server metrics:

```text
HTTP latency/error rate
MQTT messages received
telemetry samples accepted/rejected
measurement insert latency
commands created/applied/rejected/timed out
outbox depth
active alerts
WebSocket/SSE clients
```

Do not introduce a heavy metrics stack into V0 unless needed; expose a Prometheus-compatible endpoint when operational deployment justifies it.

## Telemetry ingestion diagnostics

Rejected telemetry must be countable and diagnosable by reason:

```text
unknown_device
unknown_channel
unit_mismatch
invalid_payload
clock_outlier
implausible_value
duplicate_sequence
```

Avoid flooding application logs with one line per normal sensor sample.

## Backups

At minimum document:

- PostgreSQL backup procedure
- restore test procedure
- media backup if enabled
- configuration backup

A backup that has never been restored is unverified.

## Data export

Operators should eventually be able to export their own grow/event/measurement data in documented formats. Local-first means the user's farm history should not be trapped inside the UI.

## Failure visibility

The UI must distinguish:

```text
server unavailable
MQTT unavailable
device offline
channel stale
channel fault
command pending
command rejected
```

Do not collapse these into a generic red status dot.

## What the server emits today

### Structured logs

Every request logs method, path, correlation ID, and duration as JSON. The
correlation ID is taken from `X-Correlation-ID` when the client supplies a valid
UUID and generated otherwise, and it is echoed back on the response, so a single
operator action can be followed from the browser through the API and into the
audit record.

Named events worth alerting on in a log pipeline:

```text
command_timed_out          a command was never acknowledged before its expiry
device_marked_offline      a heartbeat went stale
command_queued_for_retry   the broker was unreachable when a command was accepted
outbox_message_parked      a queued message exhausted its attempts
telemetry_append_failed    accepted telemetry could not be persisted
audit_queue_full           audit entries were dropped under load
background_job_failed      a runtime job errored and will retry on its next tick
http_rate_limited          a client exceeded its allowance
authentication_failed      a credential was rejected
```

### Audit log

Security-relevant actions are written to `audit_log` with actor, action, target,
timestamp, and correlation ID: command requests including refusals, command
timeouts, media uploads, and edge configuration delivery. It is the record to
query when reconstructing an incident — see `23-development-and-operations.md`.

### Live change stream

`GET /api/v1/stream` emits small invalidation envelopes over server-sent events.
It carries no farm data, so it is safe to leave open, and a client that misses a
hint re-reads on reconnect.

### Not yet implemented

- No metrics endpoint. Operational signals come from logs and the audit table.
- No outbound notification. An open alert is visible in the UI and over the
  change stream; nothing pages an operator who is not looking. This is the most
  significant gap for genuinely unattended operation and is stated in
  `22-implementation-status.md`.
