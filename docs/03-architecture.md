# 03 — System Architecture

## Architectural style

GrowNerve starts as a modular monolith with explicit domain boundaries. The runtime should remain boring:

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

Media storage may begin on a filesystem-backed adapter in development and move to S3-compatible object storage when real image volume justifies it.

## Layering

### Domain

Contains entities, value objects, invariants, policies, and interfaces required by the business model. It knows nothing about HTTP, MQTT, SQL, Three.js, or ESP32 implementations.

Suggested packages:

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

### Adapters

Infrastructure implementations:

```text
internal/platform/postgres
internal/platform/mqtt
internal/platform/httpapi
internal/platform/media
internal/platform/auth
internal/platform/clock
```

## Server responsibilities

The Go server is authoritative for:

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

## Browser responsibilities

The React application is responsible for:

- operational navigation
- current-state views
- history and charts
- configuration workflows
- observations/media
- command intent
- 3D digital twin rendering and interaction

The browser never becomes the safety authority. A 3D radial-menu action is only a request to the API.

## State flows

### Telemetry

```text
Sensor -> ESP32 -> MQTT -> ingestion adapter -> validation -> measurements -> current-state projection -> UI
```

### Command

```text
UI/automation -> application service -> safety policy -> command record -> MQTT -> ESP32 -> acknowledgement -> command state/event
```

### Farm event

```text
User/system action -> domain validation -> append event + related quantities -> update projections -> UI timeline
```

## Current-state projections

Do not force every operational screen to reconstruct state from raw events or telemetry. Maintain query-friendly projections for:

- latest channel value
- latest device heartbeat
- current alert state
- equipment current state
- grow-cycle current stage
- current inventory balance

Historical source records remain durable and auditable.

## Real-time UI

V0 should use one server-to-browser live channel, preferably WebSocket or Server-Sent Events depending on command/status requirements. Do not expose MQTT directly to the browser.

Live payloads should identify changed resources by UUID so the 2D UI and Three.js scene can update the same entity store.

## Time-series storage

Start with PostgreSQL. Use partitioning and retention/downsampling policies only when measurement volume demonstrates the need. Do not introduce a separate time-series database in V0.

Recommended measurement key:

```text
(channel_id, observed_at, sequence)
```

Support batched insert from MQTT ingestion.

## Failure boundaries

### Internet failure

Local server, broker, devices, and UI on LAN remain operational.

### Server failure

ESP32 nodes continue essential persisted schedules and safe states. No server-dependent new automation is performed.

### MQTT broker failure

Devices continue local essentials and buffer only a bounded amount of telemetry if memory allows. Do not risk device stability to retain history.

### Database failure

Server rejects state-changing operations requiring persistence. Existing edge schedules continue.

### Browser failure

No effect on control.

## Scaling rule

Do not split services merely because domains have separate packages. A service split requires a measured operational or ownership need that outweighs distributed-system cost.
