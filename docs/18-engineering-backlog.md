# 18 — Initial Engineering Backlog

This backlog is ordered by dependency and user value. Ticket IDs are provisional planning identifiers, not a commitment to an issue tracker.

## GN-000 — Foundations

- **GN-001** Initialize Go module and `cmd/server`.
- **GN-002** Add YAML configuration with environment overrides and validation.
- **GN-003** Add structured logging and correlation IDs.
- **GN-004** Add `/health/live` and `/health/ready`.
- **GN-005** Add PostgreSQL development service.
- **GN-006** Add Mosquitto development service.
- **GN-007** Add migration toolchain.
- **GN-008** Add pgx + sqlc generation.
- **GN-009** Add OpenAPI 3.1 specification and generation.
- **GN-010** Initialize React + TypeScript + Vite application.
- **GN-011** Add TanStack Router/Query.
- **GN-012** Add CI formatting, lint, test, security, generated-drift gates.
- **GN-013** Define frontend application/repository interfaces independent of HTTP/IndexedDB.

## GN-050 — Browser-only runtime and portability

- **GN-051** Add explicit frontend runtime mode bootstrap: `server|browser`.
- **GN-052** Add IndexedDB persistence wrapper behind typed repositories.
- **GN-053** Add browser database schema/version metadata.
- **GN-054** Add deterministic IndexedDB migration framework.
- **GN-055** Add local application change/event notification bus.
- **GN-056** Add browser-mode first-run create/example/import flow.
- **GN-057** Add deterministic pilot example loader without silent seeding.
- **GN-058** Define `.grownerve.json` archive schema v1.
- **GN-059** Add complete JSON export service.
- **GN-060** Add optional media-to-base64 archive export.
- **GN-061** Add archive JSON/schema validation.
- **GN-062** Add referential-integrity and domain validation before import.
- **GN-063** Add transactional replace import.
- **GN-064** Add import rollback/unchanged-state tests on failure.
- **GN-065** Add archive schema migration framework.
- **GN-066** Add golden archive fixtures and round-trip tests.
- **GN-067** Add deterministic export ordering tests.
- **GN-068** Add storage-usage view and reset-local-farm workflow.
- **GN-069** Add static-safe router mode for GitHub Pages.
- **GN-070** Add configurable Vite base path for `/GrowNerve/`.
- **GN-071** Add PWA manifest and application-shell/3D-asset caching.
- **GN-072** Verify IndexedDB survives PWA/application upgrades.
- **GN-073** Add manual `build:browser` and `deploy:pages` npm scripts.
- **GN-074** Add browser/server adapter contract test harness.
- **GN-075** Add visible browser-only/simulated runtime state.
- **GN-076** Add browser telemetry replay engine.
- **GN-077** Add browser local device simulator/fault scenarios.
- **GN-078** Add browser command lifecycle simulator.
- **GN-079** Add browser alert/rule evaluation while runtime is active.
- **GN-080** Add future server import path for `.grownerve.json` archives.

## GN-100 — Facility and crop model

- **GN-101** Facility CRUD and invariants.
- **GN-102** Hierarchical zone model.
- **GN-103** Reservoir model.
- **GN-104** Crop and variety catalogue.
- **GN-105** Plant-position model.
- **GN-106** GrowCycle lifecycle.
- **GN-107** GrowCycle-to-zone/reservoir assignment.
- **GN-108** GrowRecipe aggregate.
- **GN-109** Immutable recipe versions.
- **GN-110** Recipe stages.
- **GN-111** Typed stage setpoints and unit validation.
- **GN-112** Grow-cycle current-stage projection.
- **GN-113** Pilot tent seed/demo configuration.

## GN-200 — Devices and MQTT

- **GN-201** Device registry.
- **GN-202** DeviceChannel capability model.
- **GN-203** Physical-to-logical channel binding history.
- **GN-204** MQTT adapter lifecycle/reconnect.
- **GN-205** Protocol v1 telemetry schema.
- **GN-206** Protocol v1 health schema.
- **GN-207** Device heartbeat projection.
- **GN-208** Device/channel configuration API.
- **GN-209** Software device simulator.
- **GN-210** Simulator fault/offline scenarios.

## GN-300 — Telemetry

- **GN-301** Measurement schema/migration.
- **GN-302** Batched telemetry ingestion.
- **GN-303** Channel/unit validation.
- **GN-304** Sequence/duplicate handling.
- **GN-305** Measurement quality model.
- **GN-306** Latest-measurement projection.
- **GN-307** Historical measurement query endpoint.
- **GN-308** Telemetry retention configuration placeholder.
- **GN-309** Live browser update transport.
- **GN-310** Environmental/reservoir chart components.
- **GN-311** Stale/offline UI states.

## GN-400 — Events and crop records

- **GN-401** FarmEvent registry/model.
- **GN-402** Event entity references.
- **GN-403** EventQuantity and canonical units.
- **GN-404** Observation model.
- **GN-405** Observation targeting.
- **GN-406** Media storage interface.
- **GN-407** Photo upload/attachment.
- **GN-408** Grow-cycle merged timeline.
- **GN-409** Inventory item model.
- **GN-410** Append-only inventory adjustments.
- **GN-411** Harvest record.

## GN-500 — Alerts

- **GN-501** Setpoint evaluation service.
- **GN-502** Alert definitions.
- **GN-503** Hysteresis and duration handling.
- **GN-504** Alert deduplication.
- **GN-505** Open/acknowledge/resolve lifecycle.
- **GN-506** Live alert list.
- **GN-507** Alert-to-entity navigation.

## GN-600 — 3D foundation

- **GN-601** Three.js WebGPU renderer spike.
- **GN-602** Decide direct Three.js vs React Three Fiber from spike evidence.
- **GN-603** WebGPU capability/fallback bootstrap.
- **GN-604** Scene resource lifecycle/cache.
- **GN-605** GLB asset loading pipeline.
- **GN-606** Scene layout data model/API.
- **GN-607** SceneEntityBinding abstraction.
- **GN-608** Runtime entity-to-object index.
- **GN-609** Shared 2D/3D selection store.
- **GN-610** Interaction-only raycast layer.
- **GN-611** Selected-object highlight.
- **GN-612** Camera focus behavior.
- **GN-613** Camera presets.

## GN-650 — 3D pilot scene and interactions

- **GN-651** Model/optimize 3 x 3 tent shell.
- **GN-652** Model LED fixture.
- **GN-653** Model circulation fan.
- **GN-654** Model reservoir and water surface.
- **GN-655** Model net pots/plant positions.
- **GN-656** Model ESP32/controller enclosure.
- **GN-657** Model air pump/air stones.
- **GN-658** Add sensor/probe markers.
- **GN-659** Add initial lettuce growth-stage assets.
- **GN-660** HTML hover tooltip anchoring.
- **GN-661** Tooltip live-state adapter.
- **GN-662** HTML radial-menu positioning.
- **GN-663** Interaction profiles by entity type.
- **GN-664** Entity inspector panel.
- **GN-665** Alert highlight/focus from 2D UI.
- **GN-666** Reservoir fill-level visualization.
- **GN-667** Fan running animation.
- **GN-668** LED state visualization.
- **GN-669** Instanced plant positions with `instanceId` mapping.
- **GN-670** 3D accessibility/non-3D action parity.
- **GN-671** Reference-scene visual regression tests.

## GN-700 — Commands and low-risk control

- **GN-701** Command aggregate/state machine.
- **GN-702** Command idempotency.
- **GN-703** Command API.
- **GN-704** Safety/capability validation pipeline.
- **GN-705** MQTT command schema.
- **GN-706** MQTT acknowledgement schema.
- **GN-707** Timeout/retry policy.
- **GN-708** Device-simulator command support.
- **GN-709** Light manual command.
- **GN-710** Fan percentage command.
- **GN-711** Air-pump command where hardware allows.
- **GN-712** Command history UI.
- **GN-713** 3D radial command actions.
- **GN-714** Rejection/reason UI.

## GN-800 — Edge resilience and schedules

- **GN-801** Versioned edge configuration.
- **GN-802** Config MQTT delivery/ack.
- **GN-803** Persisted ESP32 config.
- **GN-804** Edge photoperiod scheduler.
- **GN-805** Fan minimum policy.
- **GN-806** Air-pump essential state.
- **GN-807** Command/config precedence policy.
- **GN-808** Manual override with expiry.
- **GN-809** Server-loss integration scenario.
- **GN-810** Watchdog/reset recovery tests.

## GN-900 — Real pilot commissioning

- **GN-901** Real ESP32 reference firmware target.
- **GN-902** Air temp/RH driver.
- **GN-903** Water-temperature driver.
- **GN-904** Water-level driver.
- **GN-905** Fan PWM/RPM integration.
- **GN-906** Light relay/smart-control adapter as selected.
- **GN-907** Air-pump control adapter as selected.
- **GN-908** Hardware commissioning checklist.
- **GN-909** Calibration/installation photos and metadata.
- **GN-910** Complete first operational grow.

## GN-1000 — Chemistry monitoring

- **GN-1001** pH channel capability.
- **GN-1002** EC channel capability.
- **GN-1003** Calibration event schema/workflow.
- **GN-1004** Calibration validity projection.
- **GN-1005** Chemistry drift/plausibility policies.
- **GN-1006** Chemistry target UI.
- **GN-1007** Chemistry timeline annotations.

## GN-1100 — Assisted/automatic dosing

Do not begin automatic actions until safety entry criteria are met.

- **GN-1101** Nutrient material/inventory workflow.
- **GN-1102** Dosing-pump calibration model.
- **GN-1103** Assisted dose recommendation.
- **GN-1104** Mix/wait/re-measure workflow.
- **GN-1105** Observe-only automation mode.
- **GN-1106** Dose safety budget policy.
- **GN-1107** Low-water lockout.
- **GN-1108** Stale/quality/calibration lockouts.
- **GN-1109** Emergency-stop domain model.
- **GN-1110** Bounded dosing state machine.
- **GN-1111** Exhaustive dosing negative tests.

## Backlog discipline

Each ticket should be split further only when implementation shows a genuinely independent responsibility. Avoid turning every function into a ticket. Definition of done follows `15-testing-and-quality.md`.
