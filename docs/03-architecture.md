# 03 — System Architecture

## Architectural style

GrowNerve's farm/control runtime starts as a modular monolith with explicit domain boundaries, plus an optional browser-only runtime for static deployments.

A separate **optional commerce service** is allowed as a deliberate external commercial boundary. It is not a decomposition of farm-control logic and is never required for crop operation.

### Full runtime

```text
Browser / tablet
       |
   HTTPS / WS
       |
+-------------------------+
| GrowNerve Go server     |
|                         |
| HTTP API                |
| domain services         |
| scheduler               |
| automation engine       |
| MQTT adapter            |
| alert evaluator         |
| background jobs         |
+------------+------------+
             |
        PostgreSQL

             ^
             | MQTT
        Mosquitto
             ^
             |
         ESP32 nodes
```

### Browser-only runtime

```text
Browser / tablet
       |
       v
+-------------------------+
| React application       |
|                         |
| browser app services    |
| local rule evaluation   |
| simulator/replay        |
| Three.js/WebGPU twin    |
+------------+------------+
             |
          IndexedDB
```

### Optional commerce runtime

The same frontend may optionally request product recommendations and purchase offers from a separate hosted service:

```text
GrowNerve React application
          |
          | optional HTTPS
          v
+-----------------------------+
| GrowNerve Commerce Service  |
|                             |
| product catalog             |
| compatibility mappings      |
| regional offers             |
| affiliate/referral config   |
| disclosures                 |
| aggregate attribution       |
+-------------+---------------+
              |
       Commerce PostgreSQL
              |
              v
     merchant APIs / feeds
```

The commerce service:

- has no MQTT credentials
- has no credentials for the farm/control PostgreSQL database
- is not authoritative for component technical definitions, telemetry, automation, alerts, commands, or grow history
- receives only minimum technical/market context needed for a recommendation, not the whole farm document
- owns merchant API credentials, affiliate configuration, regional offers, and merchant-specific link policy
- can be disabled completely by self-hosted/browser-only users

If it is unavailable, GrowNerve simply has no current commercial offers. Farm functionality continues unchanged.

See `27-commerce-catalog-and-referral-service.md`.

Media storage may begin on a filesystem-backed adapter in development and move to S3-compatible object storage when real image volume justifies it. Browser-only mode stores media in IndexedDB and includes it in portable exports when requested.

## Layering

### Domain

Contains entities, value objects, invariants, policies, and interfaces required by the business model. It knows nothing about HTTP, MQTT, SQL, IndexedDB, Three.js, commerce services, or ESP32 implementations.

Suggested server packages:

```text
internal/facility
internal/growcycle
internal/recipe
internal/inventory
internal/device
internal/telemetry
internal/event
internal/automation
internal/alert
internal/observation
internal/harvest
```

### Application

Coordinates use cases and transactions:

- start grow cycle
- assign recipe
- ingest validated measurement batch
- create observation
- evaluate setpoints
- issue command
- acknowledge command
- record inventory adjustment
- close harvest

Browser-only mode implements equivalent client-side use cases behind frontend application interfaces. Shared behavioral fixtures/contract tests keep observable rules aligned between runtimes.

Commerce/recommendation lookup is an optional presentation/application integration and must not be called from control/safety domain logic.

### Adapters

Server infrastructure implementations:

```text
internal/platform/postgres
internal/platform/mqtt
internal/platform/httpapi
internal/platform/media
internal/platform/auth
internal/platform/clock
```

Frontend application adapters conceptually include:

```text
server adapters    -> generated OpenAPI/live API
browser adapters   -> IndexedDB/local simulator
commerce adapter   -> optional remote commerce API or disabled/null adapter
```

React components and Three.js scene code must not depend directly on either farm persistence mechanism or merchant APIs.

## Server responsibilities

The Go farm/control server is authoritative for:

- configuration of facilities/zones/devices
- logical channel definitions
- crop/grow-cycle state
- recipe/setpoint state
- event history
- alerts
- command authorization and safety validation
- automation-rule evaluation
- inventory
- harvest history

The server is not responsible for millisecond hardware control loops.

The farm/control server is also not responsible for affiliate credentials, merchant catalog APIs, commercial sponsorship ranking, or referral attribution. Those belong to the optional commerce service.

## Edge responsibilities

ESP32 nodes are responsible for:

- reading attached sensors
- applying local calibration parameters where appropriate
- publishing telemetry
- reporting health and firmware information
- receiving commands
- acknowledging applied/rejected commands
- persisting essential schedules/safe states
- watchdog behavior
- retaining safe operation when disconnected from the server

The edge must never independently invent a nutrient dose.

## Browser responsibilities in full mode

The React application is responsible for:

- operational navigation
- current-state views
- history and charts
- configuration workflows
- observations/media
- command intent
- 3D digital twin rendering and interaction
- optional commerce recommendation surfaces when enabled

The browser never becomes the physical safety authority. A 3D radial-menu action is only a request to the API.

Commercial data never grants command permissions or changes safety rules.

## Browser responsibilities in browser-only mode

The same React application additionally owns:

- IndexedDB persistence
- local domain/application workflows
- JSON import/export
- deterministic simulator/replay
- local alert/rule evaluation while active
- PWA/offline application shell

Browser-only commands target the local simulator by default. The application must identify browser-only/simulated operation clearly and must not claim unattended hardware guarantees.

Commerce remains optional in browser-only mode. A user can disable the remote commerce client and keep a fully local/offline farm application.

## Frontend application boundary

UI features depend on typed repositories/services such as:

```text
FarmRepository
GrowCycleRepository
RecipeRepository
TelemetryRepository
EventRepository
InventoryRepository
AlertRepository
CommandRepository
MediaRepository
CommerceClient (optional, non-domain-critical)
```

The server adapter calls OpenAPI/live endpoints. The browser adapter uses IndexedDB/local runtime services. The commerce adapter calls only the external commerce API or resolves to a disabled/no-offers implementation.

This prevents server/browser/commerce mode conditionals from spreading through screens and 3D components.

## State flows

### Full-mode telemetry

```text
Sensor -> ESP32 -> MQTT -> ingestion adapter -> validation -> measurements -> current-state projection -> UI
```

### Browser-mode telemetry

```text
manual/import/simulator/replay -> validation -> IndexedDB measurements -> current-state projection -> UI
```

### Full-mode command

```text
UI/automation -> application service -> safety policy -> command record -> MQTT -> ESP32 -> acknowledgement -> command state/event
```

### Browser-mode command

```text
UI/automation -> browser application service -> local validation -> command record -> simulator -> acknowledgement/rejection -> UI
```

### Farm event

```text
User/system action -> domain validation -> append event + related quantities -> update projections -> UI timeline
```

### Optional commerce recommendation

```text
local technical need/selected component
 -> normalize minimum requirements
 -> commerce API
 -> compatible product/offer metadata
 -> disclosed recommendation UI
```

The flow does not upload the entire farm state and cannot mutate farm state or issue commands.

## Current-state projections

Do not force every operational screen to reconstruct state from raw events or telemetry. Maintain query-friendly projections for:

- latest channel value
- latest device heartbeat
- current alert state
- equipment current state
- grow-cycle current stage
- current inventory balance

Historical source records remain durable and auditable.

In browser mode these projections may be recomputed or persisted locally, but they must remain derived from the same source concepts.

Commercial offers are not farm-state projections and are not persisted as authoritative farm history.

## Real-time UI

Full mode should use one server-to-browser live channel, preferably WebSocket or Server-Sent Events depending on command/status requirements. Do not expose MQTT directly to the browser.

Live payloads identify changed resources by UUID so the 2D UI and Three.js scene can update the same entity store.

Browser mode emits equivalent local change notifications from the IndexedDB/application layer so the rest of the UI behaves the same way.

Commerce updates use ordinary bounded HTTPS queries/caching. They do not share the farm-control live channel.

## Time-series storage

Full mode starts with PostgreSQL. Use partitioning and retention/downsampling policies only when measurement volume demonstrates the need. Do not introduce a separate time-series database in V0.

Recommended measurement key:

```text
(channel_id, observed_at, sequence)
```

Browser mode stores bounded local measurement history in IndexedDB. Storage usage must be visible because browser quota varies by platform.

## Portable data boundary

Browser-only state is not disposable demo state. A versioned `.grownerve.json` archive can export/import all domain data and optionally media.

Stable UUIDs are preserved so the archive can later be imported into the full server runtime.

Import validation occurs before any destructive local write.

Transient merchant offers/referral URLs are not part of the authoritative portable farm archive. A local shopping-need record may be exported later, but it references technical requirements/products rather than expiring affiliate URLs.

See `21-browser-only-runtime.md` and `27-commerce-catalog-and-referral-service.md`.

## Failure boundaries

### Internet failure

Full local server, broker, devices, and UI on LAN remain operational. A cached browser-only PWA can also continue using IndexedDB.

Commercial offer refresh is unavailable; this has no effect on farm operation.

### Server failure

ESP32 nodes continue essential persisted schedules and safe states. No server-dependent new automation is performed.

### MQTT broker failure

Devices continue local essentials and buffer only a bounded amount of telemetry if memory allows. Do not risk device stability to retain history.

### Database failure

Server rejects state-changing operations requiring persistence. Existing edge schedules continue.

### Browser failure in full mode

No effect on control.

### Browser failure in browser-only mode

Local UI/rule evaluation stops until reopened. Persisted IndexedDB data remains available, but browser-only mode must never be relied on for unattended crop-critical scheduling.

### Commerce-service failure

Hide or disable current commercial offers and optionally retain clearly stale cached product identity. Do not block scene rendering, farm editing, import/export, alerts, history, telemetry, or command workflows.

## Scaling rule

Do not split farm/control services merely because domains have separate packages. A farm/control service split requires a measured operational or ownership need that outweighs distributed-system cost.

The optional commerce server is the deliberate exception because it holds external commercial credentials and policy, serves both static and server-backed clients, may have a different deployment/ownership lifecycle, and must be independently disposable without affecting farm operation.
