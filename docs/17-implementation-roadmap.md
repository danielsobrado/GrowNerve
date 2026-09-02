# 17 — Implementation Roadmap

## Goal

Build GrowNerve as a sequence of end-to-end slices around the real pilot installation. Avoid building all backend models first and all UI later; each phase should leave a demonstrable, testable system.

The browser-only runtime is built early so GitHub Pages is a real usable deployment target rather than a late demo adapter.

## How to read this roadmap now

This document began as a build sequence, but the repository has already implemented much of the original V0 baseline. `22-implementation-status.md` is authoritative for what exists today.

In particular, the selectable procedural digital twin already exists. The new JSON component/plugin work is therefore a **refactor and extensibility track over working code**, not a claim that component schemas were implemented before the current Three.js scene.

The component track is labeled `6R` below to make that distinction explicit.

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

## Phase 6 — 3D digital twin baseline

This baseline is already represented in the current implementation. See `12-3d-digital-twin.md` and `22-implementation-status.md`.

Baseline capabilities:

- React Three Fiber / Three.js scene
- WebGPU-first renderer with WebGL fallback
- procedural reference geometry
- scene bindings using `(entity_type, entity_id)`
- shared 2D/3D selection
- raycast picking
- HTML tooltips
- radial actions
- live equipment/reservoir/plant state

The current implementation intentionally becomes the compatibility fixture for the component refactor below.

## Phase 6R1 — Component contract and compatibility registry

See `24-component-plugin-system.md`.

Deliver:

- JSON Schema 2020-12 component/pack/component-ref contracts
- `snake_case` JSON matching the existing portable format
- stable component IDs with separate SemVer revisions and SHA-256 digest
- channel-slot model aligned with the existing `Channel` domain type
- small capability vocabulary
- primitive model definitions matching current procedural geometry
- built-in component revisions for every current `profile`
- exact deterministic profile -> component-ref migration
- valid/invalid schema fixtures

Exit criteria:

- every current pilot scene entry maps to one exact built-in component revision
- no second operational instance identity is introduced
- current v1 scene data can be normalized without losing `(entity_type, entity_id)` identity
- component conflicts are explicit rather than last-write-wins

## Phase 6R2 — Additive scene migration and generic primitive renderer

Deliver:

- additive `SceneEntity.component_ref`
- optional rotation/configuration/channel bindings
- one-release compatibility fallback for old `profile` layouts
- generic definition-to-render-model adapter
- primitive geometry renderer
- normalized render-state behaviors for dynamic state such as reservoir fill and fan rotation
- capability/domain-driven radial action resolver replacing `profileActions`

Exit criteria:

- the current pilot remains visually and semantically equivalent after migration
- renderer selection no longer branches on pilot profiles such as `binding.profile === "fan"`
- selecting an entity resolves the same UUID before and after migration
- command/safety logic remains outside component definitions

## Phase 6R3 — Registry persistence and portable archive v2

Deliver:

- browser IndexedDB stores for immutable component revisions/assets separate from the existing farm snapshot
- server-side component registry storage separate from the whole farm document
- exact dependency lock
- `.grownerve.json` schema v1 -> v2 migration
- missing dependency UX
- optional bundled ZIP export for local/community component packs
- transactional pack installation with digest/path/size validation

Exit criteria:

- old v1 archives still import through migration
- large GLB assets are not rewritten on every browser farm edit
- farm document compare-and-swap remains the concurrency boundary for scene bindings
- exported projects reproduce exact component revisions or report missing dependencies

## Phase 6R4 — GLB and component-pack support

Deliver:

- GLB inspection/validation/cache
- pack ZIP import/export
- local component browsing
- imported component rendering through the same registry path as built-ins
- configured model/texture/file budgets

Do not add arbitrary JavaScript, WebAssembly, shaders, remote asset hot-links, or a required marketplace.

Exit criteria:

- a user can import a valid third-party declarative component pack without code changes
- invalid/malicious packs fail before installation
- imported components work in both server and browser rendering modes

## Phase 6R5 — Ports, anchors, connections, and lightweight placement

Deliver only when a real layout-editing workflow needs them:

- physical topology ports
- spatial anchors
- explicit compatibility validation
- connection persistence/visualization where useful
- simple snap placement
- assemblies where they remove real repeated work

Do not build a general CAD editor.

## Phase 6R6 — MCP component authoring proof

See `25-mcp-component-authoring.md`.

The first proof uses the current Go server and MCP 2026-07-28 through an integrated `/mcp` endpoint.

Start with:

```text
components.list
components.get
components.validate
components.create
farms.get_layout
farms.set_component
farms.validate_layout
```

Deliver:

- deterministic read-only registry/schema discovery
- structured JSON Schema 2020-12 tool inputs/outputs
- primitive immutable component creation
- farm-layout binding mutation using the existing farm compare-and-swap version
- current viewer/manager/admin authorization boundaries
- audit source metadata for MCP writes

Exit criteria:

- an MCP client can create a valid primitive component without generating Three.js code
- the component can be bound to an existing pilot entity without creating a second identity
- MCP and normal validation paths accept/reject the same fixtures
- stale farm versions conflict rather than overwrite
- no MCP operation bypasses authorization, component validation, or command safety

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

## Later extensibility work

After local packs and MCP are proven against real use, consider:

- optional remote component catalog
- signing/trust metadata
- publisher tooling
- richer MCP asset/draft workflows
- richer assembly/editor ergonomics

None of these is required to validate the component architecture.

## Prioritization rule

When two features compete, prefer the one that improves reliability, portability, or understanding of the real farm over the one that merely makes the demo broader.