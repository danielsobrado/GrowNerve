# 24 — Component and Plugin System

## Purpose

GrowNerve's digital-twin components are data, not Three.js classes.

The long-term rule is:

```text
component JSON -> validation -> registry -> normalized scene model -> Three.js
                                      \-> 2D UI / inspectors
                                      \-> MCP authoring
```

Three.js remains a renderer. Domain entities, channels, permissions, commands, and farm state remain authoritative outside the renderer.

This document is intentionally grounded in the current codebase. The component system is **not implemented yet**. The existing twin is the migration starting point, not something to replace wholesale.

## Current implementation baseline

Today the frontend already has the right seams:

- `frontend/src/domain/model.ts` defines `SceneEntity` and `SceneLayout` inside `FarmData`.
- `SceneEntity` currently contains `entity_type`, `entity_id`, `profile`, `position`, and `scale`.
- `frontend/src/twin/sceneState.ts` indexes scene nodes by `(entity_type, entity_id)` and maps `profile` to radial actions.
- `frontend/src/twin/DigitalTwin.tsx` renders procedural geometry with explicit branches for `zone`, `reservoir`, `light`, `fan`, and `plant`.
- `frontend/src/runtime/pilotData.ts` seeds the reference layout using those profiles.
- browser mode persists one transactional `FarmData` snapshot through Dexie.
- server mode persists the same configuration document with compare-and-swap concurrency; PostgreSQL projects only the relational identities it needs for telemetry.
- `.grownerve.json` is currently archive `schema_version: 1`.

That means the safest migration is additive:

```text
current SceneEntity + profile
        |
        v
SceneEntity + exact component_ref
        |
        v
registry resolves definition
        |
        v
renderer stops branching on profile
        |
        v
profile becomes migration-only compatibility data
```

Do not create a second farm model just for plugins.

## Design decisions

1. **Keep operational identity.** A scene object representing a real farm entity continues to use `(entity_type, entity_id)` as its identity. Do not invent a second component-instance ID for the same light, fan, reservoir, plant position, or sensor.
2. **Definitions are reusable; scene bindings are not.** A component definition describes appearance and generic capabilities. `SceneEntity` binds that definition to one existing domain entity.
3. **Use exact revisions.** A scene references an exact component `id`, SemVer `version`, and content `digest`.
4. **Components do not own telemetry.** Existing `Channel` UUIDs remain the telemetry/control identities. Components only declare compatible channel slots and bind those slots to real channels.
5. **Ports are topology, not telemetry.** Water, air, electrical, and mounting connections are ports/anchors. A telemetry channel is not modeled as a fake physical port.
6. **Capabilities are descriptive.** They allow GrowNerve to choose supported UI/workflows. They never grant permissions and never bypass safety.
7. **No executable community plugins in V0.** Component packs are declarative JSON + assets only.
8. **Keep farm state small.** Large GLBs/textures do not belong inside the whole `FarmData` snapshot or the PostgreSQL farm document.
9. **One validation contract.** Browser import, server install, tests, and MCP use the same schema fixtures and semantic rules.
10. **Migrate the current twin incrementally.** The procedural renderer is useful working software and becomes the first set of built-in primitive definitions.

## Terminology

### Component definition

A reusable visual/behavioral description such as:

```text
generic rectangular reservoir
240 W LED panel
circulation fan
generic pH probe
lettuce plant
```

A component definition does not represent a particular installed device.

### Scene binding

The placement of a component definition against an existing GrowNerve domain entity.

For operational components, the binding identity is already:

```text
(entity_type, entity_id)
```

This matches `entityKey()` in `frontend/src/twin/sceneState.ts` and preserves shared 2D/3D selection.

### Component pack

A versioned collection of component definitions and local assets.

"Plugin" in this architecture means a declarative component pack. It does not mean arbitrary JavaScript execution.

### Assembly

A reusable visual/topology composition such as an aeration kit. Assemblies may contain multiple visual children but do not create a new command or safety boundary.

### Component registry

The resolved catalog of immutable component revisions available to a runtime.

## Naming and JSON conventions

GrowNerve's existing portable/domain JSON uses `snake_case`. New component contracts should follow it instead of introducing a second style.

Use:

```json
{
  "schema_version": 1,
  "component_id": "grownerve.sensor.ph.generic"
}
```

not:

```json
{
  "schemaVersion": "1.0",
  "componentId": "grownerve.sensor.ph.generic"
}
```

Schema versions are simple positive integers. Component and pack versions use SemVer separately.

JSON Schemas target JSON Schema 2020-12 and should be closed by default (`additionalProperties: false` or equivalent) for contract objects. External `$ref` resolution is not allowed during untrusted pack validation.

## Component identity and immutable revisions

Stable component IDs use a namespace-like string:

```text
grownerve.sensor.ph.generic
grownerve.sensor.ec.generic
grownerve.actuator.pump.peristaltic
grownerve.structure.reservoir.rectangular
grownerve.plant.lettuce.generic
com.atlas-scientific.sensor.ph-ezo
org.example.light.led-panel-240w
```

A published revision is identified by:

```text
component_id + version + digest
```

Example:

```json
{
  "component_id": "grownerve.sensor.ph.generic",
  "version": "1.0.0",
  "digest": "sha256:..."
}
```

Rules:

- `component_id` is permanent once published.
- `version` follows SemVer.
- a published `(component_id, version)` is immutable.
- the same `(component_id, version)` with a different digest is a conflict and must be rejected.
- farms pin exact revisions; V0 does not resolve floating ranges such as `^1.0` at load time.
- upgrades are explicit migrations, never silent latest-version replacement.

The digest covers the normalized component manifest plus declared asset digests. Pack manifests keep a deterministic sorted file list with SHA-256 digests and sizes so integrity can be verified without trusting filenames.

## Target `SceneEntity` evolution

The current `SceneEntity` is close to what is needed. Extend it instead of replacing it with a parallel instance graph.

Conceptual target:

```ts
interface SceneEntity {
  entity_type: EntityType;
  entity_id: UUID;

  component_ref: {
    component_id: string;
    version: string;
    digest: string;
  };

  position: [number, number, number];
  rotation?: [number, number, number];
  scale: [number, number, number];

  configuration?: Record<string, unknown>;
  channel_bindings?: Record<string, UUID>;

  // Temporary compatibility field while v1 layouts migrate.
  profile?: string;
}
```

### Why there is no new operational instance ID

The current code already selects, indexes, alerts, and inspects by domain UUID. Giving the same fan a separate `component_instance_id` would create two identities that have to remain synchronized forever.

Only future scene-only nodes that genuinely have no domain entity may need their own UUID. Do not add that abstraction until a real use case requires it.

## V1 layout migration

Archive/layout migration from the current pilot is deterministic.

Example mapping:

```text
profile zone       -> grownerve.structure.zone.generic@1.0.0
profile reservoir  -> grownerve.structure.reservoir.rectangular@1.0.0
profile light      -> grownerve.light.panel.generic@1.0.0
profile fan        -> grownerve.air.circulation-fan.generic@1.0.0
profile plant      -> grownerve.plant.lettuce.generic@1.0.0
```

The migration adds `component_ref` while preserving the existing `entity_type`, `entity_id`, placement, and scale.

For one compatibility release the renderer may fall back to `profile` when `component_ref` is absent. New writes must use `component_ref` once the migration ships.

## Component definition contract

Keep the first schema small. It should describe only what the current product can consume.

Example pH probe definition:

```json
{
  "schema_version": 1,
  "component_id": "grownerve.sensor.ph.generic",
  "version": "1.0.0",
  "name": "Generic pH Probe",
  "category": "sensor",
  "tags": ["ph", "water", "hydroponics"],
  "model": {
    "type": "primitive",
    "primitive": "cylinder",
    "parameters": {
      "radius_m": 0.0125,
      "height_m": 0.18
    }
  },
  "capabilities": ["telemetry", "calibration"],
  "channel_slots": [
    {
      "slot": "ph",
      "semantic": "water.ph",
      "kind": "measurement",
      "value_type": "number",
      "dimension": "ph",
      "unit": "pH"
    }
  ],
  "anchors": [
    {
      "anchor_id": "probe_tip",
      "type": "water.submersible",
      "position_m": [0, -0.09, 0]
    }
  ]
}
```

The definition does **not** contain:

- channel UUIDs
- MQTT topics
- broker addresses
- ESPHome entity IDs
- Modbus registers
- REST URLs
- user permissions
- command authorization rules

Those belong to runtime/domain configuration.

## Channel slots and existing `Channel` integration

GrowNerve already has a strong logical channel model with:

```text
id
entity_type/entity_id
key
kind
value_type
unit
dimension
minimum/maximum
safe_minimum/safe_maximum
stale_after_seconds
```

The component system should reuse it.

A definition declares a **slot** describing what kind of channel may be bound. A `SceneEntity.channel_bindings` maps that slot to a real channel UUID.

Example:

```json
{
  "channel_bindings": {
    "ph": "0199..."
  }
}
```

Validation checks the referenced channel exists and is semantically compatible with the slot.

Do not put a fixed channel `key` such as `water.ph` into the definition as the runtime identity. `internal/registry` currently enforces channel-key uniqueness, and the farm owns the actual channel configuration.

## Capabilities and actions

Initial capability vocabulary should stay small and tied to real GrowNerve workflows:

```text
telemetry
calibration
switchable
variable_output
growth_stage
harvestable
```

The current `profileActions` table in `frontend/src/twin/sceneState.ts` becomes a compatibility adapter.

Target flow:

```text
component capabilities
       + domain entity type/state
       + available bound channels
       + runtime mode
       + user role
       + safety policy
              |
              v
      GrowNerve action resolver
              |
              v
    radial menu / inspector UI
```

Component JSON does not define privileged UI commands. At most it may contain presentation hints such as tooltip labels or preferred icon names. GrowNerve owns the action catalog and workflow implementation.

This prevents a third-party pack from creating a dangerous action by declaring `"action": "dose_now"`.

## Ports versus anchors

### Ports

Ports represent physical/topological flow or connection interfaces.

Initial examples:

```text
water.input
water.output
air.input
air.output
electric.ac_input
electric.dc_input
pipe.connection
```

Example pump ports:

```json
[
  {"port_id": "water_in", "type": "water.input", "direction": "input"},
  {"port_id": "water_out", "type": "water.output", "direction": "output"},
  {"port_id": "power", "type": "electric.ac_input", "direction": "input"}
]
```

Do not add `telemetry.ph`, MQTT, PWM, or relay as topology ports merely because they are data/control concepts. Those are channel bindings or device implementation details.

### Anchors

Anchors are spatial snap/mount points and are intentionally separate from ports.

Examples:

```text
mount.surface
mount.tent_pole
mount.shelf
water.submersible
reservoir.rim
```

An anchor can optionally reference a port when the physical connector and snap point are the same thing.

V0 compatibility uses a small explicit table. Do not build a general ontology engine.

## Model types

The current twin already proves procedural geometry is sufficient for a useful operational scene. Preserve that advantage.

### Primitive — required first

Initial primitive set:

```text
box
sphere
cylinder
plane
```

Add `pipe` only when visible connections are implemented.

Primitive parameters use real-world meters and closed schemas. Arbitrary shader/material code is prohibited.

The first built-in definitions should reproduce the exact geometry currently generated by `DigitalTwin.tsx`. That gives a low-risk renderer migration with deterministic visual regression tests.

### GLB — second

Use binary glTF (`.glb`) for authored runtime assets in V0. Supporting both `.gltf` plus arbitrary external sidecar files adds path/reference complexity without current value.

Requirements:

- meters as the runtime unit
- Y-up convention unless the importer normalizes explicitly
- deterministic origin/pivot convention
- local pack asset only
- no remote URI dependencies
- bounded triangle, texture, node, animation, and file-size budgets
- parse/inspection failure is a validation failure

Compression may be added when it has a measured benefit and all target runtimes support the required decoder.

### Composite / assembly — later

Composition is useful, but do not make it part of the first renderer refactor. The first goal is replacing hard-coded profile branches, not building a CAD graph.

## Registry architecture

The registry exposes immutable definitions from ordered sources:

```text
1. built-in revisions shipped with GrowNerve
2. locally installed revisions
3. explicitly bundled project revisions
4. remote catalog metadata (future, never required)
```

A request is always exact:

```text
get(component_id, version, digest)
```

There is no last-write-wins behavior.

### Browser storage

Do **not** put imported GLB bytes into the existing Dexie `snapshots` record. `BrowserFarmRepository.update()` currently clones and rewrites the full `FarmData` snapshot; putting large binary assets there would make every farm edit rewrite those assets.

When component packs are implemented, evolve the same IndexedDB database with separate stores such as:

```text
snapshots             existing FarmData snapshot
component_revisions   manifest/definition JSON keyed by revision identity
component_assets      Blob keyed by SHA-256 digest
component_packs       installed-pack metadata
```

This preserves the current repository architecture while isolating large immutable blobs.

### Server storage

The Go server currently stores the farm configuration as an opaque versioned JSON document and relationally projects facilities/devices/channels. Unknown farm-document fields are preserved; only measurements receive special relational handling.

Therefore scene bindings can evolve without redesigning PostgreSQL.

Installed component packs, however, should not live inside that whole farm document because they are global/reusable and may contain large assets. Add a small component-registry storage boundary when implementation begins. Reuse existing platform storage conventions, but do not route GLBs through `internal/media` while that service intentionally accepts images only.

## Farm archive compatibility

Current `.grownerve.json` archives are `schema_version: 1`. Component support changes the portable contract materially, so the archive must move to **schema version 2** rather than silently changing v1 semantics.

Version 2 should add a top-level dependency lock such as:

```json
{
  "component_lock": [
    {
      "component_id": "grownerve.light.panel.generic",
      "version": "1.0.0",
      "digest": "sha256:...",
      "source": "builtin"
    }
  ]
}
```

Migration rules:

```text
v1 archive
  -> validate using existing v1 validator
  -> migrate profile-based scene entries to built-in component_ref values
  -> produce normalized v2 FarmData/archive
```

Never mutate a v1 import partially before the complete v2 migration validates.

### Normal JSON export

`.grownerve.json` remains the normal human-inspectable farm archive. It carries exact component dependency metadata but not large GLB bytes.

### Bundled export

When a farm depends on local/community packs, optionally create a ZIP containing:

```text
project.grownerve.json
components/<pack-id>/<version>/pack.json
components/<pack-id>/<version>/components/...
components/<pack-id>/<version>/assets/...
```

The JSON project remains valid and inspectable without the ZIP container. Missing dependencies surface explicitly instead of being silently substituted.

## Component pack format

Development layout:

```text
hydroponics-pack/
  pack.json
  components/
    ph-probe.json
    ec-probe.json
  assets/
    ph-probe.glb
    ph-probe.webp
  LICENSE
  README.md
```

Example manifest:

```json
{
  "schema_version": 1,
  "pack_id": "org.example.hydroponics",
  "version": "1.0.0",
  "name": "Example Hydroponics Components",
  "components": [
    "components/ph-probe.json",
    "components/ec-probe.json"
  ],
  "files": [
    {
      "path": "components/ph-probe.json",
      "sha256": "...",
      "size": 1234
    }
  ]
}
```

V0 import uses ZIP only as a container. Do not introduce a custom binary format.

## Validation pipeline

Keep structure validation and domain validation separate.

```text
archive limits
 -> ZIP/path validation
 -> JSON parse
 -> JSON Schema 2020-12
 -> manifest/file-digest validation
 -> component semantic validation
 -> component-reference validation
 -> model inspection
 -> farm binding validation
 -> install/commit
```

Semantic checks include:

- finite positive dimensions
- finite transforms
- unique slot/port/anchor IDs within a definition
- known capability vocabulary
- compatible channel bindings
- exact component revision available
- no recursive composite graph when composites arrive
- pack-relative paths only
- declared files exist and match digest/size

### Archive hardening

Reject at minimum:

- absolute paths
- `..` traversal after normalization
- symlink/hardlink entries
- encrypted archives
- excessive entry count
- excessive compressed or expanded size
- duplicate normalized paths
- remote model/texture URLs
- executable code formats

Limits belong in configuration, not scattered constants.

## Security boundary

Allowed V0 pack content:

```text
JSON
GLB
approved raster image formats
plain/localization text
Markdown documentation treated as untrusted content
```

Not allowed:

```text
JavaScript
WebAssembly
custom shader source
shell commands
native libraries
network callbacks
remote asset hot-links
```

Component metadata is never an authorization source.

## Implementation sequence from the current code

### C0 — Freeze the current compatibility fixture

Before refactoring, add/keep deterministic tests for the existing pilot scene:

- scene entity identity
- current layout transforms
- profile action resolution
- selection behavior
- WebGPU/WebGL fallback behavior

The current procedural pilot is the migration fixture.

### C1 — Schemas and built-in registry

Deliver:

- `component.schema.json`
- `pack.schema.json`
- component-ref schema
- channel-slot schema
- primitive model schema
- exact revision/digest rules
- built-in definitions matching current profiles

Exit criteria:

- every current pilot `profile` maps to one built-in component revision
- valid/invalid fixtures exercise all schema families

### C2 — Additive `SceneEntity` migration

Deliver:

- `component_ref`
- optional `rotation`
- optional configuration/channel bindings
- deterministic v1 profile -> component-ref migration
- compatibility fallback for old scene data

Exit criteria:

- current v1 pilot data opens unchanged
- migrated data preserves the same `(entity_type, entity_id)` selection keys

### C3 — Generic primitive renderer

Refactor `DigitalTwin.tsx` so primitive/model rendering is selected by resolved component definition rather than `binding.profile` branches.

Exit criteria:

- pilot visuals and interactions remain equivalent
- no hydroponic component type requires a renderer `if profile === ...` branch

Equipment-specific dynamic state such as fan rotation and reservoir fill remains implemented by normalized render-state behaviors owned by GrowNerve, not arbitrary plugin code.

### C4 — Capability/action resolver

Replace `profileActions` with domain/capability-based action resolution while preserving authorization and command safety.

### C5 — Browser/server registry persistence

Add separate immutable definition/asset storage without bloating `FarmData` snapshots.

### C6 — Archive v2 and bundled export

Implement v1 -> v2 migration, dependency lock, missing-dependency UX, and optional bundled pack export.

### C7 — GLB loading and validation

Only after primitives are stable.

### C8 — Ports/anchors/connections

Add physical topology and placement metadata only when layout editing needs it.

### C9 — MCP adapter

Expose the same component/layout services using `25-mcp-component-authoring.md`.

## Testing requirements

Unit/contract coverage:

- schema validation
- v1 -> v2 migration fixtures
- exact revision/digest conflict rules
- channel-slot compatibility with the existing `Channel` model
- registry resolution order
- archive path/size/file-digest validation
- profile compatibility mapping
- action-resolution capability rules

Browser tests:

- old pilot archive still opens
- migrated pilot selects the same entities
- imported primitive definition renders
- local component registry survives reload
- component assets are not rewritten with every farm snapshot update
- missing dependency is explicit
- bundled export/import preserves exact digests

Server tests:

- component-ref fields survive whole-document save/load
- farm CAS conflicts still protect layout edits
- component pack storage is independent from farm-document CAS
- unauthorized component/layout mutations fail before persistence

Visual regression:

Use the existing deterministic pilot camera/layout before and after the renderer refactor. Pixel-perfect equality is not required across WebGPU/WebGL, but object placement, identity, and semantic state must remain stable.

## Non-goals for the first implementation

Do not add these to make the system appear more general:

- arbitrary executable plugins
- arbitrary shaders
- a public marketplace requirement
- automatic component upgrades
- SemVer range resolution at farm load
- a second identity for every operational scene object
- a second telemetry model parallel to `Channel`
- a CAD/Blender-like editor
- a generic semantic ontology engine
- remote asset hot-linking

The first success criterion is simpler: the current pilot scene renders from validated component definitions with the same domain identities and without hard-coded component profile branches.