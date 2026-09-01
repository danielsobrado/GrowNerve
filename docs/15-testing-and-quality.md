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
