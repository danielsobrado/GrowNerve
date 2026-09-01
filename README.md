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

It supports complete import/export through a versioned `.grownerve.json` archive so a farm can be backed up, moved between browsers, or later migrated into the full PostgreSQL deployment.

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
- **Portable local data:** browser-only farms persist in IndexedDB and can be exported/imported as versioned JSON archives.
- **Edge-safe:** an ESP32 must retain essential schedules and safe states if the server disappears.
- **WebGPU first:** the 3D client targets Three.js WebGPU first, with a deliberate fallback path where required.
- **One UI:** server-backed and browser-only deployments use the same screens, entity selection, 3D twin, and interaction model.
- **Typed domain:** use explicit agricultural and control concepts rather than a generic everything-is-an-asset database.
- **Event history:** meaningful farm actions are immutable historical events.
- **Telemetry is separate:** high-volume measurements are not stored as ordinary farm events.
- **Human before autonomy:** recommendations precede automatic dosing.
- **KISS:** one Go server, one PostgreSQL database, one MQTT broker, one React application until scaling proves a need for more.
- **SOLID boundaries:** domain logic does not depend on MQTT, HTTP, PostgreSQL, Three.js, or hardware implementations.
- **Fail closed for dangerous control:** stale sensors, low water, offline devices, or violated limits inhibit hazardous actions.

## Planned stack

### Server

- Go
- `net/http`
- PostgreSQL
- pgx + sqlc
- OpenAPI 3.1
- MQTT client
- structured logging

### Frontend

- React
- TypeScript
- Vite
- TanStack Router / Query
- Three.js
- WebGPU renderer first
- glTF/GLB assets
- IndexedDB browser-runtime adapter
- PWA support for static/offline browser mode

### Edge

- ESP32
- MQTT
- local persisted configuration
- watchdog and fail-safe outputs

### Local infrastructure

- PostgreSQL
- Mosquitto
- optional S3-compatible object storage when media requirements justify it

No Kafka, Redis, Kubernetes, or microservices are planned for the initial system.

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
- [3D digital twin and Three.js plan](docs/12-3d-digital-twin.md)
- [Implementation roadmap](docs/17-implementation-roadmap.md)
- [Initial engineering backlog](docs/18-engineering-backlog.md)
- [Browser-only / GitHub Pages runtime](docs/21-browser-only-runtime.md)

## Relationship to farmOS

GrowNerve borrows several strong domain ideas from farmOS at the conceptual level: durable identities, event/log history, quantities attached to actions, inventory derived from adjustments, hierarchical locations, and separation between physical sensors and logical data streams.

GrowNerve is intentionally narrower and more opinionated. Controlled-environment crop production, real-time telemetry, automation safety, grow recipes, setpoints, actuator commands, and a 3D operational digital twin are first-class concepts.

No farmOS source code is copied into this project.

## Status

Documentation and architecture definition are the first deliverable. Implementation begins only after the V0 boundaries and safety model are explicit enough to prevent the hardware, backend, browser-only runtime, and UI from evolving into incompatible prototypes.
