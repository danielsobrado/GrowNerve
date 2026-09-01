# 06 — API Design

## API style

GrowNerve exposes a versioned REST API described by OpenAPI 3.1. The API is the authoritative interface for both 2D and 3D browser interactions.

Suggested prefix:

```text
/api/v1
```

The browser must never talk directly to PostgreSQL or MQTT.

## Resource groups

```text
/api/v1/facilities
/api/v1/zones
/api/v1/reservoirs
/api/v1/crops
/api/v1/varieties
/api/v1/grow-cycles
/api/v1/recipes
/api/v1/devices
/api/v1/channels
/api/v1/measurements
/api/v1/events
/api/v1/observations
/api/v1/inventory
/api/v1/alerts
/api/v1/automation-rules
/api/v1/commands
/api/v1/scene-layouts
```

## Query/read separation

Operational reads may return projections designed for screens instead of exposing raw tables.

Examples:

```text
GET /api/v1/overview
GET /api/v1/zones/{id}/status
GET /api/v1/grow-cycles/{id}/timeline
GET /api/v1/reservoirs/{id}/status
GET /api/v1/devices/{id}/health
```

These are legitimate application views, not violations of REST purity.

## Command endpoints

Do not expose actuator control as arbitrary patching of a device row.

Use explicit intent:

```text
POST /api/v1/commands
```

Conceptual request:

```json
{
  "targetChannelId": "...",
  "commandType": "set_percent",
  "value": 45,
  "reason": "manual_override",
  "expectedStateVersion": 12
}
```

The server applies authorization, safety policies, command limits, and idempotency before accepting it.

## Idempotency

State-changing endpoints that may be retried by clients should support an idempotency key. Device commands always have their own durable UUID and must be safely retryable.

## Concurrency

Mutable configuration resources should expose a version/ETag. Updates reject stale versions with a conflict response.

## Errors

Use a stable problem-details structure based on RFC 9457 concepts.

Example fields:

```text
type
title
status
detail
instance
code
correlationId
fieldErrors
```

Domain error codes remain stable even when wording changes.

Examples:

```text
COMMAND_SAFETY_INTERLOCK
SENSOR_DATA_STALE
INVALID_UNIT_DIMENSION
GROW_CYCLE_ALREADY_COMPLETED
RECIPE_VERSION_IMMUTABLE
DEVICE_OFFLINE
```

## Live updates

Expose a browser live-update endpoint using WebSocket or SSE.

Events sent to the browser should be small invalidation/update envelopes, for example:

```json
{
  "type": "entity.updated",
  "entityType": "reservoir",
  "entityId": "...",
  "version": 21
}
```

The client then updates TanStack Query state or fetches the required resource. Avoid streaming raw MQTT payloads into the UI.

## Telemetry queries

Support bounded queries:

```text
GET /api/v1/channels/{id}/measurements?from=...&to=...&resolution=raw
```

Later resolutions may include server rollups:

```text
1m
15m
1h
```

Enforce time-range and result-size limits.

## Timeline API

A grow-cycle timeline should merge relevant domain events and annotations while keeping sensor telemetry separate.

Example:

```text
GET /api/v1/grow-cycles/{id}/timeline
```

Timeline items may include:

- stage transition
- nutrient input
- reservoir refill
- observation
- calibration
- alert opened/resolved
- equipment maintenance
- harvest

Charts can request sensor series independently and overlay timeline annotations.

## 3D scene API

The digital twin needs two concerns:

1. domain state
2. spatial scene configuration

Recommended endpoints:

```text
GET /api/v1/scene-layouts/{facilityId}
GET /api/v1/entities/{entityType}/{entityId}/interaction
```

The scene layout returns bindings from model nodes to domain UUIDs. The interaction endpoint may return allowed actions based on entity state and permissions.

Do not encode user permissions or command safety solely in scene metadata.

## Generated client

Generate TypeScript API types/client from OpenAPI. Frontend feature code should not duplicate wire types manually.

## Pagination

Use cursor pagination for large timelines and lists where insertion order matters. Small configuration catalogues may use bounded page/limit pagination.

## API tests

Contract tests must cover:

- OpenAPI validation
- auth failures
- permission failures
- unit validation
- concurrency conflicts
- idempotency replay
- command safety rejection
- bounded telemetry queries
- malformed UUIDs
- unknown references
