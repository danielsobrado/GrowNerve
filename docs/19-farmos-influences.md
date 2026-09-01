# 19 — farmOS Influences

## Purpose

GrowNerve is not a farmOS fork or replacement. farmOS has useful domain ideas proven in broad agricultural record-keeping, and GrowNerve selectively adopts those concepts while remaining narrower and control-oriented.

No farmOS source code should be copied into GrowNerve unless the project deliberately accepts the licensing consequences. These notes describe conceptual influence only.

## Concepts worth adopting

### Durable things vs historical actions

farmOS separates managed assets from logs/events. GrowNerve follows the same broad principle:

```text
Durable domain entities
  facility
  zone
  reservoir
  crop/grow cycle
  device
  inventory item

Historical actions
  seeded
  transplanted
  water added
  nutrient added
  calibrated
  maintained
  harvested
```

This avoids filling every entity with mutable `last_*` fields and preserves history.

### Quantities attached to events

A nutrient-input event should carry quantities such as:

```text
8 mL Nutrient A
8 mL Nutrient B
2 L water
```

rather than adding a new amount column to the schema for every action subtype.

### Hierarchical locations

farmOS location concepts translate well to indoor facilities, but GrowNerve uses topology rather than GIS as the primary model:

```text
Facility
 -> Room
    -> Tent
       -> Rack
          -> Level
```

### Inventory from adjustments

Inventory is append-only adjustment history, not a mutable number with unexplained changes.

```text
+5 L purchased
-8 mL applied
-8 mL applied
```

The current balance is derived/projected.

### Physical sensors vs logical data streams

GrowNerve makes this especially explicit through `Device` and `DeviceChannel` bindings. A probe can be replaced while `reservoir-01.ph` remains the durable logical channel.

### Portable UUID identities

Stable UUID identities make local installations, exports, future synchronization, and 3D bindings safer than relying on local integer IDs or human names.

## Where GrowNerve intentionally differs

### Controlled-environment focus

GrowNerve does not need to model every possible agricultural asset. It can make first-class assumptions about:

- grow cycles
- reservoirs
- stages
- recipes
- setpoints
- sensors
- actuators
- automation
- indoor spatial layouts

### Typed domain rather than extreme genericity

Do not implement a universal entity/field-value system merely to support arbitrary future farm concepts. Typed Go domain models and PostgreSQL schemas are easier to validate, query, and maintain for the defined product scope.

### Telemetry as a separate high-volume data class

Meaningful farm events and sensor readings have very different volume and query patterns. Measurements remain separate from farm events.

### Closed-loop control is central

GrowNerve treats command validation, edge behavior, schedules, automation, acknowledgement, and fail-safe control as core architecture rather than optional sensor extensions.

### 3D digital twin is first-class

Indoor farming benefits from spatially locating sensors, plant positions, reservoirs, lights, fans, pumps, and alerts. GrowNerve binds those real domain entities directly to an operational Three.js scene.

## Interoperability direction

A future adapter may support importing/exporting selected GrowNerve records to farmOS-compatible concepts where useful.

Potential mappings:

```text
GrowNerve Facility/Zone    <-> farmOS location-like assets
GrowCycle                  <-> plan/plant-related records
FarmEvent                  <-> log
EventQuantity              <-> quantity
InventoryAdjustment        <-> inventory adjustment semantics
Device/Channel             <-> sensor/data-stream concepts
```

Do not distort GrowNerve's core model merely to make this mapping one-to-one.

## Rule of thumb

Borrow farmOS ideas when they improve durable agricultural history. Keep GrowNerve opinionated wherever controlled-environment automation, safety, spatial visualization, or crop execution benefits from a stronger model.
