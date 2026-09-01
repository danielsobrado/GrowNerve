# 05 — Data Model

## Goals

The database should support reliable operational queries, durable historical records, and moderate time-series volume without introducing a second database prematurely.

PostgreSQL is the system of record.

## Naming and identity

- UUIDv7 primary keys for application-created records.
- `created_at` and `updated_at` use timezone-aware timestamps.
- External human names are not keys.
- Optimistic concurrency uses a version column on mutable configuration records where concurrent edits matter.

## Suggested tables

### Core location

```text
facilities
zones
zone_relationships or parent_zone_id
```

`zones` stores hierarchy through `parent_zone_id` initially. Avoid a separate closure table until deep-hierarchy query performance requires it.

### Crop production

```text
crops
varieties
grow_cycles
grow_cycle_positions
grow_cycle_stage_history
harvests
```

### Recipes

```text
grow_recipes
grow_recipe_versions
recipe_stages
stage_setpoints
stage_schedules
```

Published recipe versions are immutable.

### Water systems

```text
reservoirs
grow_cycle_reservoirs
```

### Devices and channels

```text
devices
device_bindings
device_channels
```

A device binding maps a logical channel to the current physical provider. History should retain previous bindings with validity intervals.

### Telemetry

```text
measurements
latest_measurements
```

`latest_measurements` can be a projection table maintained transactionally or by ingestion logic. Do not query the entire measurement table for every dashboard refresh.

Conceptual measurement columns:

```text
id UUID
channel_id UUID
observed_at TIMESTAMPTZ
received_at TIMESTAMPTZ
sequence BIGINT NULL
value NUMERIC
unit_id UUID
quality TEXT
source_device_id UUID
```

For high-frequency channels, numeric precision should be chosen per value class; avoid unbounded arbitrary precision if it harms ingestion unnecessarily.

### Farm history

```text
farm_events
farm_event_entities
farm_event_quantities
```

`farm_event_entities` permits an event to reference a grow cycle, reservoir, zone, plant position, device, or inventory item without copying fields into every event subtype. The event type itself remains from a controlled registry.

### Observations and media

```text
observations
observation_targets
media_objects
observation_media
```

Media bytes live behind a storage adapter; metadata stays in PostgreSQL.

### Inventory

```text
inventory_items
inventory_adjustments
```

Balance is the sum of accepted adjustments, optionally cached as a projection.

### Automation and commands

```text
automation_rules
automation_rule_versions
commands
command_attempts
actuation_events
manual_overrides
```

Automation rule changes are versioned so historical actions can identify which rule definition caused them.

### Alerts

```text
alert_definitions
alerts
alert_events
```

An alert instance represents an active incident. `alert_events` records opened, acknowledged, updated, and resolved transitions.

### 3D digital twin

```text
scene_layouts
scene_entities
```

Do not put Three.js implementation details into the domain tables. Store only domain-to-scene mapping/configuration such as:

```text
scene_id
entity_type
entity_id
asset_key
position_x/y/z
rotation_x/y/z
scale_x/y/z
parent_scene_entity_id
interaction_profile
```

GLB files and renderer-specific metadata remain in frontend/static asset configuration unless runtime editing requires server storage.

## Units

Use a canonical unit catalogue with dimensions. Values entering domain services are normalized or validated against expected dimensions.

Examples:

```text
temperature: degC
volume: L, mL
conductivity: mS/cm
mass: g, kg
flow: mL/min
relative_humidity: %RH
pH: dimensionless semantic unit
```

Store the actual submitted/observed unit when useful for traceability, but provide normalized values for calculations.

## Telemetry indexes

Start with:

```text
(channel_id, observed_at DESC)
(observed_at)
(source_device_id, received_at DESC)
```

Consider monthly/daily partitioning only after real volume measurements.

## Retention and rollups

V0 keeps raw telemetry at practical sampling intervals. Later retention policy may define:

- raw measurements for a bounded period
- 1-minute rollups
- 15-minute rollups
- daily summaries

Never downsample meaningful farm events or actuator command history.

## Transaction boundaries

Examples:

### Command creation

One transaction should:

1. validate domain and safety state
2. create durable command
3. create outbox/message record if using transactional publish
4. commit

MQTT publishing happens after durable persistence.

### Inventory event

A nutrient-addition operation should atomically create:

- farm event
- quantity records
- inventory decrement

This prevents crop history and stock from diverging.

## Outbox

Use a database outbox if reliable post-commit MQTT/event delivery is required. Do not use in-memory publish as the only path for commands whose existence matters after a crash.

## Schema migrations

- forward-only numbered migrations
- migration integration test from empty database
- migration test from latest supported prior schema where practical
- no manual production schema edits
