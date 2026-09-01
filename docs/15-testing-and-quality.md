# 15 — Testing and Quality

## Quality goals

GrowNerve controls physical equipment, so correctness around commands, safety, time, and state transitions matters more than superficial test counts.

## Backend tests

### Unit tests

Prioritize pure domain logic:

- recipe version immutability
- stage/setpoint evaluation
- unit compatibility
- alert hysteresis/duration
- command state transitions
- safety policies
- dose-budget calculations later
- inventory balance calculations

### Integration tests

Use real PostgreSQL for:

- migrations
- sqlc queries
- transactions
- optimistic concurrency
- idempotency
- outbox behavior
- measurement ingestion

### MQTT integration

Run Mosquitto in test infrastructure where practical and validate:

- telemetry ingestion
- reconnect
- invalid payload rejection
- command publish
- acknowledgement
- timeout
- duplicate command acknowledgement

A software device simulator should support deterministic scenarios.

## Firmware tests

Where feasible separate pure policy/scheduler/config logic from hardware drivers so it can be tested on host or in embedded test builds.

Important scenarios:

- boot with valid saved config
- boot with corrupt config
- server disconnected
- clock uncertain
- duplicate command
- expired command
- local output bounds
- watchdog/reset recovery

## Frontend tests

### Unit/component

- query adapters
- status formatting
- stale-data behavior
- action capability filtering
- selection state
- chart annotation mapping

### E2E

Use Playwright for critical user paths:

- create/start grow cycle
- open live zone
- add observation
- acknowledge alert
- issue manual fan/light command
- see command success/rejection
- navigate from alert to entity
- interact with 3D entity

## 3D tests

Do not rely only on screenshots.

Test deterministic logic separately:

- entity binding index
- raycast result -> entity selection mapping
- instance ID -> plant position mapping
- render-state adapters
- radial menu actions by entity profile
- camera-focus target calculations

Browser tests should verify selection, tooltips, radial menus, and shared 2D/3D state.

Use visual regression for a few fixed scenes/camera presets to catch major rendering regressions.

## Safety test matrix

Every hazardous command path later requires negative tests for:

```text
sensor stale
sensor suspect
calibration expired
water level low
leak active
pump runtime limit reached
hourly dose limit reached
daily dose limit reached
conflicting operation active
device offline
emergency stop active
```

A rejected command is expected correct behavior.

## Property/fuzz testing

Good candidates:

- MQTT payload parsing
- unit conversion
- command state transitions
- rule parser/config validation

Go fuzz tests can protect parsers exposed to device/network input.

## Static and security checks

CI should include:

```text
gofmt/go vet
staticcheck
golangci-lint or focused equivalent
govulncheck
frontend lint/typecheck
dependency vulnerability scan
OpenAPI lint
secret scan
container scan when images exist
```

Do not enable overlapping linters just to maximize tool count.

## Generated-code drift

If sqlc/OpenAPI generated files are committed, CI should regenerate and fail on drift.

## Race testing

Run Go race-enabled tests for concurrency-sensitive packages and integration paths where practical.

## Performance tests

Before optimizing, define scenarios:

- sustained telemetry ingestion from expected device count
- current-state overview query
- chart query over a typical grow
- 3D live updates with representative scene size

The reference system must remain smooth while telemetry arrives.

## Definition of production-ready for a feature

A feature is not done until:

- behavior and error cases are specified
- server invariants exist
- migrations/API contracts exist if needed
- tests cover critical paths
- failure state is visible in UI
- logs/diagnostics are sufficient
- documentation/configuration is updated

## What is actually covered today

This section records the real state, so the checklists above read as intent and
this one reads as fact.

### Backend

- **Concurrency.** `internal/farm/concurrency_test.go` runs fifty concurrent
  ETag-guarded writers against the HTTP handler and fails if any write is lost.
  `TestPostgresConcurrentMutateKeepsEveryIncrement` asserts the same against a
  real database, which is where the retry backoff was found to be necessary.
- **Safety.** `internal/farm/authz_test.go` holds a negative matrix with one
  case per rejection reason — out of range in both directions, offline device,
  non-controllable channel, expired command, unknown channel, missing reason,
  unknown field — asserted at the HTTP boundary rather than only in the domain
  unit. It also proves an administrator cannot override an interlock.
- **Alerts.** `internal/alert/alert_test.go` covers duration suppression,
  deduplication across repeated evaluation, directional hysteresis, staleness
  superseding range checks, an offline device superseding staleness, device-set
  fault quality, restart restoration, and a removed rule resolving its alert.
- **Edge precedence.** `internal/edge/controller_test.go` covers operation with
  no server, reboot from retained configuration, override expiry, the
  controller's own timeout outranking a generous server expiry, emergency
  latching that does not stop aeration, hardware interlocks outranking commands,
  refusal of another device's configuration, and photoperiods crossing midnight.
- **Delivery.** `internal/farm/outbox_publisher_test.go` proves an accepted
  command survives a broker outage, that a successful publish is not also
  queued, that a permanently failing message parks, and that one outage does not
  burn every queued message's attempts.
- **Media.** `internal/media/media_test.go` proves HTML and SVG are refused even
  when named `.png`, that hostile filenames cannot escape the store, and that
  malformed identifiers are not-found rather than path lookups.

### Integration

Integration tests skip when their dependency is unconfigured and run when it is.
CI sets `GROWNERVE_TEST_DATABASE_URL` and `GROWNERVE_TEST_MQTT_BROKER` and
**fails the build if any of them report SKIP**, because an integration test that
silently skips is worse than one that does not exist.

Covered against a real PostgreSQL: compare-and-swap semantics, concurrent
mutation, measurement round-trip, duplicate rejection, the latest-value
projection refusing to go backwards, downsampling, retention, and referential
rejection of an unknown channel.

Covered against a real broker: telemetry reaching the measurement store,
rejection of an unregistered device and of malformed payloads, acknowledgement
updating the durable command record, and — the phase 8 exit criterion — a
controller that connects *after the server has stopped* still receiving its
retained configuration and running its photoperiod from it.

### Frontend

Unit coverage is enforced at 80% over `src/domain` and `src/runtime`. Screens,
components, and the digital twin are covered by the Playwright journeys rather
than by unit coverage; this scoping is deliberate and is stated in
`frontend/vitest.config.ts` and in `22-implementation-status.md` as well as here.

### Known gaps

- Reference-scene visual regression (GN-671) is not implemented.
- There is no load or soak test at the documented scale.
- Firmware policy logic is tested through its Go twin (`internal/edge`), not by
  an on-target or host build of the C++ itself.
