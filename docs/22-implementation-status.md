# 22 — Implementation Status

This document is a ledger, not a summary. Every entry states what the code
actually does, so a reader can plan against it without reading the source first.
An earlier revision overstated several items; those are corrected here.

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
- **server-sent-event live updates**, replacing polling. The stream carries
  invalidation hints only and no farm data
- **durable command delivery.** A command is persisted before publication, and a
  broker outage queues it in the outbox for retry rather than dropping it
- **audit records** for command requests and refusals, timeouts, media uploads,
  and configuration delivery
- **media storage** with content sniffed rather than trusted, an image
  allow-list, generated storage keys, and size limits
- forward/down PostgreSQL migrations and sqlc-generated access code
- MQTT protocol-v1 telemetry, command, acknowledgement, health, and edge
  configuration handling, with per-device broker credentials supported
- a deterministic MQTT device simulator that runs the same precedence engine as
  the firmware, and an **ESP32 reference firmware target** with persisted
  configuration, a watchdog, and fail-safe outputs
- one React application with server and browser adapters, IndexedDB persistence,
  versioned validated archives, transactional replace import, deterministic
  pilot data, and PWA/GitHub Pages builds
- a selectable digital twin that initialises WebGPU first and falls back to
  WebGL, with shared entity identity, inspector, radial actions, equipment
  state, water level, and plant state
- local Docker Compose services, production-oriented images, CI quality,
  security, generated-code, and integration gates, and operator runbooks

## Capability matrix

| Roadmap area | Browser runtime | Full server runtime | Current boundary |
|---|---|---|---|
| Facilities, zones, reservoirs, crops, grows, recipes | Implemented | Persisted as a versioned document with compare-and-swap concurrency | Per-resource endpoints can replace whole-document writes without changing the UI contract |
| Telemetry/history | IndexedDB + simulator | Relational append-only storage with bounded history, downsampling, and retention | Batching is per-message rather than time-windowed; tune before high sample rates |
| Events, observations, inventory, harvest | Implemented | Document persistence | Media objects are stored with validated content; binary retention policy is deployment-specific |
| Alerts and rules | Local active-session evaluation | Continuous server-side evaluation with hysteresis and deduplication | Notification delivery (email, push) is not implemented; alerts surface in the UI |
| Live updates | Local change bus | Server-sent events | — |
| Digital twin | Implemented | Same UI and identities | Procedural pilot geometry is used until approved GLB assets are produced |
| Low-risk commands | Acknowledged simulation | Durable-before-publish flow with safety limits, expiry sweeping, and outbox retry | Real relay/PWM application requires commissioned hardware |
| Essential schedules | Author/simulate | Versioned edge configuration delivered retained, with adoption tracked by device acknowledgement | Proven against the simulator and the reference firmware source; **not** proven on physical hardware |
| Authentication | Not applicable (local-only) | dev/local/OIDC, refused in production when set to dev | Provider integration is deployment-specific |
| Chemistry | Typed channels/setpoints/history-ready schema | Schema-ready | Real probes, calibration evidence, and drift policy are required |
| Automatic dosing | Intentionally unavailable | Intentionally unavailable | Prohibited until every entry criterion in `02-scope-and-releases.md` is evidenced |

## Known limitations

These are stated plainly rather than left for a reader to discover.

- **Configuration is a document, not a row per entity.** Facilities, zones,
  devices, channels, recipes, alerts, and commands live in one versioned JSON
  document; only measurements are relational. This is a deliberate split —
  see `20-architecture-decisions.md` — and it means a configuration write
  serialises against other configuration writes. At the documented V0 scale that
  is correct; it is not a design for hundreds of concurrent editors.
- **Frontend coverage is scoped.** The 80% threshold is enforced over
  `src/domain` and `src/runtime`. Screens, components, and the twin are covered
  by Playwright journeys rather than by unit coverage. This is stated in
  `frontend/vitest.config.ts` as well as here.
- **Alert notification is in-app only.** An open alert is visible in the UI and
  over the change stream. Nothing pages an operator who is not looking.
- **The reference firmware has no sensor drivers.** `readSensors()` reports the
  `unknown` quality for every channel until a commissioning engineer completes
  it against real hardware, so the server treats readings as untrustworthy
  rather than charting invented numbers.

## Deliberate external gates

Roadmap phases 9–12 include outcomes that cannot be truthfully completed by
repository code alone. They require selected electrical hardware, sensor
datasheets, wiring and enclosure review, device credentials, calibrated probes, a
flashed ESP32, failure-injection tests, and at least one observed pilot grow.
GrowNerve therefore fails closed: no repository default can energize real
equipment or perform nutrient/pH dosing.

Before any non-trusted-LAN exposure, configure `auth.mode: oidc` or `local`,
terminate TLS at a reverse proxy, provision per-device broker credentials and
ACLs using `deployments/mosquitto/mosquitto.production.conf` and
`acl.example`, and complete the threat review in `13-security-and-permissions.md`.
Configuration validation refuses to start a production process with development
authentication, wildcard CORS, an unbounded write rate, or an anonymous broker.

## Definition of "implemented"

Software marked implemented has source, automated tests, and a documented
runnable path. Anything proven only against the simulator says so. Hardware-
dependent items are marked gated until physical evidence exists; simulator
success is never presented as commissioning evidence.
