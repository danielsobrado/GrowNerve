# 22 — Implementation Status

## Delivered software baseline

The repository now contains the executable V0 software baseline described by phases 0–7 of the roadmap:

- one Go modular monolith with health/version endpoints, structured middleware, OpenAPI types, PostgreSQL persistence, optimistic concurrency, durable commands, and MQTT transport
- forward/down PostgreSQL migrations and sqlc-generated access code for the documented domain
- MQTT protocol-v1 telemetry, command, acknowledgement, and health validation
- a deterministic MQTT device simulator and an explicit edge precedence engine
- one React application with server and browser adapters
- IndexedDB persistence, versioned validated archives, transactional replace import, deterministic pilot data, and PWA/GitHub Pages builds
- operational overview, farm topology, grows/recipes/observations, history, alerts, inventory, automation, device/command, settings, and responsive views
- a selectable digital twin that initializes WebGPU first and falls back to WebGL, with shared entity identity, inspector, radial actions, equipment state, water level, and plant state
- unit/contract coverage above the documented 80% threshold and Playwright desktop/mobile operational journeys
- local Docker Compose services, production-oriented images, CI quality/security/generated-code gates, and operator runbooks

## Capability matrix

| Roadmap area | Browser runtime | Full server runtime | Current boundary |
|---|---|---|---|
| Facilities, zones, reservoirs, crops, grows, recipes | Implemented | Persisted through compatibility state/API and schema | Fine-grained CRUD endpoints can replace the compatibility state without changing the UI contract |
| Telemetry/history | IndexedDB + simulator | MQTT validation + PostgreSQL state persistence | High-volume batching/retention tuning is a production hardening item |
| Events, observations, inventory, harvest | Implemented | Schema/API state persistence | Media objects are portable metadata; binary upload storage remains gated by a selected storage policy |
| Alerts and rules | Local active-session evaluation/simulation | Persisted model | A continuously running server evaluator remains required before unattended alerting |
| Digital twin | Implemented | Same UI and identities | Procedural pilot geometry is used until approved GLB assets are produced |
| Low-risk commands | Acknowledged simulation | Durable-before-publish MQTT flow with safety limits | Real relay/PWM application requires commissioned hardware |
| Essential schedules | Author/simulate + tested precedence | Edge precedence model | Persistence on real ESP32 hardware and server-loss proof are not software-only claims |
| Chemistry | Typed channels/setpoints/history-ready schema | Schema-ready | Real probes, calibration evidence, and drift policy are required |
| Automatic dosing | Intentionally unavailable | Intentionally unavailable | Prohibited until every entry criterion in `02-scope-and-releases.md` is evidenced |

## Deliberate external gates

Phases 8–12 include outcomes that cannot be truthfully completed by repository code alone. They require selected electrical hardware, sensor datasheets, wiring and enclosure review, device credentials, calibrated probes, a real ESP32 firmware target, failure-injection tests, and at least one observed pilot grow. GrowNerve therefore fails closed: no repository default can energize real equipment or perform nutrient/pH dosing.

Production identity is also deployment-specific. The current V0 keeps browser operation local and binds server services to loopback by default. Before any non-trusted-LAN exposure, integrate the chosen OIDC/local identity provider at the HTTP authorization boundary, configure TLS and per-device broker credentials/ACLs, and complete the threat review in `13-security-and-permissions.md`.

## Definition of "implemented"

Software marked implemented has source, automated tests, and a documented runnable path. Hardware-dependent items are marked gated until physical evidence exists; simulator success is never presented as commissioning evidence.
