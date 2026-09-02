# 24 — Component and Plugin System

## Purpose

GrowNerve's 3D components are data, not hard-coded Three.js classes.

The core rule is:

```text
JSON defines components.
The component registry resolves them.
Three.js renders them.
Application services provide state and actions.
MCP and the normal UI use the same component services.
```

This keeps the digital twin extensible without coupling the farm domain to Three.js, individual hardware vendors, or a specific deployment mode.

The component system must work identically in:

- full server mode
- browser-only / GitHub Pages mode
- simulator and replay mode
- imported/exported farms
- future MCP-assisted authoring

## Design principles

1. **Domain first.** Three.js is a renderer, not the source of truth.
2. **Declarative by default.** A component pack contains JSON and assets, not arbitrary executable JavaScript.
3. **Stable identity.** Component IDs are permanent names; versions change separately.
4. **Definition and instance are separate.** A component definition describes a kind of object. A component instance describes one placed/configured object.
5. **Logical signals, not protocols.** Components expose telemetry/control semantics without embedding MQTT, Home Assistant, ESPHome, Modbus, or REST details.
6. **Capabilities over type checks.** UI and behavior resolve from declared capabilities rather than `if component.type === ...` branches.
7. **Ports and anchors are first-class.** Connections and physical placement are modeled explicitly.
8. **Portable assets.** Imported components work offline and in static hosting.
9. **Validate before install.** Schema, semantic, referential, and asset validation happen before a pack enters the registry.
10. **One application service boundary.** UI, import/export, backend APIs, and MCP call the same component/farm services.

## Terminology

### Component definition

Describes a reusable type of object, for example:

- generic pH probe
- 240 W LED panel
- rectangular reservoir
- circulation fan
- lettuce plant

### Component instance

A concrete use of a definition in a farm. It owns placement, user configuration, bindings, and runtime state references.

### Component pack

A portable directory or ZIP containing one or more component definitions and their assets.

"Plugin" in this document means a declarative component pack. V0 plugins do **not** execute arbitrary JavaScript.

### Registry

The resolved catalog of available component definitions.

### Assembly

A reusable composition of component instances, such as an aeration system or complete DWC module.

### Farm layout

The set of placed component/assembly instances, transforms, connections, bindings, and scene metadata for a farm.

## Architecture

```text
                    GrowNerve UI / MCP
                           |
                           v
                 Component/Farm Services
                           |
              +------------+------------+
              |                         |
              v                         v
       Component Registry          Farm State
              |                         |
              +------------+------------+
                           |
                           v
                 Normalized View Model
                           |
              +------------+------------+
              |                         |
              v                         v
        Three.js Renderer          2D Inspectors
```

Neither the renderer nor MCP owns component semantics.

## Repository boundaries

The implementation should converge toward packages/modules with responsibilities similar to:

```text
component-schema      JSON Schema and version migration
component-core        normalized component model and invariants
component-validator   semantic/referential/asset validation
component-registry    built-in/imported component resolution
farm-layout           instances, transforms, assemblies, connections
three-renderer        rendering/picking/visual state adapter
mcp-server            MCP adapter over application services
```

Exact source paths may differ from this conceptual split. The dependency direction must remain:

```text
three-renderer -> component/farm application contracts
component-core -X-> Three.js
component-core -X-> MQTT
component-core -X-> HTTP
```

## Component identity and versioning

Do not put a mutable display name, filename, or version number into identity semantics.

Examples of stable IDs:

```text
grownerve.sensor.ph.generic
grownerve.sensor.ec.generic
grownerve.actuator.pump.peristaltic
grownerve.structure.reservoir.rectangular
grownerve.plant.lettuce.generic
com.atlas-scientific.sensor.ph-ezo
org.community.dwc.bucket-20l
```

Each definition has a separate semantic version:

```json
{
  "schemaVersion": "1.0",
  "id": "grownerve.sensor.ph.generic",
  "version": "1.0.0"
}
```

Rules:

- `id` is permanent once published.
- `version` uses SemVer.
- breaking component-contract changes require a major version.
- exported farms pin the exact resolved component version for reproducibility.
- imported packs with the same `id` + `version` but different content hash are rejected as conflicting.
- schema version and component version are separate concepts.

## Component definition shape

The canonical format is JSON and is validated by a versioned JSON Schema.

Example:

```json
{
  "schemaVersion": "1.0",
  "id": "grownerve.sensor.ph.generic",
  "version": "1.0.0",
  "name": "Generic pH Sensor",
  "category": "sensor",
  "tags": ["ph", "water", "hydroponics"],
  "model": {
    "type": "gltf",
    "src": "./models/ph-sensor.glb",
    "scale": [1, 1, 1],
    "rotation": [0, 0, 0]
  },
  "dimensions": {
    "width": 0.025,
    "height": 0.18,
    "depth": 0.025,
    "unit": "m"
  },
  "capabilities": ["telemetry", "configurable", "calibration"],
  "telemetry": [
    {
      "id": "ph",
      "label": "pH",
      "type": "number",
      "unit": "pH",
      "min": 0,
      "max": 14
    }
  ],
  "ports": [
    {
      "id": "probe",
      "type": "water.probe",
      "direction": "input"
    },
    {
      "id": "data",
      "type": "telemetry.ph",
      "direction": "output"
    }
  ],
  "anchors": [
    {
      "id": "probe-tip",
      "type": "water.submersible",
      "position": [0, -0.09, 0]
    }
  ],
  "ui": {
    "tooltip": {
      "fields": ["ph", "status"]
    },
    "radialMenu": [
      {"action": "inspect", "icon": "info"},
      {"action": "history", "icon": "chart", "requires": "telemetry"},
      {"action": "calibrate", "icon": "tune", "requires": "calibration"}
    ]
  }
}
```

The exact schema may evolve before implementation, but the concepts above are contractual.

## Definition versus instance

A definition describes what a component **is**. An instance describes where a particular component **is used**.

Example instance:

```json
{
  "id": "ph-sensor-01",
  "component": {
    "id": "grownerve.sensor.ph.generic",
    "version": "1.0.0"
  },
  "transform": {
    "position": [1.24, 0.42, -0.85],
    "rotation": [0, 1.57, 0],
    "scale": [1, 1, 1]
  },
  "configuration": {
    "name": "Reservoir pH"
  },
  "bindings": {
    "ph": "channel-uuid-for-reservoir-ph"
  }
}
```

Do not duplicate model metadata or generic capabilities on every instance.

## Farm layout JSON

Farm scene/layout data references component definitions instead of embedding Three.js implementation state.

Example:

```json
{
  "schemaVersion": "1.0",
  "farmId": "home-dwc",
  "components": [
    {
      "id": "reservoir-01",
      "component": {
        "id": "grownerve.structure.reservoir.rectangular",
        "version": "1.0.0"
      }
    },
    {
      "id": "ph-01",
      "component": {
        "id": "grownerve.sensor.ph.generic",
        "version": "1.0.0"
      }
    }
  ],
  "connections": []
}
```

The farm domain keeps authoritative UUID/entity bindings where required. Scene instance IDs are stable within the farm archive and must not replace domain UUIDs.

## Model types

V0 should support three model strategies.

### Primitive

For simple objects and AI/MCP-created placeholders that do not need an authored GLB.

Initial primitive set:

```text
box
sphere
cylinder
plane
pipe
```

Example:

```json
{
  "model": {
    "type": "primitive",
    "primitive": "cylinder",
    "parameters": {
      "radius": 0.15,
      "height": 0.35
    }
  }
}
```

Primitive parameter schemas are closed and versioned. Do not accept arbitrary shader/code expressions.

### glTF / GLB

Preferred for authored runtime models.

Requirements:

- meters as canonical runtime unit
- documented forward/up orientation
- deterministic pivot/origin conventions
- bounded polygon/texture budgets
- local pack-relative asset paths
- optional compression where supported

### Composite

A reusable component or assembly made from other registered definitions.

Composite graphs must be acyclic. Validation rejects recursive component references.

## Capabilities

Capabilities describe generic features the application can act on.

Initial examples:

```text
telemetry
configurable
calibration
switchable
variable-output
power-monitoring
flow-control
growth-stage
biological
harvestable
```

The UI resolves actions from capabilities rather than component categories.

Bad:

```ts
if (component.category === "pump") {
  showPowerButton();
}
```

Preferred conceptual rule:

```text
capability: switchable -> offer permitted switch action
capability: telemetry  -> offer history/current-value UI
capability: calibration -> offer calibration workflow
```

Capabilities never bypass normal authorization or physical safety checks.

## Telemetry and control semantics

Component definitions declare logical signals only.

They must not embed deployment-specific transport details such as:

```text
MQTT broker URL
MQTT topic
Home Assistant entity ID
ESPHome API address
Modbus register
REST endpoint
```

Instead:

```text
component telemetry/control semantic
              |
              v
       component instance binding
              |
              v
     logical DeviceChannel / command
              |
              v
 adapter: MQTT / simulator / future integration
```

This allows one component definition to work in server mode, browser simulation, replay, and future adapters.

## Ports

Ports represent logical or physical connection interfaces between components.

Examples:

```text
electric.ac
electric.dc
water.input
water.output
air.input
air.output
nutrient.output
sensor.probe
telemetry.ph
telemetry.ec
control.pwm
control.relay
network.logical
```

A pump may declare:

```json
{
  "ports": [
    {"id": "water-in", "type": "water.input", "direction": "input"},
    {"id": "water-out", "type": "water.output", "direction": "output"},
    {"id": "power", "type": "electric.ac", "direction": "input"}
  ]
}
```

Connection validation checks compatibility before the layout is saved.

V0 compatibility can use an explicit table maintained by GrowNerve. Do not invent a general ontology engine.

## Anchors

Anchors represent spatial attachment/snap points. They are separate from ports because not every physical mount is a flow/data connection and not every logical port has a visible attachment point.

Examples:

```text
mount.surface
mount.tent-pole
mount.shelf
water.submersible
pipe.connection
reservoir.rim
```

Anchor definitions use local model coordinates.

Later scene editing can snap compatible anchors while still storing explicit transforms in farm layout JSON.

## Declarative UI metadata

Component packs may request standard GrowNerve actions. They do not provide arbitrary UI code.

Example:

```json
{
  "ui": {
    "radialMenu": [
      {"action": "inspect", "icon": "info"},
      {"action": "toggle", "icon": "power", "requires": "switchable"},
      {"action": "history", "icon": "chart", "requires": "telemetry"}
    ]
  }
}
```

GrowNerve owns the implementation of standard actions and filters them by:

- declared capability
- current component/domain state
- user permissions
- runtime mode
- safety policy

A component pack cannot introduce a privileged command merely by naming it in JSON.

## State binding

Definitions may describe how normalized component state maps to visuals and UI, but must not reference raw API payload structures.

Conceptually:

```text
DeviceState / ChannelState / PlantState
                |
                v
       normalized component state
                |
        +-------+-------+
        |               |
        v               v
     Three.js          HTML UI
```

Examples of normalized state:

```text
power: on/off/pending/offline
output: 0..1
value + unit + freshness + quality
fillRatio: 0..1
alertSeverity
plantGrowthStage
```

## Plants

Plants use the same component framework but may declare biological/growth capabilities.

A plant definition may reference discrete visual stages:

```json
{
  "capabilities": ["growth-stage", "biological", "harvestable"],
  "visualStates": {
    "seedling": {"model": "./models/lettuce-seedling.glb"},
    "vegetative": {"model": "./models/lettuce-vegetative.glb"},
    "mature": {"model": "./models/lettuce-mature.glb"}
  }
}
```

Displayed growth is an operational representation, not a claim of exact biological simulation.

## Component pack format

A pack is a directory during development and a ZIP for import/export.

Example:

```text
hydroponics-pack/
  plugin.json
  components/
    ph-sensor/
      component.json
      models/
        sensor.glb
      textures/
      thumbnail.webp
    ec-sensor/
      component.json
      models/
        sensor.glb
  locales/
    en.json
  README.md
  LICENSE
```

Pack manifest example:

```json
{
  "schemaVersion": "1.0",
  "id": "org.example.hydroponics-pack",
  "name": "Example Hydroponics Pack",
  "version": "1.2.0",
  "components": [
    "./components/ph-sensor/component.json",
    "./components/ec-sensor/component.json"
  ]
}
```

V0 uses ZIP because it is universally supported. A custom file extension can be added later only if it improves UX without changing the internal format.

## Component registry

The registry exposes one normalized read interface over multiple sources:

```text
built-in
imported/local
farm-embedded dependencies
remote registry (later)
```

Conceptual operations:

```text
get(id, version)
list()
search(category/tags/capability)
install(pack)
uninstall(pack)
validate(pack)
resolveFarmDependencies(farm)
```

Resolution rules must be deterministic. A farm export records exact component versions and content hashes needed to reproduce the layout.

A future remote marketplace must be optional. GrowNerve must remain usable with only local packs.

## JSON Schema and validation

Maintain separate schemas rather than one unbounded document:

```text
schemas/component.schema.json
schemas/plugin.schema.json
schemas/farm-layout.schema.json
schemas/connection.schema.json
schemas/assembly.schema.json
```

Validation pipeline:

```text
parse
 -> JSON Schema validation
 -> semantic validation
 -> reference validation
 -> asset validation
 -> compatibility validation
 -> install/register
```

Semantic validation includes checks JSON Schema cannot express cleanly, for example:

- minimum is not greater than maximum
- composite graph is acyclic
- referenced component versions exist
- port IDs are unique within a component
- model paths stay inside the pack
- dimensions are finite and positive where required
- transforms contain finite numbers
- duplicate stable IDs are handled according to version/conflict rules

## Schema migration

Every persisted format includes `schemaVersion`.

Older supported documents are migrated into the current normalized model before use:

```text
stored/imported v1
      |
      v
migration chain
      |
      v
current normalized model
```

Renderers and application services should consume the normalized model, not carry branches for every historic schema.

## Browser-only mode

The component architecture is required to work without a server.

Browser mode stores:

- installed component definitions
- imported assets
- farm layout instances
- dependency/version metadata

in IndexedDB/application storage.

A `.grownerve.json` farm export should contain farm/domain data plus component dependency metadata. When portability requires bundled binary assets, GrowNerve may additionally export a ZIP archive that contains the JSON archive and referenced component packs/assets.

Do not force all GLBs into base64 inside the normal JSON format; large binary assets should remain separate archive entries when a bundled export is used.

## Security model

V0 imported packs are declarative.

Allowed content is limited to validated formats such as:

- JSON
- GLB/glTF according to policy
- supported image texture/thumbnail formats
- localization text
- Markdown documentation displayed as untrusted/plain content unless sanitized

V0 packs do **not** execute arbitrary JavaScript, WebAssembly, shaders, shell commands, or network calls.

Import protections include:

- ZIP path traversal prevention
- compressed/uncompressed size limits
- file count limits
- MIME/content validation
- asset path containment
- duplicate/conflicting ID detection
- finite geometry/metadata limits where practical
- remote URL rejection by default

Remote model/texture URLs are rejected by default because they break offline operation, reproducibility, privacy, and content validation.

## Assemblies

Assemblies allow reusable systems without turning every composition into a new opaque GLB.

Examples:

```text
Component: air pump
Component: air stone
Component: tube
Assembly: aeration system

Assembly: aeration system
Component: reservoir
Component: net pots
Assembly: DWC module
```

Assemblies store child component references, relative transforms, and internal connections. They can be instantiated more than once.

An assembly is not a new runtime safety boundary. Commands still resolve to the underlying domain/device entities.

## Initial built-in component library

Keep V0 intentionally small.

### Structures

- grow tent
- rack/shelf

### Reservoir/water

- rectangular reservoir
- bucket
- tube/pipe primitive
- water pump
- air pump
- air stone

### Lighting/air

- LED panel
- circulation fan

### Sensors

- air temperature
- relative humidity
- water temperature
- pH
- EC
- water level
- leak

### Controls

- smart plug
- relay
- PWM controller

### Plants

- lettuce
- tomato
- generic plant

The reference 3 x 3 DWC installation remains the acceptance scene.

## Implementation sequence

### C1 — Component specification

Deliver:

- component/plugin/farm-layout JSON Schemas
- stable ID/version rules
- ports, anchors, capabilities, telemetry semantics
- primitive model schemas
- sample component packs

Exit criteria:

- valid/invalid fixtures cover every schema family
- the reference installation can be represented without Three.js-specific JSON

### C2 — Component runtime

Deliver:

- normalized component model
- validator
- migrations
- registry
- deterministic dependency resolution

Exit criteria:

- built-in and imported definitions resolve identically
- conflicts are explicit rather than last-write-wins

### C3 — Generic Three.js renderer integration

Deliver:

- definition-to-render-model adapter
- primitive rendering
- GLB loading/cache
- generic selection/tooltip/radial action metadata

Exit criteria:

- renderer has no component-specific hydroponics branches for the pilot scene

### C4 — Farm layout and assemblies

Deliver:

- component instances
- transforms
- assemblies
- import/export
- deterministic layout loading

### C5 — Ports, anchors, and connections

Deliver:

- compatible connection validation
- visible connection representation where useful
- snap metadata foundation

Do not turn this phase into a full CAD editor.

### C6 — Pack management

Deliver:

- ZIP import/export
- install/uninstall/enable/disable semantics as needed
- browser IndexedDB storage
- asset validation

### C7 — MCP adapter

Expose component/farm services through MCP as defined in `25-mcp-component-authoring.md`.

### C8 — Community distribution

Later only:

- optional remote catalog
- signing/trust metadata
- publisher tooling
- version discovery/update UX

## Testing

Unit/contract coverage must include:

- schema validation
- semantic validation
- migration fixtures
- stable ID/version resolution
- conflicting pack rejection
- port compatibility
- anchor transform parsing
- composite cycle detection
- farm dependency resolution
- browser/server parity where the same operation exists

Browser/E2E coverage must include:

- import valid pack
- reject invalid/malicious archive paths
- render imported primitive and GLB component
- save/reload layout
- export/import layout preserving component identity and transforms
- missing dependency UX
- selecting imported component resolves the correct domain binding

## Non-goals for V0

Do not add these merely to make the plugin system appear general:

- arbitrary JavaScript plugin execution
- arbitrary WebAssembly execution
- user-defined shaders
- a Blender-like/CAD editor
- a required public marketplace
- remote asset hot-linking
- a general semantic ontology engine
- arbitrary protocol logic inside components

The smallest useful component contract for the real pilot is the priority.
