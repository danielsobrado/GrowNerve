# 22 — Implementation Status

This document is a ledger, not a product pitch. Every entry states what the code actually does so a reader can plan against it without reverse-engineering source or roadmap wording.

`17-implementation-roadmap.md` describes intended work. This file is authoritative for delivered status.

## Delivered software baseline

- one Go modular monolith with health/version endpoints, structured middleware,
  correlation IDs, security headers, per-client rate limiting, OpenAPI types,
  and RFC 9457 problem responses
- **authenticated and authorized** access: dev, local-token, and OIDC modes
  behind one interface, with viewer/operator/manager/administrator enforced at
  the HTTP boundary and revalidated on every write
- **compare-and-swap persistence.** The farm document is versioned and written
  through a conditional update, so concurrent writers conflict instead of losing
  updates. `internal/farm/concurrency_test.go` is the regression guard, and
  `TestPostgresConcurrentMutateKeepsEveryIncrement` proves the same against a
  real database
- **relational measurement storage.** Telemetry is append-only in `measurements`
  with a `latest_measurements` projection, bounded history and server-side
  downsampling endpoints, duplicate rejection, and a configurable retention job
- a continuously running server runtime: command expiry sweeping, alert
  evaluation, telemetry retention, edge configuration synchronisation, and
  outbox draining
- **server-side alert evaluation** with duration, hysteresis, deduplication, and
  device-liveness inference, restored across restarts so a reboot neither
  reopens nor forgets existing alerts
- **server-sent-event live updates** carrying invalidation hints rather than farm
  data
- **durable command delivery.** A command is persisted before publication, and a
  broker outage queues it in the outbox for retry rather than dropping it
- **audit records** for command requests/refusals, timeouts, media uploads, and
  configuration delivery
- **media storage** with content sniffed rather than trusted, an image allow-list,
  generated storage keys, and size limits
- forward/down PostgreSQL migrations and sqlc-generated access code
- MQTT protocol-v1 telemetry, command, acknowledgement, health, and edge
  configuration handling, with per-device broker credentials supported
- a deterministic MQTT device simulator that runs the same precedence engine as
  the firmware, and an **ESP32 reference firmware target** with persisted
  configuration, a watchdog, and fail-safe outputs
- one React application with server and browser adapters, IndexedDB persistence,
  archive schema v1 validation, transactional replace import, deterministic pilot
  data, and PWA/GitHub Pages builds
- a selectable digital twin that attempts WebGPU first and falls back to WebGL,
  using shared domain identity, procedural pilot geometry, HTML tooltips, radial
  actions, equipment state, reservoir level, and plant state
- local Docker Compose services, production-oriented images, CI quality,
  security, generated-code, and integration gates, and operator runbooks

## Digital-twin implementation detail

The current twin is intentionally simpler than the planned component/plugin architecture.

Implemented today:

```text
SceneEntity
  entity_type
  entity_id
  profile
  position
  scale
```

`frontend/src/twin/DigitalTwin.tsx` currently contains explicit procedural rendering branches for:

```text
zone
reservoir
light
fan
plant
```

`frontend/src/twin/sceneState.ts` currently maps `profile` to radial actions.

This is useful production-facing pilot code, but it is not yet a generic component registry. Documents 12 and 24 define the incremental migration that preserves `(entity_type, entity_id)` while replacing profile-driven rendering with validated component definitions.

## Capability matrix

| Area | Browser runtime | Full server runtime | Current boundary |
|---|---|---|---|
| Facilities, zones, reservoirs, crops, grows, recipes | Implemented | Persisted as a versioned document with compare-and-swap concurrency | Per-resource endpoints may replace whole-document writes later without changing the UI contract |
| Telemetry/history | IndexedDB + simulator | Relational append-only storage with bounded history, downsampling, and retention | Batching is per-message rather than time-windowed; tune before high sample rates |
| Events, observations, inventory, harvest | Implemented | Document persistence | Media objects are stored separately with validated content; retention policy is deployment-specific |
| Alerts and rules | Local active-session evaluation | Continuous server-side evaluation with hysteresis/deduplication | Notification delivery outside the app is not implemented |
| Live updates | Local change bus | Server-sent events | — |
| Digital twin baseline | Implemented | Same frontend and identities | Procedural/profile-driven pilot scene; no generic component registry yet |
| Component definitions/registry | **Not implemented** | **Not implemented** | Planned additive migration in `24-component-plugin-system.md`; current profiles become built-in compatibility definitions |
| Third-party component-pack import | **Not implemented** | **Not implemented** | Requires immutable registry storage, ZIP/asset validation, and archive v2 |
| GLB component assets | **Not implemented as pack system** | **Not implemented as pack system** | Current twin uses procedural geometry; authored GLB validation/storage is planned |
| Ports/anchors/connections | **Not implemented** | **Not implemented** | Add only when a real layout-editing workflow requires them |
| MCP component authoring | **Not applicable / not implemented** | **Not implemented** | Planned after component services exist; target is integrated Go `/mcp`, not a separate current service |
| Low-risk commands | Acknowledged simulation | Durable-before-publish flow with safety limits, expiry sweeping, and outbox retry | Real relay/PWM application requires commissioned hardware |
| Essential schedules | Author/simulate | Versioned edge configuration delivered retained, adoption tracked by device acknowledgement | Proven against simulator/reference firmware source; **not** proven on physical hardware |
| Authentication | Not applicable to local browser data | dev/local/OIDC, refused in production when set to dev | Provider integration is deployment-specific |
| Chemistry | Typed channels/setpoints/history-ready schema | Schema-ready | Real probes, calibration evidence, and drift policy are required |
| Automatic dosing | Intentionally unavailable | Intentionally unavailable | Prohibited until every entry criterion in `02-scope-and-releases.md` is evidenced |

## Portable archive status

The implemented normal archive is:

```text
format: grownerve
schema_version: 1
FarmData snapshot
optional MediaObject[]
```

The current validator checks the envelope, UUID uniqueness, and core referential integrity before browser replacement.

The component system is expected to require archive **schema version 2** because exact component dependency metadata changes the portable contract materially. V1 must remain importable through an explicit v1 -> v2 migration; the existing v1 format must not silently change meaning.

Archive v2 and bundled component-pack export are therefore **planned, not implemented**.

## Browser persistence status

The current `BrowserFarmRepository` uses one Dexie `snapshots` store containing the complete `FarmData` snapshot.

That is suitable for the current configuration size. It is deliberately **not** the intended location for imported GLB/texture bytes because each farm update clones and rewrites the snapshot.

Planned immutable component revisions/assets will use separate IndexedDB stores when the registry is implemented.

## Server component-storage status

The current Go server stores configuration as one versioned farm JSON document and relationally projects facilities/devices/channels required by telemetry. Unknown farm-document fields can survive that storage path, so future `component_ref` scene fields do not require redesigning farm persistence.

A global component registry and binary model store do not exist yet. They should be implemented as a separate storage boundary rather than embedding large reusable assets in the farm document.

The existing `internal/registry` package is a **telemetry identity projection**, not the planned 3D component registry. The two concepts must not be confused or merged merely because they share the word "registry".

## MCP status

There is no MCP server or MCP dependency in the repository today.

Planned direction in `25-mcp-component-authoring.md`:

- implement component/layout services first
- add MCP as an adapter inside the Go modular monolith
- expose the smallest useful read/validate/create/bind tool set
- reuse existing farm compare-and-swap, role authorization, audit, and command safety
- keep browser-only mode usable without MCP

Do not mark MCP work implemented until an MCP client can exercise the real service with automated protocol/domain tests.

## Known limitations

- **Configuration is a document, not a row per entity.** Facilities, zones,
  devices, channels, recipes, alerts, commands, and scene layouts live in one
  versioned JSON document; measurements are relational. A configuration write
  therefore serialises against other configuration writes. At the documented V0
  scale this is deliberate, not a design for many concurrent editors.
- **The twin is profile-driven today.** Adding a new visual type currently
  requires code/profile changes. That is the exact limitation the component
  registry refactor is intended to remove.
- **The twin uses procedural geometry.** Component-pack GLBs, immutable asset
  storage, and pack validation are not present yet.
- **Frontend coverage is scoped.** The 80% threshold is enforced over
  `src/domain` and `src/runtime`; screens/components/twin are covered mainly by
  Playwright journeys.
- **Alert notification is in-app only.** Nothing pages an operator who is not
  looking at GrowNerve.
- **The reference firmware has no sensor drivers.** `readSensors()` reports
  `unknown` quality until commissioning code is completed against real hardware,
  so the server treats those readings as untrustworthy rather than inventing data.

## Deliberate external gates

Roadmap phases 9–12 include outcomes that cannot be truthfully completed by repository code alone. They require selected electrical hardware, sensor datasheets, wiring/enclosure review, device credentials, calibrated probes, a flashed ESP32, failure-injection tests, and at least one observed pilot grow.

GrowNerve therefore fails closed: no repository default can energize real equipment or perform nutrient/pH dosing.

Before any non-trusted-LAN exposure, configure `auth.mode: oidc` or `local`, terminate TLS at a reverse proxy, provision per-device broker credentials and ACLs, and complete the threat review in `13-security-and-permissions.md`.

Configuration validation refuses to start a production process with development authentication, wildcard CORS, an unbounded write rate, or an anonymous broker.

## Definition of "implemented"

Software marked implemented has source, automated tests, and a documented runnable path. Anything proven only against the simulator says so. Hardware-dependent items remain gated until physical evidence exists; simulator success is never presented as commissioning evidence.

For the new component/MCP work specifically, documentation and schemas alone do **not** count as implementation.