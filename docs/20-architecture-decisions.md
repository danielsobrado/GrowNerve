# 20 — Architecture Decisions

This document records initial decisions so implementation does not repeatedly reopen settled questions without new evidence. Decisions can change, but changes should record why.

## ADR-001 — Modular monolith first

**Decision:** one Go server process for HTTP, application services, scheduler, automation evaluation, MQTT integration, and background jobs.

**Why:** the initial farm is small; distributed services add failure modes without solving a current problem.

**Revisit when:** measured scaling, deployment isolation, or team ownership creates a concrete need.

## ADR-002 — PostgreSQL is the full-runtime system of record

**Decision:** use PostgreSQL for configuration, events, commands, projections, and initial telemetry storage in server mode.

**Why:** operational simplicity and strong transactions outweigh premature time-series specialization.

**Revisit when:** real telemetry volume/retention makes PostgreSQL operationally inefficient despite partitioning/rollups.

## ADR-003 — MQTT between server and edge

**Decision:** local Mosquitto + versioned GrowNerve MQTT protocol.

**Why:** simple, proven pub/sub for unreliable/reconnecting edge nodes and command/telemetry separation.

## ADR-004 — Browser never connects directly to MQTT in normal architecture

**Decision:** full-mode browser uses GrowNerve HTTP + live-update API. Browser-only mode uses IndexedDB/local simulator rather than MQTT.

**Why:** centralizes authorization, protocol translation, query state, and security; keeps MQTT credentials off normal browser clients.

## ADR-005 — Local-first control

**Decision:** Internet is optional for essential operation. Edge controllers persist essential schedules and safe states.

**Why:** crop survival cannot depend on external connectivity.

## ADR-006 — Server is command/safety authority; edge has hardware limits

**Decision:** normal physical command intent is validated server-side, while edge firmware independently enforces hardware-local limits and command validity.

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

**Decision:** OpenAPI 3.1 defines server/browser contract and generates TypeScript client/types for server mode.

**Why:** prevents frontend/backend wire-type drift and supports future integrations.

## ADR-019 — YAML for non-secret server configuration

**Decision:** structured server application configuration is YAML with environment/secret overrides.

**Why:** readable deployment configuration and no scattered hardcoding.

## ADR-020 — Manual GitHub Pages browser-only publication

**Decision:** the same frontend can be built in a self-contained browser runtime and manually deployed to GitHub Pages through npm.

**Why:** static hosting becomes a genuinely usable local-data version of GrowNerve rather than a read-only mock.

**Boundary:** unattended real-hardware automation still requires full server/edge mode.

## ADR-021 — farmOS concepts, not farmOS implementation

**Decision:** adopt selected conceptual lessons—events, quantities, adjustment history, hierarchy, durable identities—without copying farmOS source or reproducing its generic Drupal entity architecture.

**Why:** GrowNerve has a narrower controlled-environment domain and different control/visualization requirements.

## ADR-022 — No automatic optimistic success for real actuators

**Decision:** UI may show a command as pending, but applied equipment state changes only after acknowledgement/state telemetry in full mode.

**Why:** visual state must not claim physical action occurred when the device may be offline or rejected it.

Browser simulator acknowledgements are explicitly identified as simulated.

## ADR-023 — Simulator is first-class

**Decision:** build deterministic server and browser simulators before real hardware integration.

**Why:** protocol/backend/UI development, browser-only use, and CI should not depend on physical devices being connected.

## ADR-024 — Browser-only mode is a first-class runtime

**Decision:** support `server` and `browser` frontend runtime modes behind typed application/repository interfaces.

**Why:** the Pages build must preserve the real application experience and not fork into a separate demo UI.

## ADR-025 — IndexedDB is the browser system of record

**Decision:** browser-only domain data, telemetry, events, and media are persisted in IndexedDB.

**Why:** it is designed for structured/large local browser data and transactions; `localStorage` is not.

## ADR-026 — Portable archive is part of the product contract

**Decision:** browser-only mode exports/imports a versioned `.grownerve.json` format using stable UUIDs.

**Why:** users must be able to back up, move, inspect, and later migrate local farm data into the full runtime.

## ADR-027 — Import validates before destructive writes

**Decision:** browser archive import performs syntax/schema/referential/domain validation before replacing local state and writes replacement data transactionally.

**Why:** a malformed backup must not destroy a valid local farm.

## ADR-028 — Browser/server behavioral parity is tested

**Decision:** shared fixtures/contract tests exercise equivalent use cases against server and browser adapters where functionality overlaps.

**Why:** two runtime implementations can otherwise drift even if they share UI components.

## ADR-029 — Browser-only automation is not unattended physical automation

**Decision:** browser mode may author/evaluate rules, replay telemetry, and exercise commands against the simulator, but it does not claim reliable background hardware control.

**Why:** browsers can throttle/suspend background execution and require interactive permissions for hardware APIs.

## ADR-030 — Browser Pages build is a PWA

**Decision:** cache the static application shell and versioned 3D assets for offline reopening while keeping mutable farm data in IndexedDB.

**Why:** this makes the static deployment genuinely local-first without mixing service-worker cache with application records.

## ADR-031 — Farm state is compare-and-swap, not read-compare-write

**Decision:** the farm document carries a version, `Save` is a conditional
update on that version, and the ETag a client echoes in `If-Match` is that same
version. Every read-modify-write goes through `farm.Mutate`, which retries with
jittered backoff on conflict.

**Why:** the previous design loaded the state, compared a content hash, and then
saved — three separate steps with no atomicity. Under fifty concurrent
ETag-guarded writers, six survived and forty-four were silently lost while their
clients were told they had succeeded. Optimistic concurrency that is not atomic
is not concurrency control. The jitter matters as much as the retry: writers
that collide once collide again if they retry in lockstep.

## ADR-032 — Telemetry is relational; configuration stays a document

**Decision:** measurements are stored append-only in their own tables with a
latest-value projection. Facilities, zones, devices, channels, recipes, alerts,
and commands remain one versioned JSON document.

**Why:** the two data classes have opposite shapes. Telemetry is high-volume,
append-only, and queried by time range; rewriting the whole farm on every sample
made an operator's edit race a sensor on every save and put unbounded data in a
document that has to be read whole. Configuration is low-volume, edited by
people, and benefits from whole-document consistency — splitting it across
forty-odd tables would buy nothing at the documented V0 scale and cost a large
mapping layer.

The compatibility projection keeps the contract intact: `GET /api/v1/state`
merges a bounded recent window of measurements into the document, and
`PUT /api/v1/state` strips them back out, adopting any submitted history into
the measurement store so a browser-to-server migration does not lose it.

This is a deliberate boundary, not a permanent one. Per-resource endpoints can
replace whole-document writes without changing the UI contract when editing
concurrency becomes the constraint.

## ADR-033 — Live updates carry hints, never data

**Decision:** the server-sent-event stream emits `{"topic":"..."}` envelopes. A
client re-reads through the normal authorized endpoints.

**Why:** pushing farm data down the stream would bypass the authorization
applied to those endpoints and duplicate the projection logic in a second place.
A hint is also cheap enough to broadcast to every subscriber without regard for
what each is permitted to see.

## ADR-034 — Authorization and safety are separate checks

**Decision:** a command passes an authorization check based on the caller's
role, then a safety check based on channel limits, device liveness, expiry, and
interlocks. Neither substitutes for the other, and an administrator cannot
override an interlock.

**Why:** they answer different questions. "May this person ask for this?" is a
policy decision that changes with staffing. "Is this safe to do right now?" is a
physical one that does not care who is asking.

## ADR-035 — Accepted commands are queued, never dropped

**Decision:** a command is persisted before publication. If the broker is
unreachable at that moment, the message goes to the outbox and a worker retries
it with a bounded attempt count.

**Why:** the operator has already been told the command was accepted. Losing it
because a broker restarted turns a transient fault into a silent one. The
attempt bound exists so a permanently bad message parks rather than blocking
everything behind it.

## ADR-036 — The edge precedence engine exists twice, deliberately

**Decision:** `internal/edge` in Go and `firmware/esp32/include/edge_policy.h`
in C++ implement the same resolution order. The Go copy is exercised in CI; the
C++ copy drives the relays.

**Why:** the alternative is a controller whose precedence logic is only ever
tested on a bench. Keeping an executable copy in the server repository is what
lets the server-loss behaviour, override expiry, emergency latching, and
midnight-crossing photoperiods be asserted automatically. The duplication is
real and is called out in both files: they must be changed together.

## ADR-037 — Media is typed by content, not by filename

**Decision:** uploads are accepted only if their sniffed content type is in an
image allow-list, are stored under a generated key, and are served as
attachments with sniffing disabled.

**Why:** a filename and a declared content type are both attacker-controlled.
HTML and SVG can carry script, and this store's contents are served back to
browsers, so the check has to be on the bytes.
