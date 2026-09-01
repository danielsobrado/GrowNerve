# 12 — 3D Digital Twin and Three.js Plan

## Product role

The 3D view is an operational digital twin of the farm. It is not a separate visualization demo. Every interactive object maps to a real GrowNerve domain entity or stable scene structure.

Examples:

```text
Grow tent mesh       -> Zone UUID
Reservoir mesh       -> Reservoir UUID
Plant mesh           -> PlantPosition UUID
Fan mesh             -> Device UUID
pH probe marker      -> Device/Channel UUID
LED fixture          -> Device UUID
ESP32 enclosure      -> Device UUID
Leak sensor marker   -> Device/Channel UUID
```

## Technology direction

### Renderer

Use Three.js with a WebGPU-first renderer. The application should detect support and maintain an explicit fallback strategy where required by target browsers/hardware.

Do not build two feature-divergent renderers. Scene/data/interaction logic must be renderer-agnostic as far as practical.

### React integration

The UI is React/TypeScript. Either direct Three.js integration or React Three Fiber may be used after a spike, but the decision must preserve:

- WebGPU-first rendering
- direct access to Three.js capabilities
- predictable resource lifecycle
- good integration with shared React selection/query state
- performance with repeated plant assets

The domain model must not depend on the chosen React wrapper.

### Asset format

Prefer glTF/GLB for runtime models.

Pipeline targets:

- consistent real-world units
- origin/pivot conventions
- Meshopt/Draco compression where beneficial
- compressed textures where supported
- LODs for larger scenes
- instancing for repeated plants/components

Store source authoring assets separately from optimized runtime GLBs when those assets exist.

## Scene graph

A facility scene may be structured conceptually as:

```text
FacilityRoot
  Room/Tent zone
    structure
    light
    fan
    reservoir
      water visual
      probes
      air stones
    controller
    plant positions
      P1 plant
      P2 plant
      P3 plant
      P4 plant
```

Structural meshes without a domain identity remain scene-only nodes. Operational meshes carry an entity binding.

## Scene entity binding

Use one metadata abstraction:

```ts
interface SceneEntityBinding {
  entityType: EntityType;
  entityId: string;
  interactionProfile: string;
}
```

Runtime objects may expose this through `Object3D.userData`, but `userData` is an adapter detail; application selection state receives only the typed binding.

## Selection architecture

There is one shared selection store for 2D and 3D.

```text
Three.js hit
  -> resolve SceneEntityBinding
  -> set selected entity
  -> inspector/query layer fetches entity
  -> highlight selection
```

The inverse also works:

```text
Alert/table/search selects entity
  -> shared selection state
  -> scene index resolves entity binding
  -> highlight/focus camera
```

Maintain an in-memory scene index:

```text
(entityType, entityId) -> Object3D / instance reference
```

## Picking

For the initial scene size, Three.js raycasting is sufficient and simpler than GPU picking.

Optimize with:

- interaction layers
- bounding proxies for complex meshes
- only raycast interactive objects
- instanced-mesh `instanceId` mapping for repeated plant positions

Move to GPU picking only after measured raycast performance becomes a problem.

## Hover tooltips

Tooltips are HTML overlays, not 3D text.

Flow:

```text
pointer move
 -> raycast throttled to frame/interval
 -> resolve entity
 -> read cached current-state summary
 -> project world anchor to screen
 -> render HTML tooltip
```

Benefits:

- sharp text
- accessibility
- normal layout/styling
- easy localization
- no texture atlas management

Tooltip anchoring must:

- clamp to viewport
- hide when target is occluded if practical
- avoid flicker when crossing child meshes
- respect touch devices where hover does not exist

## Radial menus

Radial menus should be HTML overlays anchored to the selected object's projected screen position. They are visually connected to the 3D entity while retaining normal DOM interaction/accessibility.

Why not render the entire menu in 3D:

- text/input quality is worse
- accessibility is harder
- responsive layout is harder
- command confirmation becomes awkward

### Interaction profiles

The server/client defines profile-to-action mapping, for example:

```text
sensor:
  inspect
  history
  calibrate
  alerts
  configure

plant:
  inspect
  observe
  photo
  history
  harvest

fan:
  inspect
  set_output
  override
  history
  maintenance

reservoir:
  inspect
  chemistry
  add_input
  refill
  history
```

The client filters actions using capability, state, and permissions. Server authorization/safety remains authoritative.

## Command interaction

Example fan control:

```text
select fan
 -> radial menu
 -> Set output
 -> small DOM control opens
 -> choose 55%
 -> POST command intent
 -> server safety validation
 -> MQTT command
 -> ack
 -> scene animation/state updates from acknowledged state telemetry
```

Never optimistically animate a hazardous actuator as successfully changed before the system receives accepted/applied state.

## Camera behavior

Modes:

### Explore

Orbit/pan/zoom around the facility.

### Focus

Smoothly frame a selected entity while maintaining user orientation.

### Zone preset

Named views such as:

```text
Tent overview
Reservoir service view
Top-down plant view
Controller/electrical view
```

Avoid free-fly controls in V0; they increase navigation complexity without helping a small farm.

## Scene editing

Do not build a full CAD editor in V0.

Initial approach:

- scene/layout authored/configured through controlled JSON/YAML/seed data or a simple admin form
- runtime UI supports inspection, not arbitrary mesh editing

Later editor capabilities may include:

- place device
- move sensor marker
- assign mesh to entity
- save camera preset

## Asset library

Create reusable GLB assets for common components:

```text
grow tent
rack/shelf
DWC tote/reservoir
net pot
lettuce growth stages
LED panel
circulation fan
air pump
air stone
ESP32 enclosure
pH probe
EC probe
temperature probe
level sensor
leak sensor
dosing pump
valve
camera
```

Models should be recognizable, lightweight, and dimensionally plausible rather than photorealistically heavy.

## Plant visualization

Plants need a different strategy than equipment because appearance changes over time.

V0 options:

- discrete growth-stage models
- scale/canopy interpolation between stage representations

A plant position's visual stage can derive from grow-cycle stage/day but must not pretend to be an exact biological simulation.

Later, actual camera/computer-vision estimates may influence the displayed canopy size.

## Live status visualization

Use subtle, semantic effects:

### Selected

outline/ring/halo

### Warning

small status marker or emissive accent

### Offline

desaturated/status marker

### Fan running

actual blade rotation at visually bounded speed

### Light on

fixture emissive state plus limited scene-light change

### Reservoir level

water plane height based on valid level data

### Pump/flow

small flow indicator while active

Avoid flashing, excessive bloom, constant particles, or unrealistic effects.

## WebGPU-specific opportunities

Once baseline rendering works, WebGPU can support richer local visualization efficiently:

- GPU-driven particle/flow visualization
- many repeated plant instances
- node/TSL materials for health/status effects
- compute-assisted visualization where it provides measurable value
- local image/vision workloads elsewhere in the app where supported

Do not add compute shaders merely because WebGPU exists.

## Performance budgets

Define budgets early for target tablet/laptop hardware.

Initial goals:

- interactive 60 fps on a modern desktop for reference scene
- graceful ~30+ fps on supported tablets
- scene remains responsive while live telemetry updates
- no full scene rebuild on measurement updates

Rules:

- telemetry updates modify small visual state objects/material uniforms, not React-remount the scene
- dispose geometries/materials/textures explicitly
- cache GLB assets
- use instancing for repeated components
- avoid high-poly hidden interior geometry
- use LOD for commercial-scale scenes

## 3D state adapter

Do not pass raw API responses throughout scene components. Build a small scene view-model layer that converts current domain state into render state:

```text
DeviceState -> running/offline/output percent
ChannelState -> value/quality/target status
PlantPositionState -> occupied/stage/health marker
ReservoirState -> fill ratio/chemistry status
AlertState -> severity marker
```

This keeps Three.js rendering independent from API response shape.

## Tooltips and data freshness

Every live tooltip must show freshness or quality where relevant. Never display cached values as current without stale indication.

## 3D test strategy

Unit test:

- entity-to-scene mapping
- interaction-profile resolution
- render-state adapters
- radial-action capability filtering

Browser/E2E test:

- selecting mesh selects correct UUID
- 2D selection focuses correct 3D entity
- tooltip displays expected live data
- radial menu action calls correct command workflow
- unsafe command rejection is displayed
- scene works with WebGPU and supported fallback path

Visual regression:

Use a small deterministic reference scene and stable camera poses for screenshot comparison. Avoid relying only on screenshot tests for interaction correctness.

## V0 reference scene

The first 3D deliverable should represent exactly the real pilot system:

```text
3 x 3 tent shell
240 W LED
fan
30 L-class reservoir
four net pots/plants
two air-stone positions
air pump outside/adjacent
ESP32/controller
sensor markers for air temp/RH, water temp, water level
```

This scene becomes both the product demo and the operational UI for the actual installation.
