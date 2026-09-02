# 12 — 3D Digital Twin and Three.js

## Product role

The 3D view is an operational digital twin, not a visualization demo. Interactive objects bind to GrowNerve domain entities and share the same identities used by tables, alerts, history, and commands.

```text
Grow tent        -> Zone UUID
Reservoir        -> Reservoir UUID
Plant            -> PlantPosition UUID
Fan / light      -> Device UUID
Sensor visual    -> Device UUID and bound Channel UUIDs
```

The authoritative operational identity remains:

```text
(entity_type, entity_id)
```

Do not introduce a second component-instance identity for the same operational object.

See `24-component-plugin-system.md` for the declarative component model that will replace hard-coded visual profiles.

## What is implemented now

The current twin is working software and is the migration baseline.

Relevant files:

```text
frontend/src/domain/model.ts
frontend/src/twin/DigitalTwin.tsx
frontend/src/twin/sceneState.ts
frontend/src/runtime/pilotData.ts
```

Current behavior:

- React Three Fiber over Three.js
- WebGPU renderer attempted first, explicit WebGL fallback
- `SceneLayout`/`SceneEntity` stored inside `FarmData`
- scene selection keyed by `(entity_type, entity_id)`
- procedural pilot geometry
- profile-based rendering for `zone`, `reservoir`, `light`, `fan`, and `plant`
- profile-based radial actions
- HTML tooltip overlays
- fan animation, light state, reservoir level, and plant-health visuals
- shared selection between the normal UI and twin

This is intentionally described as implemented. Component packs, component registry persistence, GLB pack import, topology ports, and MCP authoring are **not implemented yet**.

## Migration direction

Do not rewrite the twin from scratch.

The migration is additive:

```text
current SceneEntity
  entity_type
  entity_id
  profile
  position
  scale
        |
        v
add exact component_ref
        |
        v
registry resolves component definition
        |
        v
generic primitive/GLB renderer
        |
        v
remove profile dependency after compatibility period
```

The current procedural geometry becomes the first set of built-in primitive component definitions. This gives a deterministic before/after visual and interaction fixture.

## Rendering architecture

Target flow:

```text
FarmData / current domain state
            |
            +---- SceneLayout / SceneEntity
            |           |
            |           v
            |     exact component_ref
            |           |
            |           v
            |     Component Registry
            |           |
            +-----------+
                    |
                    v
           Scene Render State
                    |
          +---------+---------+
          |                   |
          v                   v
       Three.js            DOM UI
   geometry/materials   tooltip/radial/inspector
```

Three.js must not own farm semantics. Component definitions must not contain MQTT, HTTP, authorization, or physical-safety logic.

## Technology choices

### Three.js and React Three Fiber

React Three Fiber is already the integration layer and should remain unless measured problems justify changing it.

It currently provides:

- normal React composition
- Three.js event/raycast integration
- lifecycle integration with the application
- `@react-three/drei` helpers such as `Html` and `OrbitControls`

Do not introduce a second rendering framework.

### WebGPU first, WebGL fallback

`DigitalTwin.tsx` already attempts `WebGPURenderer` when `navigator.gpu` exists and falls back to `WebGLRenderer` if initialization fails.

Keep scene/domain logic independent of which renderer succeeds.

Do not create WebGPU-only product behavior unless there is a graceful supported fallback or the feature is explicitly optional.

## Scene entity binding

The existing domain model already provides the correct binding:

```ts
interface SceneEntity {
  entity_type: EntityType;
  entity_id: UUID;
  profile: string;
  position: [number, number, number];
  scale: [number, number, number];
}
```

The component migration extends this shape rather than replacing it:

```ts
component_ref
rotation?
configuration?
channel_bindings?
```

`profile` becomes temporary compatibility metadata.

Runtime `Object3D.userData` may cache resolved binding information, but it is an adapter detail. Application selection continues to carry domain identity only.

## Selection and picking

The current `entityKey(entity_type, entity_id)` approach is correct.

```text
3D hit
 -> entity key
 -> shared selection
 -> inspector/action state
```

Inverse navigation should continue to work:

```text
alert/table/search selection
 -> same entity key
 -> scene lookup
 -> highlight/focus
```

Raycasting is adequate for the pilot. Optimize only when measured scene size requires it:

- raycast only interactive objects
- use simple hit proxies for complex GLBs
- use instancing for repeated plants/components
- add GPU picking only after profiling proves raycasting insufficient

## Tooltips and overlays

Keep text UI in the DOM.

Current `Html` tooltips are the right direction because they provide:

- sharp text
- accessible HTML
- normal localization/layout
- simpler controls and confirmations

As the twin matures, tooltip data must show freshness/quality for live measurements and avoid presenting stale telemetry as current.

## Actions and radial menus

Today `sceneState.ts` maps `profile` to action names. That is a compatibility implementation, not the long-term extension point.

Target action resolution:

```text
component capabilities
 + entity type/state
 + bound logical Channels
 + runtime mode
 + authenticated role
 + physical safety policy where command-related
        |
        v
GrowNerve-owned action catalog
        |
        v
radial menu / inspector
```

Community component JSON must not be able to invent privileged actions. It can describe capabilities; GrowNerve decides which workflows exist and whether the current caller may invoke them.

## Dynamic render behavior

A fully generic component file should not mean arbitrary plugin code.

GrowNerve owns a small normalized render-state vocabulary, for example:

```text
selected
online/offline
alert severity
power on/off/pending
output ratio
fill ratio
measurement value/quality/freshness
growth stage
plant health
```

Built-in renderer behaviors consume that state:

```text
fan rotation      <- power/output
reservoir water   <- fill ratio
light emissive    <- power state
plant visual      <- growth/health
warning marker    <- alert severity
```

A declarative component may opt into supported behaviors. It does not ship JavaScript to implement them.

## Models and assets

### Primitive first

The current scene already demonstrates that simple geometry is enough for a useful operational twin.

The first component renderer should support:

```text
box
sphere
cylinder
plane
```

Those definitions should reproduce the existing pilot visuals before GLB support is refactored in.

### GLB second

Use `.glb` as the first authored-model format for component packs.

Requirements:

- real-world meters
- consistent orientation and pivot rules
- local assets only
- no remote model/texture references
- bounded file, triangle, node, texture, and animation budgets
- cache by immutable asset digest

Do not put imported GLB bytes inside the current whole-farm Dexie snapshot. See document 24 for registry/asset storage.

## Plants

Plants use the same component system but their visual state changes over time.

Initial strategy:

- discrete supported growth-stage visuals
- optional bounded scale/canopy interpolation
- health/attention overlay from current domain state

The visual is an operational representation, not a biological prediction.

## Camera behavior

Keep navigation simple for the pilot.

### Explore

Orbit/pan/zoom.

### Focus

Frame a selected entity while preserving understandable orientation.

### Presets — later as needed

Examples:

```text
Tent overview
Reservoir service view
Top-down plant view
Controller/electrical view
```

Avoid free-fly/CAD navigation until a larger facility demonstrates the need.

## Scene editing boundary

Do not build a CAD editor.

Useful future editing is deliberately constrained:

- assign/replace component definition
- move/rotate/scale a scene binding
- place a new domain-backed visual
- bind compatible Channels
- connect supported physical ports
- snap compatible anchors
- save camera preset

Every saved result is explicit data. Do not rely on hidden editor state that must be replayed to reconstruct a layout.

## Command interaction

3D control uses the same command path as the rest of GrowNerve.

```text
select fan
 -> choose Set output
 -> submit command intent
 -> authorization
 -> safety/interlock validation
 -> durable command publication
 -> acknowledgement/state telemetry
 -> visual state changes
```

Do not visually claim that physical equipment changed merely because the request was sent.

Browser mode may acknowledge simulator commands and must keep simulated status clear.

## Performance rules

Performance goals are guidance, not unmeasured guarantees.

Priorities:

- no full scene reconstruction for one telemetry sample
- cache immutable component/model assets
- instance repeated geometry where worthwhile
- avoid high-poly hidden details
- keep dynamic material/state updates local
- dispose uncached Three.js resources explicitly
- measure desktop/tablet reference hardware before setting hard budgets

Do not add LOD systems, GPU compute, Meshopt/Draco, or custom shaders simply because Three.js/WebGPU supports them. Add them when scene measurements justify the complexity.

## Testing strategy

### Existing compatibility tests

Preserve behavior around:

- `entityKey()` identity
- scene-index mapping
- profile-action compatibility during migration
- WebGPU/WebGL startup path
- browser/server selection behavior

### Component migration tests

Add:

- deterministic profile -> component-ref mapping
- same entity key before/after migration
- primitive definition resolves to expected renderer
- missing component dependency fails visibly
- channel bindings reference compatible existing Channels
- capability action resolution cannot grant unauthorized commands

### Browser/E2E

Cover:

- selecting a mesh selects the correct UUID
- alert/table selection focuses the same entity
- tooltip/current-state data is correct
- radial action invokes the expected normal workflow
- unsafe command rejection is visible
- imported valid component renders after registry persistence exists

### Visual regression

Use the deterministic pilot layout and stable camera as the comparison scene before and after the generic-renderer refactor.

Do not rely on pixel-perfect screenshots across WebGPU and WebGL; assert geometry placement, semantic state, and interaction correctness separately.

## Reference scene

The acceptance scene remains the real pilot:

```text
3 x 3 ft tent
240 W LED
circulation fan
30 L-class reservoir
four plant positions
two air-stone positions
air pump
ESP32/controller
sensor markers for air temperature/RH, water temperature, water level
```

The next meaningful 3D milestone is **not** a prettier scene. It is rendering this same scene from validated component definitions while preserving the exact existing domain identities and interaction behavior.