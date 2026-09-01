# GrowNerve Documentation

This directory is the implementation blueprint for GrowNerve. Documents are ordered so a new contributor can move from product intent to architecture, domain design, edge control, UI, safety, deployment, and implementation work without reverse-engineering assumptions from code.

## Reading order

1. [Product vision](01-product-vision.md)
2. [Scope and releases](02-scope-and-releases.md)
3. [System architecture](03-architecture.md)
4. [Domain model](04-domain-model.md)
5. [Data model](05-data-model.md)
6. [API design](06-api-design.md)
7. [Edge and MQTT](07-edge-and-mqtt.md)
8. [Hardware and sensors](08-hardware-and-sensors.md)
9. [Automation and safety](09-automation-and-safety.md)
10. [Grow cycles and recipes](10-grow-cycles-and-recipes.md)
11. [UI and interaction design](11-ui-ux.md)
12. [3D digital twin](12-3d-digital-twin.md)
13. [Security and permissions](13-security-and-permissions.md)
14. [Observability and operations](14-observability-and-operations.md)
15. [Testing and quality](15-testing-and-quality.md)
16. [Deployment and configuration](16-deployment-and-configuration.md)
17. [Implementation roadmap](17-implementation-roadmap.md)
18. [Engineering backlog](18-engineering-backlog.md)
19. [farmOS influences](19-farmos-influences.md)
20. [Architecture decisions](20-architecture-decisions.md)
21. [Browser-only / GitHub Pages runtime](21-browser-only-runtime.md)

## Runtime modes

GrowNerve deliberately supports two modes from one frontend codebase:

```text
server mode   -> Go + PostgreSQL + MQTT + ESP32
browser mode  -> IndexedDB + local simulator/imported data
```

Both expose the same product UI, entity identities, grow-management concepts, history views, alerts, and 3D twin. Browser mode is portable and static-hostable, but it is not considered a safe substitute for unattended physical automation.

## Non-negotiable constraints

- Local operation must survive Internet loss.
- Essential schedules must survive server loss at the edge.
- Automatic nutrient/pH dosing is not part of the first control release.
- WebGPU is the preferred 3D rendering path.
- The 3D scene is an operational digital twin, not decorative content.
- Dangerous commands require server-side safety validation in full mode even when initiated from the 3D UI.
- Browser-only mode must clearly identify simulated/non-authoritative control.
- Telemetry and meaningful farm events are separate data classes.
- Domain services do not depend directly on HTTP, MQTT, SQL, or Three.js.
- Browser persistence uses IndexedDB, not `localStorage`.
- Browser exports are versioned and validated before import.
- Configuration belongs in YAML/environment configuration, not hidden constants in application code.
- The system stays a modular monolith until actual scaling pressure demonstrates a need to split it.

## Reference installation

All V0 decisions should be tested against one real installation: a 3 x 3 ft DWC tent with one LED, circulation fan, air pump, about 30 L of nutrient solution, four plant positions, and ESP32-based telemetry/control. If a feature is not useful there and is not required to keep the architecture extensible, it is probably not V0.
