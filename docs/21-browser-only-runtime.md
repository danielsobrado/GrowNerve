# 21 — Browser-Only / GitHub Pages Runtime

## Goal

GrowNerve supports an optional **browser-only runtime** that can be built as static files and published to GitHub Pages.

This mode must preserve the same product surfaces and domain concepts as the normal server-backed application:

- facility and zone management
- reservoirs
- crops and varieties
- grow cycles
- recipes, stages, and setpoints
- devices and logical channels
- telemetry history
- observations and photos
- farm events and quantities
- inventory
- harvests
- alerts
- automation-rule authoring/evaluation
- command history
- 2D dashboards
- Three.js/WebGPU digital twin
- tooltips, inspectors, and radial menus
- search and history
- import/export

The difference is the runtime adapter, not the UI.

## Important safety boundary

A static browser application cannot provide the same unattended real-hardware guarantees as the local server + MQTT + ESP32 deployment.

Browsers may suspend tabs, throttle timers, close background work, lose local-device sessions, or require user interaction for hardware APIs. GitHub Pages also cannot host PostgreSQL, Mosquitto, or the Go control service.

Therefore browser-only mode provides **functional UI/domain parity**, but not unattended physical-control parity.

In browser-only mode:

- manual/domain actions work locally
- telemetry can be entered, imported, replayed, or simulated
- automation rules can evaluate while the application is active
- commands can target the built-in simulator
- optional experimental browser hardware adapters may be added later
- the UI must never imply that an unattended physical action is guaranteed

Production farm automation remains the server/edge mode.

## Runtime modes

The frontend supports two explicit modes:

```text
server
browser
```

### `server`

```text
React UI
   -> HTTP/live API
   -> Go application services
   -> PostgreSQL
   -> MQTT
   -> ESP32
```

### `browser`

```text
React UI
   -> browser application services
   -> IndexedDB
   -> local simulator / imported telemetry

Three.js/WebGPU uses the same entity IDs and state model.
```

The mode is selected at build/runtime configuration and must be visibly identifiable in Settings/About.

## Frontend architecture

UI components must not branch throughout the codebase on `if (browserMode)`.

Use small application ports/interfaces such as:

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
ExportService
ImportService
```

Implementations:

```text
Server adapters
  -> generated OpenAPI client / live API

Browser adapters
  -> IndexedDB / local runtime
```

Views, Three.js scene bindings, inspectors, radial menus, charts, and workflows depend on these interfaces rather than persistence details.

## Domain parity

Where a browser-side use case duplicates a server-side invariant, it must be covered by **shared behavioral fixtures/contract tests**.

Examples:

```text
cannot start a grow with invalid recipe version
published recipe versions are immutable
inventory adjustments are append-only
invalid units are rejected
harvest cannot precede grow start
alert lifecycle follows allowed transitions
logical channel identity remains stable
```

The implementation language may differ, but observable behavior must match.

Do not copy random handler logic into React components.

## Local persistence

Use **IndexedDB** for farm data.

Do not use `localStorage` for domain records or telemetry.

`localStorage` is acceptable only for small preferences such as:

- theme
- last route
- camera preference
- UI density

Suggested IndexedDB logical stores:

```text
metadata
facilities
zones
reservoirs
crops
varieties
growCycles
plantPositions
recipes
recipeVersions
recipeStages
setpoints
devices
channels
channelBindings
measurements
events
eventQuantities
observations
inventoryItems
inventoryAdjustments
harvests
alerts
commands
automationRules
sceneLayouts
media
```

The exact physical IndexedDB schema can evolve, but the browser runtime must expose typed repositories rather than leaking IndexedDB APIs into UI code.

## Transactions

Multi-record workflows must use IndexedDB transactions.

Examples:

- importing a complete archive
- creating an event and its quantities
- recording an inventory adjustment plus related event
- deleting/replacing a local farm during import

A failed import must leave the previous local database intact.

## Portable JSON archive

Browser-only mode must support a complete portable JSON export.

Recommended extension:

```text
.grownerve.json
```

Top-level format:

```json
{
  "format": "grownerve",
  "schema_version": 1,
  "exported_at": "2026-09-01T12:00:00Z",
  "app_version": "0.1.0",
  "export_id": "uuid",
  "data": {
    "facilities": [],
    "zones": [],
    "reservoirs": [],
    "crops": [],
    "varieties": [],
    "grow_cycles": [],
    "plant_positions": [],
    "recipes": [],
    "recipe_versions": [],
    "recipe_stages": [],
    "setpoints": [],
    "devices": [],
    "channels": [],
    "channel_bindings": [],
    "measurements": [],
    "events": [],
    "event_quantities": [],
    "observations": [],
    "inventory_items": [],
    "inventory_adjustments": [],
    "harvests": [],
    "alerts": [],
    "commands": [],
    "automation_rules": [],
    "scene_layouts": []
  },
  "media": []
}
```

All persistent entity IDs remain UUIDs so archives can later be imported into the full server deployment.

## Media in JSON

For a true all-data export, media can be included as base64 in the JSON archive:

```json
{
  "id": "uuid",
  "mime_type": "image/jpeg",
  "sha256": "...",
  "filename": "plant-p3-day-23.jpg",
  "data_base64": "..."
}
```

This can make JSON files large. The UI must show the estimated/exported size and must never silently omit media.

A future compressed archive format may be added, but plain JSON remains the portable baseline.

## Import modes

Import presents two explicit choices.

### Replace

- validate complete archive first
- create a backup/export option before replacement
- clear local GrowNerve data only after validation succeeds
- write the imported dataset in a transaction

### Merge

- match by stable UUID
- add missing entities
- identical existing records are accepted
- conflicting records are shown to the user
- never silently overwrite conflicting history

For V0, **Replace** is required. Merge can follow after conflict semantics are tested thoroughly.

## Import validation

Validate before modifying local state:

- JSON syntax
- `format == grownerve`
- supported `schema_version`
- required fields
- UUID validity
- referential integrity
- unit compatibility
- enum/state validity
- timestamp validity
- duplicate IDs
- media hash where present
- basic domain invariants

Unknown future schema versions fail with a clear message rather than attempting a partial import.

## Archive migrations

The importer owns schema migration.

Example:

```text
v1 archive
  -> migrate v1 to v2 in memory
  -> validate v2
  -> persist
```

Never mutate the source file.

Migration functions should be deterministic and tested with golden fixture archives.

## Export determinism

For testability, arrays should be emitted in deterministic order and object keys should follow a stable serializer strategy where practical.

`exported_at` and `export_id` are expected to vary; the underlying data should not reorder randomly between exports.

## Browser simulator

Browser mode ships with the same deterministic pilot simulator used for screenshots/tests:

```text
one facility
one 3 x 3 tent
one active lettuce grow
one reservoir
four plant positions
LED
fan
air pump
air temperature
relative humidity
water temperature
water level
one warning scenario
```

The simulator supports:

- live-value generation while the app is open
- accelerated replay
- device offline state
- stale channel state
- alert scenarios
- command acknowledgement/rejection

Simulated state must be labelled as simulated.

## Optional telemetry replay

Imported measurement history may be replayed through the browser runtime to test:

- charts
- alerts
- 3D visual states
- rule evaluation

Replay speed can be adjustable without changing original timestamps in the archive.

## Automation in browser mode

Rules may be authored, validated, simulated, and evaluated locally.

Modes:

```text
observe
simulate
```

Browser-only mode must not advertise reliable unattended physical execution.

If a browser tab is backgrounded, timer throttling may occur. The UI should display a runtime status such as:

```text
Browser runtime active
Browser runtime suspended/restarted
```

## Commands in browser mode

The command workflow remains present so the UI has parity:

```text
requested -> validated -> pending -> applied/rejected/timed_out
```

The default command target is the local device simulator.

This lets radial menus, command history, confirmations, and status visualization behave exactly like the server-backed UI without claiming real equipment changed.

## Optional browser hardware experiments

Later, narrowly scoped adapters may investigate:

- Web Serial
- Web Bluetooth
- WebUSB

These are commissioning/manual-session features only until proven otherwise.

They must not weaken the rule that unattended crop-critical control belongs on the local server/edge runtime.

Direct browser MQTT remains out of scope for normal architecture.

## 3D digital twin

The browser-only build must include the complete Three.js/WebGPU twin:

- GLB assets
- scene entity bindings
- hover tooltips
- selection
- entity inspector
- radial menus
- live/simulated sensor state
- reservoir fill level
- fan/LED animations
- plant growth stages
- alert highlighting

All 3D state is resolved from the same browser repositories used by 2D views.

## PWA/offline behavior

The GitHub Pages build should be installable/cacheable as a PWA.

Cache:

- application shell
- JS/CSS bundles
- icons/fonts used by the app
- GLB models
- textures required by the pilot scene

Do not cache mutable farm data in the service-worker cache; it belongs in IndexedDB.

After the first successful load, the application should be able to reopen without network access when the browser/platform supports the configured PWA behavior.

## GitHub Pages routing

Project Pages runs below:

```text
/GrowNerve/
```

All asset URLs must respect the configured Vite base path.

The browser-only Pages build should use routing that works without server rewrites. Hash-based routing is acceptable for the static build if required:

```text
/GrowNerve/#/grow-cycles/...
```

Server mode may use normal history routing.

## Build configuration

Conceptual frontend configuration:

```text
VITE_RUNTIME_MODE=browser
VITE_BASE_PATH=/GrowNerve/
```

Recommended npm scripts:

```text
npm run dev
npm run dev:browser
npm run build
npm run build:browser
npm run test
npm run deploy:pages
```

`deploy:pages` remains manual.

## First-run experience

When no IndexedDB data exists, show:

```text
Welcome to GrowNerve

[ Create local farm ]
[ Load pilot example ]
[ Import .grownerve.json ]
```

Do not silently seed fake data into a user's new local farm.

## Settings UI

Browser mode settings include:

```text
Runtime: Browser only
Storage: IndexedDB
Export all data
Import archive
Reset local farm
Load pilot example
Storage usage
Application version
Archive schema version
```

Destructive reset requires explicit confirmation.

## Moving from browser mode to server mode

Migration path:

```text
GitHub Pages / browser-only
  -> Export .grownerve.json
  -> Install full GrowNerve server
  -> Import archive through server import workflow
  -> Continue with PostgreSQL/MQTT/ESP32
```

The portable archive is therefore part of the product contract, not merely a backup convenience.

## Definition of done

Browser-only mode is complete when:

- the GitHub Pages build starts with no backend
- all normal non-hardware workflows are usable
- 2D and 3D views use the same local entities
- data survives browser reload/restart through IndexedDB
- full JSON export can restore a fresh browser database
- import validates before modifying data
- media can be exported/restored
- pilot simulation supports telemetry, alerts, and command UI
- routes/assets work under `/GrowNerve/`
- the PWA can reopen offline after initial cache where supported
- server and browser adapters pass shared behavior/contract tests
- the UI clearly distinguishes simulated/browser-only operation from live hardware control
