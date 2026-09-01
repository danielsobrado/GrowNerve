# 11 — UI and Interaction Design

## Goal

GrowNerve should feel like an operational control product, not a generic admin dashboard. The primary question is always: what is happening in the farm right now, and what requires attention?

The same UI must work in both runtime modes:

```text
server mode
browser-only mode
```

Screens should depend on frontend application interfaces rather than caring whether data comes from OpenAPI or IndexedDB.

## Main navigation

Suggested top-level areas:

```text
Overview
Farm
Grow Cycles
3D Twin
Alerts
History
Inventory
Automation
Devices
Settings
```

The exact labels can evolve, but operational tasks should stay separate from configuration-heavy screens.

## Runtime identity

The application must always know its runtime mode.

Browser-only mode should show a small, non-intrusive indicator in Settings/About and anywhere physical-control semantics could be misunderstood:

```text
Browser only
Local data
Simulation/manual operation
```

Do not place a permanent warning banner across every screen, but never let simulated commands look like acknowledged real-hardware commands.

## Overview screen

The overview should avoid a wall of KPI cards. Prefer a compact current-state composition:

- active grow cycles and stage/progress
- current zone conditions
- reservoir condition
- active alerts
- equipment state
- today's relevant events
- upcoming tasks such as calibration/harvest/refill

## Shared entity selection

2D and 3D interactions use one client selection model.

Conceptual state:

```text
selectedEntity = {
  type: "reservoir",
  id: "uuid"
}
```

Selecting the same reservoir from:

- search
- alert
- table row
- farm hierarchy
- timeline
- Three.js mesh

must open the same entity inspector and resolve the same resource regardless of runtime mode.

## Entity inspector

A reusable right-side/overlay inspector shows:

```text
name
entity type
current status
key measurements
active grow association
active alerts
recent events
available actions
```

Entity-specific tabs may include:

```text
Overview
History
Configuration
Maintenance
Automation
```

The 3D scene should not reimplement all detail panels inside WebGL/WebGPU.

## Commands

Low-risk manual control can be initiated from:

- entity inspector
- equipment list
- 3D radial menu

Every path calls the same command application service.

In server mode that service reaches the Go command API. In browser-only mode it reaches the local simulator unless a future explicit browser hardware adapter is active.

Dangerous actions need a confirmation surface that clearly states target, requested action, safety consequences, and current conditions. Confirmation should generally be HTML UI, not text floating in the 3D scene.

## Tooltips

Hover/focus tooltip content should be short:

Example sensor:

```text
Reservoir pH
6.03 · Target 5.8–6.2
Good · updated 8 s ago
```

Example fan:

```text
Circulation Fan
47% · 1,220 RPM
Automation · healthy
```

Example plant position:

```text
P3 · Bibb Lettuce
Day 23 · Vegetative
1 open observation
```

Tooltips should not contain command buttons. Commands belong to selection/radial/inspector actions.

## Radial menu

Radial menus are context-specific and intentionally small. Prefer 4–6 primary actions.

Examples:

### Sensor

```text
Inspect
History
Calibrate
Alerts
Configure
```

### Fan

```text
Inspect
Set output
Override
History
Maintenance
```

### Plant

```text
Inspect
Observe
Photo
History
Harvest
```

### Reservoir

```text
Inspect
Chemistry
Add input
Refill
History
```

Unavailable actions are omitted or visibly disabled with a reason.

## Input behavior

### Desktop

```text
hover -> tooltip
left click -> select
second/context action or radial button -> radial menu
Esc -> close radial/deselect as appropriate
```

A direct single-click radial menu may be tested, but selection and navigation must not become frustrating.

### Touch

```text
tap -> select
long press or explicit context control -> radial menu
pinch -> zoom
one-finger drag on empty scene -> orbit/pan depending mode
```

### Keyboard/accessibility

All important 3D actions must also be available through non-3D UI and keyboard navigation. The digital twin cannot become an accessibility gate.

## Search

Global search should find:

- facility/zone
- reservoir
- device
- sensor/channel
- active grow cycle
- plant position
- alert

Selecting a spatial entity can optionally focus the 3D camera on it.

## Alert interaction

Clicking an alert should:

1. select the affected entity
2. open relevant details
3. optionally focus/highlight it in 3D
4. show the condition and history

The user should not need to hunt through the scene to locate a problem.

## Import/export UX

Browser-only mode must make data portability obvious without dominating normal operation.

Settings contains:

```text
Export all data
Import archive
Storage usage
Reset local farm
Load pilot example
```

First-run empty state:

```text
Welcome to GrowNerve

[ Create local farm ]
[ Load pilot example ]
[ Import .grownerve.json ]
```

Export must state whether media is included and show resulting/estimated size where practical.

Import must validate first and present the selected mode clearly:

```text
Replace local data
Merge data (later)
```

For V0, replace is the required safe path. The UI must never clear existing local data before the incoming archive has passed validation.

## Visual language

Status should be consistent across 2D and 3D:

```text
normal
action needed
warning
critical
offline/unknown
manual override
simulated
```

Do not rely on color alone; use icons, labels, patterns, or animation carefully.

## Motion

Use animation to communicate state, not decoration.

Good examples:

- subtle fan rotation when running
- water-level change
- pulsing locator ring for selected alert target
- small flow animation when a pump is active

Avoid constant ambient motion that makes monitoring tiring or wastes GPU/battery.

## Responsive behavior

Desktop can show scene + inspector side by side. Tablets should remain first-class because they are practical farm-control devices. On phones, 3D may become a focused view with bottom-sheet inspection rather than squeezing full desktop panels.

## Offline/disconnected UI

Server mode distinguishes clearly between:

- browser disconnected from server
- server unable to reach device
- device healthy but a particular channel stale

Browser-only mode distinguishes:

- local browser runtime active
- simulator paused
- replay active
- imported historical data
- browser runtime resumed after suspension

Never display an old measurement as if it were live simply because it is the last value in cache.
