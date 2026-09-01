# 20 — Architecture Decisions

This document records initial decisions so implementation does not repeatedly reopen settled questions without new evidence. Decisions can change, but changes should record why.

## ADR-001 — Modular monolith first

**Decision:** one Go server process for HTTP, application services, scheduler, automation evaluation, MQTT integration, and background jobs.

**Why:** the initial farm is small; distributed services add failure modes without solving a current problem.

**Revisit when:** measured scaling, deployment isolation, or team ownership creates a concrete need.

## ADR-002 — PostgreSQL is the system of record

**Decision:** use PostgreSQL for configuration, events, commands, projections, and initial telemetry storage.

**Why:** operational simplicity and strong transactions outweigh premature time-series specialization.

**Revisit when:** real telemetry volume/retention makes PostgreSQL operationally inefficient despite partitioning/rollups.

## ADR-003 — MQTT between server and edge

**Decision:** local Mosquitto + versioned GrowNerve MQTT protocol.

**Why:** simple, proven pub/sub for unreliable/reconnecting edge nodes and command/telemetry separation.

## ADR-004 — Browser never connects directly to MQTT

**Decision:** browser uses GrowNerve HTTP + live-update API.

**Why:** centralizes authorization, protocol translation, query state, and security; keeps MQTT credentials off normal browser clients.

## ADR-005 — Local-first control

**Decision:** Internet is optional for essential operation. Edge controllers persist essential schedules and safe states.

**Why:** crop survival cannot depend on external connectivity.

## ADR-006 — Server is command/safety authority; edge has hardware limits

**Decision:** normal command intent is validated server-side, while edge firmware independently enforces hardware-local limits and command validity.

**Why:** defense in depth without putting full farm/business logic on ESP32 nodes.

## ADR-007 — Telemetry and farm events are separate

**Decision:** high-volume sensor measurements use dedicated measurement storage. Meaningful actions use FarmEvent/EventQuantity.

**Why:** different semantics, retention, volume, and query patterns.

## ADR-008 — Logical channels outlive physical devices

**Decision:** `DeviceChannel`/logical channel identity is separate from current physical sensor binding.

**Why:** probe/controller replacement must not fragment farm history or scene identity.

## ADR-009 — GrowCycle is the central agricultural context

**Decision:** production history organizes around GrowCycle rather than generic asset collections.

**Why:** controlled-environment users need a direct answer to what was grown, under what recipe/conditions, and with what result.

## ADR-010 — Recipes are versioned and published versions immutable

**Decision:** a historical grow references the exact recipe version it used.

**Why:** reproducibility and trustworthy comparison.

## ADR-011 — Human-assisted chemistry before automatic chemistry

**Decision:** pH/EC monitoring and assisted dosing precede closed-loop chemical dosing.

**Why:** sensor calibration, pump calibration, mixing dynamics, and safety need real validation.

## ADR-012 — WebGPU first for 3D

**Decision:** Three.js rendering targets WebGPU first, with an explicit supported fallback strategy.

**Why:** matches project direction and provides a path to efficient richer visualization/compute while retaining Three.js ecosystem benefits.

## ADR-013 — 3D is a domain-bound digital twin

**Decision:** interactive scene objects bind to real entity UUIDs. 3D selection uses the same client selection state as tables/search/alerts.

**Why:** prevents the 3D feature from becoming a disconnected demo.

## ADR-014 — DOM overlays for tooltips and radial menus

**Decision:** Three.js provides spatial anchoring/picking; tooltips, radial menus, forms, and confirmations render in accessible HTML overlays.

**Why:** sharper text, normal interaction, accessibility, responsiveness, and simpler forms.

## ADR-015 — Raycasting before GPU picking

**Decision:** use Three.js raycasting against an interaction layer/proxies for V0.

**Why:** scene size is small and raycasting is simpler. Optimize only after measurement.

## ADR-016 — GLB runtime asset format

**Decision:** use glTF/GLB for 3D runtime assets with compression/instancing/LOD as needed.

**Why:** strong web tooling and Three.js support.

## ADR-017 — Do not build a CAD editor in V0

**Decision:** layouts are seeded/configured; runtime focuses on inspection and control.

**Why:** a general scene editor is large scope with little value for the first installation.

## ADR-018 — Strong API boundary

**Decision:** OpenAPI 3.1 defines server/browser contract and generates TypeScript client/types.

**Why:** prevents frontend/backend wire-type drift and supports future integrations.

## ADR-019 — YAML for non-secret configuration

**Decision:** structured application configuration is YAML with environment/secret overrides.

**Why:** readable deployment configuration and no scattered hardcoding.

## ADR-020 — Manual GitHub Pages demo publication

**Decision:** static frontend/demo mode can be manually deployed to GitHub Pages through npm. Real control remains a local/server-backed capability.

**Why:** easy public demonstration without pretending a static site is the farm-control runtime.

## ADR-021 — farmOS concepts, not farmOS implementation

**Decision:** adopt selected conceptual lessons—events, quantities, adjustment history, hierarchy, durable identities—without copying farmOS source or reproducing its generic Drupal entity architecture.

**Why:** GrowNerve has a narrower controlled-environment domain and different control/visualization requirements.

## ADR-022 — No automatic optimistic success for actuators

**Decision:** UI may show a command as pending, but applied equipment state changes only after acknowledgement/state telemetry.

**Why:** visual state must not claim physical action occurred when the device may be offline or rejected it.

## ADR-023 — Simulator is first-class

**Decision:** build a device simulator before real hardware integration.

**Why:** protocol/backend/UI development and CI should not depend on physical devices being connected.
