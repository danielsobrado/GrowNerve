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
