# 04 — Domain Model

## Design goal

The domain model must be explicit enough to make controlled-environment farming easy to reason about, while hardware integration remains generic enough to support changing sensors and actuators.

GrowNerve borrows the useful farmOS idea that durable things and historical actions should be separate, but avoids reducing every record to generic assets and fields.

## Core aggregate roots

### Facility

A managed physical growing installation.

Examples:

- Home Indoor Farm
- Grow Room A
- Cebu Pilot Farm

A facility owns a location hierarchy and configuration context.

### Zone

A hierarchical operational location inside a facility.

Examples:

```text
Facility
  -> Room
     -> Tent
        -> Rack
           -> Level
```

A zone may contain plant positions, devices, reservoirs, or child zones. GIS geometry is not required for V0. A transform/layout definition may associate a zone with the 3D scene.

### Reservoir

A managed water/nutrient volume associated with one or more zones or grow cycles.

Important state includes:

- nominal capacity
- working volume when known
- chemistry channels
- water-temperature channel
- level channel
- associated pumps/air system

A reservoir is not just a device. It is an agricultural resource with history.

### Crop / Variety

Catalogue entities that describe what is being grown.

### GrowCycle

The central production record. A grow cycle groups a crop/variety, location, recipe, plant cohort/positions, stage history, observations, inputs, related telemetry, events, and harvest result.

Suggested statuses:

```text
planned
active
completed
abandoned
```

### PlantCohort / PlantPosition

V0 should avoid requiring individual plant records for every crop. A grow cycle may manage a cohort and fixed plant positions. An individual position can receive observations when needed.

Example:

```text
Grow Cycle #42
  cohort: 4 Bibb lettuce
  positions:
    P1
    P2
    P3
    P4
```

### GrowRecipe

A versioned desired operating strategy for a crop. Recipes contain ordered stages and stage-specific setpoints/schedules.

Published recipe versions are immutable. New changes create a new version so historical grow cycles remain reproducible.

### RecipeStage

Defines targets such as:

- expected duration or transition guidance
- photoperiod
- air temperature range
- relative humidity range
- water temperature range
- pH range
- EC range
- fan behavior

Stage transition may initially be manual or time-guided. Avoid complex crop-model automation in V0.

### Device

A physical controllable or sensing unit.

Examples:

- ESP32 controller
- smart relay
- LED fixture
- fan
- pH transmitter
- EC interface
- dosing pump

Devices expose logical channels.

### DeviceChannel

The stable logical point used by the application.

Examples:

```text
zone-01.air.temperature
zone-01.air.humidity
reservoir-01.water.temperature
reservoir-01.ph
reservoir-01.ec
fan-01.speed.command
fan-01.speed.feedback
light-01.state.command
light-01.state.feedback
```

The physical device providing a logical channel may be replaced without changing crop history.

Channel kinds:

- measurement
- state
- command
- counter

### Measurement

High-volume immutable telemetry attached to a channel and observation timestamp. Measurements are not farm events.

Recommended fields conceptually:

```text
channel
observed_at
received_at
value
unit
quality
sequence
```

### FarmEvent

A meaningful historical action or occurrence.

Examples:

```text
crop.seeded
crop.transplanted
reservoir.filled
reservoir.drained
input.water_added
input.nutrient_added
input.ph_adjusted
sensor.calibrated
equipment.maintained
automation.executed
command.executed
crop.harvested
```

Events may reference multiple domain entities and carry quantities and notes.

### EventQuantity

A typed quantitative value attached to a farm event.

Examples:

```text
2 L water
8 mL Nutrient A
8 mL Nutrient B
560 g harvested mass
```

The unit system must preserve dimension compatibility.

### InventoryItem / InventoryAdjustment

Consumables such as nutrients, pH solution, seeds, filters, and calibration fluids. Inventory balance is derived from append-only adjustments rather than overwritten silently.

### Observation

Human or automated crop/environment observation with category, severity, scope, notes, and media.

An observation can target:

- a grow cycle
- cohort
- plant position
- reservoir
- zone
- equipment

### Alert

A stateful exception requiring attention.

Lifecycle:

```text
open -> acknowledged -> resolved
```

Alerts should deduplicate repeated evaluation of the same continuing condition.

### AutomationRule

A configured policy that evaluates a condition/schedule and may propose or issue actions.

The rule is not allowed to bypass central safety policies.

### Command

A durable request to an actuator/device.

Suggested states:

```text
pending
published
acknowledged
applied
rejected
timed_out
cancelled
```

Commands must be idempotent by UUID.

### Calibration

Calibration is represented as a first-class historical event with structured calibration results. Current sensor health can project from the latest accepted calibration.

### Harvest

A result associated with a grow cycle, including quantity, quality notes, waste where needed, and completion timestamp.

## Relationships

```text
Facility
  1 -> many Zone

Zone
  0..many -> Device
  0..many -> Reservoir
  0..many -> PlantPosition

GrowCycle
  -> Crop
  -> Variety
  -> RecipeVersion
  -> Zone(s)
  -> Reservoir(s)
  -> PlantCohort/Positions
  -> Observations
  -> Events
  -> Harvest

Device
  1 -> many DeviceChannel

DeviceChannel
  1 -> many Measurement

FarmEvent
  -> many entity references
  -> many EventQuantity
```

## Domain invariants

Examples that must live server-side:

- an active grow cycle references an existing published recipe version if a recipe is assigned
- a completed recipe version cannot be mutated in place
- a measurement unit must match channel dimension
- a command target must be a command-capable channel/device
- an automation cannot issue a hazardous command when safety interlocks fail
- inventory adjustments cannot silently rewrite history
- a crop harvest cannot belong to a different facility than its grow cycle
- logical channel identity survives replacement of physical hardware

## Identity

Use UUIDv7 where available for application-created records. IDs are API identities and digital-twin binding keys. Human-friendly names/slugs are mutable labels and must not be used as durable foreign keys.
