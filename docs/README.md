# GrowNerve Documentation

This directory is the implementation blueprint for GrowNerve. Documents are ordered so a new contributor can move from product intent to architecture, domain design, edge control, UI, safety, deployment, implementation status, future extensibility, and optional monetization without reverse-engineering assumptions from code.

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
22. [Implementation status and external gates](22-implementation-status.md)
23. [Development, operations, and commissioning](23-development-and-operations.md)
24. [Component and plugin system](24-component-plugin-system.md)
25. [MCP component authoring and farm editing](25-mcp-component-authoring.md)
26. [Component taxonomy, capabilities, and information surfaces](26-component-taxonomy-and-capabilities.md)
27. [Commerce catalog, compatibility, and referral service](27-commerce-catalog-and-referral-service.md)

The ESP32 controller lives in [`firmware/esp32`](../firmware/esp32/README.md).

## Status versus design documents

`22-implementation-status.md` is authoritative for what the repository actually implements today.

The component registry, third-party component packs, GLB pack storage/import, archive v2, ports/anchors, richer component taxonomy, MCP server, and commerce/referral service described in documents 24–27 are planned work. The current digital twin already works using procedural geometry and profile-based rendering; document 24 describes an incremental migration of that code rather than a parallel replacement application.

Document 26 is deliberately broader than the first implementation. It defines the target categories, subtypes, capability families, information surfaces, state vocabulary, physical ports/anchors, and component families so later additions remain coherent. Its implementation order explicitly keeps the first schema small and pilot-driven.

Document 27 defines an optional monetization boundary. The farm/control runtime does not depend on it. Product catalogs, merchant APIs, affiliate credentials, referral tags, sponsorship metadata, regional offers, and click/conversion attribution belong in a separate commerce service with its own database and deployment.

## Runtime modes

GrowNerve deliberately supports two modes from one frontend codebase:

```text
server mode   -> Go + PostgreSQL + MQTT + ESP32
browser mode  -> IndexedDB + local simulator/imported data
```

Both expose the same product UI, entity identities, grow-management concepts, history views, alerts, and 3D twin. Browser mode is portable and static-hostable, but it is not considered a safe substitute for unattended physical automation.

An optional commerce client may call the separate commerce service in either runtime. Disabling or losing that service removes only shopping/recommendation information and must never affect normal GrowNerve operation.

## Target extensibility model

GrowNerve's 3D/component ecosystem is being designed around a data-driven boundary:

```text
component JSON + validated local assets
                 |
                 v
         component registry
                 |
                 v
        normalized scene model
            /          \
           v            v
      Three.js       2D UI / MCP
```

Three.js remains a renderer, not the component source of truth.

A reusable component definition is separate from a scene binding, but an operational scene binding does **not** get a second identity. It continues to use the existing GrowNerve `(entity_type, entity_id)` UUID pair so 2D/3D selection, alerts, history, and commands remain aligned.

The component semantic model separates:

```text
category     -> human browsing/discovery
subtype      -> specific component family
capabilities -> GrowNerve-supported behavior/workflows
channel slots-> bindings to existing logical Channel UUIDs
ports        -> physical/topological connections
anchors      -> placement/snap points
properties   -> reusable technical characteristics
```

Tags are discovery metadata only. They never grant behavior, permissions, or safety authority.

Component packs are declarative in V0 and do not execute arbitrary code. MCP is planned as another adapter over the same validated component/farm services rather than a filesystem editor or Three.js controller.

## Target commerce model

Technical component truth and commercial offers are separate systems:

```text
component/domain requirement
          |
          v
neutral compatibility mapping
          |
          v
separate GrowNerve Commerce Service
      /          |           \
     v           v            v
 products      offers      referrals
```

Important consequences:

- technical component definitions contain no affiliate URLs, commission rates, sponsored rank, merchant credentials, or tracking code
- merchant/API credentials and referral configuration never ship in the browser bundle or community component packs
- recommendation compatibility is evaluated before commerce ranking
- affiliate payout is not an input to organic compatibility/ranking
- sponsored results are shown separately and clearly labeled
- recommendations explain why a product is compatible or uncertain
- the commerce service receives only minimal technical/market requirements rather than full farm data
- merchant-specific rules determine whether GrowNerve returns a direct affiliate URL, permitted redirect, referral code, or ordinary non-affiliate link
- commerce is optional and non-critical; the app remains fully useful when it is disabled/offline

See `24-component-plugin-system.md`, `25-mcp-component-authoring.md`, `26-component-taxonomy-and-capabilities.md`, and `27-commerce-catalog-and-referral-service.md` before implementing extensible 3D assets, AI-assisted scene authoring, or commercial product recommendations.

## Non-negotiable constraints

- Local operation must survive Internet loss.
- Essential schedules must survive server loss at the edge.
- Automatic nutrient/pH dosing is not part of the first control release.
- WebGPU is the preferred 3D rendering path with a supported fallback.
- The 3D scene is an operational digital twin, not decorative content.
- Operational 3D identity remains `(entity_type, entity_id)`; do not create a parallel component-instance identity for the same farm entity.
- Reusable component definitions are renderer-agnostic JSON plus validated local assets; Three.js-specific implementation state is not the portable contract.
- Component IDs are stable; schema versions, component SemVer, and immutable content digests are separate concepts.
- Farms pin exact component revisions; component upgrades are explicit rather than silent latest-version replacement.
- Categories are for human discovery/default presentation; capabilities and actual domain/channel bindings drive behavior.
- Tags and third-party metadata cannot grant actions, permissions, automation authority, or safety behavior.
- Add capabilities only when GrowNerve has a concrete consumer and validation semantics; the broad taxonomy does not require broad V0 implementation.
- Imported component packs are declarative in V0 and cannot execute arbitrary JavaScript, WebAssembly, shader source, shell commands, native code, or network callbacks.
- Existing logical `Channel` UUIDs remain telemetry/control identities. Components declare compatible channel slots instead of inventing a second telemetry model.
- Physical/topology ports are distinct from telemetry/control channel bindings.
- Protocol-specific MQTT/ESPHome/Home Assistant/Modbus details belong in runtime bindings/adapters, not reusable component definitions.
- Large immutable component assets do not belong in the whole browser `FarmData` snapshot or the versioned server farm document.
- Current `.grownerve.json` archive schema v1 remains valid; component dependency support must use an explicit v2 migration rather than silently changing v1 semantics.
- MCP authoring cannot bypass farm compare-and-swap concurrency, authorization, validation, audit, or physical safety rules.
- Dangerous commands require server-side safety validation in full mode even when initiated from the 3D UI or future MCP tooling.
- Browser-only mode must clearly identify simulated/non-authoritative control.
- The commerce/referral service is outside the farm/control trust boundary and has no MQTT or farm-database credentials.
- Core farm functionality never requires the commerce service.
- Affiliate/referral credentials and commercial ranking configuration never ship in client code or component packs.
- Affiliate payout cannot alter organic recommendation ordering.
- Sponsored placements are explicit and cannot bypass technical compatibility gates.
- Affiliate and sponsored relationships are disclosed clearly at the actionable recommendation/link surface.
- Merchant link, redirect, price, content, and caching rules are enforced per program instead of assuming one universal affiliate policy.
- Commerce requests use minimum necessary data and do not upload full `FarmData`, telemetry, observations, media, or command history.
- Telemetry and meaningful farm events are separate data classes.
- Domain services do not depend directly on HTTP, MQTT, SQL, MCP, commerce services, or Three.js.
- Browser persistence uses IndexedDB, not `localStorage`.
- Browser exports are versioned and validated before import.
- Configuration belongs in YAML/environment configuration, not hidden constants in application code.
- A production process refuses to start with development authentication, wildcard CORS, an unbounded write rate, or an anonymous broker.
- Authorization and safety are separate checks; neither substitutes for the other.
- Concurrent writes to farm state conflict rather than silently overwriting one another.
- Measurement history is append-only and stored apart from the configuration document.
- The farm/control system stays a modular monolith until actual scaling pressure demonstrates a need to split it; the optional commerce service is a deliberate external commercial boundary rather than a decomposition of farm-control logic.

## Reference installation

All V0 decisions should be tested against one real installation: a 3 x 3 ft DWC tent with one LED, circulation fan, air pump, about 30 L of nutrient solution, four plant positions, and ESP32-based telemetry/control.

If a feature is not useful there and is not required to preserve a clear extension boundary, it is probably not V0.
