# 17 — Implementation Roadmap

## Goal

Build GrowNerve as a sequence of end-to-end slices around the real pilot installation. Avoid building all backend models first and all UI later; each phase should leave a demonstrable, testable system.

The browser-only runtime is built early so GitHub Pages is a real usable deployment target rather than a late demo adapter.

## Phase 0 — Repository and foundations

Deliver:

- Go module and server skeleton
- React/TypeScript/Vite frontend
- configuration loader using YAML + environment secrets
- PostgreSQL + Mosquitto development stack
- migrations/sqlc/OpenAPI toolchain
- logging/correlation/health endpoints
- CI quality gates
- frontend application/repository interfaces that can support server and browser adapters

Exit criteria:

- server starts cleanly
- frontend starts cleanly
- migrations pass from empty DB
- CI-equivalent local commands documented

## Phase 0B — Browser-only runtime foundation

Deliver:

- explicit `server` and `browser` frontend runtime modes
- IndexedDB persistence adapter
- versioned local schema migrations
- deterministic local change notifications
- `.grownerve.json` export
- validated replace import
- optional base64 media export/import
- first-run create/example/import flow
- static-safe routing and `/GrowNerve/` asset base path
- PWA/application-shell caching
- manual `npm run build:browser` and `npm run deploy:pages`
- shared behavioral/contract fixtures for server/browser parity

Exit criteria:

- GitHub Pages build runs with no backend
- local data survives reload/browser restart
- export -> clear -> import restores equivalent data
- app can reopen offline after first load where supported
- UI clearly identifies browser-only/simulated control

## Phase 1 — Facility and crop domain

Deliver:

- facilities
- hierarchical zones
- reservoirs
- crops/varieties
- grow cycles
- plant positions/cohorts
- recipes/versioning/stages/setpoints
- initial operational UI
- equivalent browser-mode persistence/workflows

Exit criteria:

- create/configure pilot tent in both runtimes
- start a lettuce grow
- display current stage and recipe targets

## Phase 2 — Device/channel model and simulator

Deliver:

- device registry
- logical channels
- physical channel bindings
- MQTT protocol v1
- server device simulator
- browser local simulator
- device health/heartbeat
- current device/channel UI

Exit criteria:

- simulated ESP32 appears online in full mode
- equivalent local device appears in browser mode
- stable logical channels survive simulated device replacement

## Phase 3 — Telemetry

Deliver:

- telemetry MQTT ingestion
- measurement validation
- PostgreSQL persistence
- IndexedDB measurement persistence
- latest-value projection
- bounded historical queries
- charts
- stale/quality states
- browser replay/import path

Reference channels:

```text
air temperature
relative humidity
water temperature
water level
```

Exit criteria:

- live simulator data updates UI
- history renders correctly
- stale/offline conditions are visible
- the same charts work against both adapters

## Phase 4 — Farm events, observations, media

Deliver:

- event registry
- event quantities
- observations
- photo/media adapters
- grow-cycle timeline
- basic inventory adjustments
- harvest record
- portable media export/import in browser mode

Exit criteria:

- one grow can be documented from seeding through harvest in either runtime

## Phase 5 — Alerts

Deliver:

- target-aware setpoint evaluation
- alert definitions
- hysteresis/duration
- open/acknowledge/resolve lifecycle
- live alert UI
- browser-local evaluation while app is active

Exit criteria:

- simulated unsafe values create useful, deduplicated alerts in both runtimes

## Phase 6 — 3D digital twin V0

Deliver:

- Three.js WebGPU-first scene foundation
- reference 3 x 3 tent GLB scene
- entity binding index
- shared 2D/3D selection
- raycast picking
- HTML tooltips
- entity inspector
- radial menus
- alert highlight/focus
- plant positions and discrete growth visuals
- live reservoir/equipment/sensor status effects

Exit criteria:

- every pilot component can be selected in 3D
- selecting an alert can focus the affected object
- tooltips display live/freshness-aware data
- radial actions resolve the correct domain entity
- the same scene works on GitHub Pages using IndexedDB/simulator data

## Phase 7 — Low-risk control

Deliver:

- durable command model
- MQTT command/ack protocol
- server safety/capability validation
- light/fan/air-pump manual control
- command status UI
- 3D radial control actions
- audit history
- browser simulator command state machine

Exit criteria:

- simulator and then real device apply acknowledged commands in full mode
- browser simulator exercises the same UI workflow
- rejected/time-out commands are visible

## Phase 8 — Edge-resilient schedules

Deliver:

- light schedule
- fan minimum/schedule
- air-pump essential state
- edge config synchronization
- persisted edge config
- offline behavior
- manual overrides with expiry

Exit criteria:

- disconnect GrowNerve server and prove essential operation continues

Browser-only mode may author/simulate these schedules but is not considered a safe unattended execution environment.

## Phase 9 — Real V0 pilot

Deliver:

- real ESP32 integration
- real environmental sensors
- real water-temperature/level sensors
- real light/fan/air-pump control as appropriate
- commissioning checklist
- one complete lettuce grow

Exit criteria:

- V0 is used operationally, not only demonstrated
- issues from the real grow feed the next backlog

## Phase 10 — Chemistry monitoring

Deliver:

- pH/EC channels
- calibration events/workflows
- quality/drift/staleness policies
- recipe target comparison
- chemistry history

No automatic dosing.

## Phase 11 — Assisted dosing

Deliver:

- nutrient inventory
- pump flow/calibration model if pumps installed
- human-readable recommended additions
- manual confirmation/recording
- mix/wait/re-measure workflow

## Phase 12 — Guarded automation

Only after entry criteria in `02-scope-and-releases.md` are satisfied.

Deliver automatic bounded dosing state machine, limits, interlocks, observe-only mode, and exhaustive safety tests.

Automatic physical dosing remains full server/edge mode only.

## Phase 13 — Vision and optimization

Later work:

- camera assets
- scheduled images
- WebGPU/local inference
- canopy/growth metrics
- grow comparisons
- energy/water/nutrient efficiency
- recommendations

## Prioritization rule

When two features compete, prefer the one that improves reliability, portability, or understanding of the real farm over the one that merely makes the demo broader.
