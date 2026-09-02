# GrowNerve

**Local-first indoor farm intelligence and control.**

GrowNerve is an open platform for managing controlled-environment growing systems: hydroponics, indoor LED farms, grow tents, racks, reservoirs, sensors, actuators, crop cycles, observations, recipes, automation, and harvest history.

The project starts deliberately small: one real indoor hydroponic installation must be useful before the architecture is generalized to larger farms.

## Product idea

GrowNerve combines four layers that are often separated:

1. **Crop management** — crops, varieties, grow cycles, stages, observations, media, inputs, harvests.
2. **Telemetry** — environmental and reservoir measurements from physical devices.
3. **Control** — schedules, rules, commands, actuators, alerts, and fail-safe automation.
4. **Digital twin** — an interactive 3D representation of the farm where zones, plants, reservoirs, sensors, lights, fans, pumps, and controllers are real selectable domain entities.

The core operating model is:

```text
Facility
  -> Zone
     -> Grow Cycle
     -> Reservoir
     -> Devices
        -> Channels
           -> Measurements
     -> Automation Rules
     -> Events
     -> Observations
     -> Harvest
```

## Runtime modes

GrowNerve has two supported runtime shapes.

### Full local/server mode

```text
React UI
   -> Go API/live updates
   -> PostgreSQL
   -> Mosquitto
   -> ESP32 controllers
```

This is the authoritative mode for real sensors, persistent farm automation, and physical equipment control.

### Browser-only mode

```text
React UI
   -> browser application services
   -> IndexedDB
   -> local simulator / imported telemetry
```

The browser-only build preserves the same normal UI, GrowCycle/recipe/event workflows, telemetry history, alerts, command workflow, and Three.js/WebGPU digital twin without requiring a backend.

It supports versioned `.grownerve.json` import/export so a farm can be backed up, moved between browsers, or later migrated into the full PostgreSQL deployment.

This mode is intended for GitHub Pages, offline/local planning, demonstrations, education, and lightweight farm record keeping. It cannot provide the same unattended real-hardware guarantees as the server + ESP32 runtime because browsers may suspend background work.

See [Browser-only / GitHub Pages runtime](docs/21-browser-only-runtime.md).

## First installation

The initial reference system is a small DWC indoor grow:

- one 3 x 3 ft grow tent
- 240 W LED fixture
- circulation fan
- air pump and two air stones
- approximately 30 L reservoir
- four plant positions
- ESP32-based sensing/control
- initial sensors: air temperature, relative humidity, water temperature, water level
- later sensors: pH and EC
- later actuators: dosing pumps and valves

The first version must remain useful without automatic nutrient dosing. Chemistry automation is intentionally delayed until sensor quality, calibration, control limits, and fail-safe behavior have been proven with real data.

## Architecture principles

- **Local-first:** essential farm operation must not depend on Internet connectivity.
- **Portable local data:** browser-only farms persist in IndexedDB and use versioned validated archives.
- **Edge-safe:** an ESP32 must retain essential schedules and safe states if the server disappears.
- **WebGPU first:** the 3D client targets Three.js WebGPU first, with an explicit WebGL fallback.
- **Domain-bound twin:** operational scene identity is the existing `(entity_type, entity_id)` identity used by the rest of the product.
- **Extensible twin target:** reusable visual definitions are moving toward renderer-agnostic JSON plus validated local assets. The current implemented pilot twin is still procedural/profile-driven; see the status document.
- **Declarative plugins:** planned V0 component packs contain validated definitions/assets and do not execute arbitrary plugin code.
- **One telemetry model:** reusable visual components bind to existing logical `Channel` UUIDs rather than creating a parallel sensor/control model.
- **One UI:** server-backed and browser-only deployments use the same screens, entity selection, 3D twin, and interaction model.
- **Typed domain:** use explicit agricultural and control concepts rather than a generic everything-is-an-asset database.
- **Event history:** meaningful farm actions are immutable historical events.
- **Telemetry is separate:** high-volume measurements are not stored as ordinary farm events.
- **Human before autonomy:** recommendations precede automatic dosing.
- **KISS:** one Go server, one PostgreSQL database, one MQTT broker, one React application until scaling proves a need for more.
- **SOLID boundaries:** domain logic does not depend on MQTT, HTTP, PostgreSQL, Three.js, MCP, or hardware implementations.
- **Fail closed for dangerous control:** stale sensors, low water, offline devices, or violated limits inhibit hazardous actions.

## Current stack

### Server

- Go
- `net/http`
- PostgreSQL
- pgx + sqlc
- OpenAPI 3.1
- Eclipse Paho MQTT client
- structured logging

### Frontend

- React 19
- TypeScript
- Vite
- React Three Fiber / Drei
- Three.js
- WebGPU renderer first with WebGL fallback
- TanStack Query dependency for query workflows
- Dexie / IndexedDB browser-runtime adapter
- Zod runtime archive validation
- PWA support for static/offline browser mode

### Edge

- ESP32 reference firmware in [`firmware/esp32`](firmware/esp32/README.md)
- MQTT
- persisted configuration in NVS
- watchdog and fail-safe outputs

### Local infrastructure

- PostgreSQL
- Mosquitto
- filesystem-backed validated image media storage in the current server implementation

No Kafka, Redis, Kubernetes, or microservices are required for the initial system.

## Planned component extensibility

The current twin renders the reference scene with procedural geometry and profile-specific renderer branches. The next extensibility track migrates that working model incrementally:

```text
existing SceneEntity/domain UUID
          |
          v
exact component_ref
          |
          v
validated component registry
          |
          v
primitive / GLB renderer
```

Important constraints:

- keep `(entity_type, entity_id)` as the operational scene identity
- exact immutable component revision = ID + SemVer + SHA-256 digest
- bind reusable component channel slots to existing `Channel` UUIDs
- keep large immutable GLB/texture assets outside the whole `FarmData` snapshot
- preserve archive schema v1 and introduce component dependencies through explicit archive v2 migration
- no arbitrary JavaScript/WebAssembly/shader/network code in component packs
- no automatic component upgrades

MCP authoring is planned **after** those component services exist. The proposed MCP adapter is part of the Go modular monolith and reuses normal validation, farm compare-and-swap, authorization, audit, and command safety.

## Documentation

The detailed implementation blueprint lives in [`docs/`](docs/README.md).

Key documents:

- [Product vision and principles](docs/01-product-vision.md)
- [Scope and releases](docs/02-scope-and-releases.md)
- [System architecture](docs/03-architecture.md)
- [Domain model](docs/04-domain-model.md)
- [Data model](docs/05-data-model.md)
- [Edge, MQTT and device protocol](docs/07-edge-and-mqtt.md)
- [Automation and safety](docs/09-automation-and-safety.md)
- [UI and interaction design](docs/11-ui-ux.md)
- [3D digital twin](docs/12-3d-digital-twin.md)
- [Implementation roadmap](docs/17-implementation-roadmap.md)
- [Initial engineering backlog](docs/18-engineering-backlog.md)
- [Browser-only / GitHub Pages runtime](docs/21-browser-only-runtime.md)
- [Implementation status and limitations](docs/22-implementation-status.md)
- [Development, operations, and commissioning](docs/23-development-and-operations.md)
- [Component and plugin system](docs/24-component-plugin-system.md)
- [MCP component authoring and farm editing](docs/25-mcp-component-authoring.md)
- [ESP32 reference firmware](firmware/esp32/README.md)

## Relationship to farmOS

GrowNerve borrows several strong domain ideas from farmOS at the conceptual level: durable identities, event/log history, quantities attached to actions, inventory derived from adjustments, hierarchical locations, and separation between physical sensors and logical data streams.

GrowNerve is intentionally narrower and more opinionated. Controlled-environment crop production, real-time telemetry, automation safety, grow recipes, setpoints, actuator commands, and a 3D operational digital twin are first-class concepts.

No farmOS source code is copied into this project.

## Status

The executable V0 software baseline is implemented: browser-only IndexedDB/PWA operation, the operational React UI and WebGPU-first digital twin, Go/PostgreSQL/MQTT server mode with authentication and role enforcement, compare-and-swap state concurrency, relational telemetry with bounded history, continuous server-side alert evaluation, server-sent live updates, durable command delivery, validated image media storage, an ESP32 reference firmware target, migrations, containers, CI, and automated unit/integration/E2E coverage.

The generic JSON component registry, third-party component packs, component GLB storage/import, archive v2, ports/anchors, and MCP server described in the extensibility documents are **planned and not implemented yet**.

Everything proven only against the simulator says so. Nothing in this repository can energize real equipment by default, and automatic nutrient or pH dosing is not implemented.

Quick browser start:

```bash
cd frontend
npm ci
npm run dev:browser
```

Quick full-stack start:

```bash
cp .env.example .env
docker compose up -d --build --wait
docker compose --profile simulator up -d simulator
cd frontend && npm ci && npm run dev
```

See [implementation status](docs/22-implementation-status.md) for the exact capability matrix and limitations. See [development and operations](docs/23-development-and-operations.md) for verification, authentication setup, the production checklist, backup, commissioning, and incident procedures.