# 16 — Deployment and Configuration

## Deployment goals

GrowNerve is local-first and supports two deployment shapes from one frontend codebase:

```text
full local/server mode
browser-only static mode
```

The default production farm runtime remains the local server because browser lifecycle rules are not appropriate for unattended crop-critical control.

## Full local/server runtime

```text
GrowNerve Go server
PostgreSQL
Mosquitto
React static frontend
optional media storage
```

For local development, Docker Compose can provide PostgreSQL and Mosquitto while Go/Vite run natively or in containers.

A small production installation can run:

```text
reverse proxy / TLS where needed
GrowNerve server
PostgreSQL
Mosquitto
static frontend
```

Do not require Kubernetes.

## Browser-only runtime

```text
GitHub Pages / any static host
        |
        v
React + Three.js/WebGPU
        |
    IndexedDB
        |
local simulator / imported data
```

No backend service is required.

The browser-only runtime supports the normal product workflows, history, 3D digital twin, rules, alerts, command state machine against a simulator, and complete JSON import/export.

It is not a substitute for unattended hardware control because browsers can suspend execution and GitHub Pages cannot host the Go/MQTT runtime.

See `21-browser-only-runtime.md`.

## Configuration

Server configuration should be YAML with environment overrides for secrets/deployment-specific values.

Suggested files:

```text
config/default.yaml
config/development.yaml
config/production.example.yaml
```

Conceptual structure:

```yaml
env: development

server:
  address: 127.0.0.1:8080

postgres:
  url_env: GROWNERVE_DATABASE_URL

mqtt:
  broker: tcp://127.0.0.1:1883
  client_id: grownerve-server

telemetry:
  batch_size: 200
  flush_interval: 1s

three_d:
  webgpu_first: true

media:
  provider: filesystem
  path: ./data/media
```

Secrets are referenced through environment variables, not stored directly in YAML.

Frontend build configuration conceptually includes:

```text
VITE_RUNTIME_MODE=server|browser
VITE_BASE_PATH=/ or /GrowNerve/
```

## Constants

Protocol constants, default limits, and shared keys belong in focused constants/config packages. Do not scatter magic values through handlers or React components.

Safety limits that vary by equipment/installation belong to configuration/domain records, not compile-time constants.

## GitHub Pages

GitHub Pages is the primary browser-only static deployment target.

The Pages build must:

- require no API server
- use IndexedDB persistence
- include the Three.js/WebGPU twin
- support `.grownerve.json` import/export
- respect `/GrowNerve/` asset base paths
- use routing compatible with static hosting
- include deterministic pilot/example data only when the user explicitly loads it
- clearly identify browser-only/simulated control

Recommended manual workflow:

```text
npm ci
npm run test
npm run build:browser
npm run deploy:pages
```

No automatic Pages publication is required.

## PWA

The browser-only build should be installable/cacheable as a PWA.

The service worker may cache:

- application shell
- versioned JS/CSS bundles
- icons
- GLB models
- textures

Mutable farm data belongs in IndexedDB, not the service-worker cache.

After first load, the app should reopen offline where the target browser supports the configured PWA behavior.

## Base paths

GitHub Pages project sites use a repository base path:

```text
/GrowNerve/
```

Vite/router/asset loading must support this. Three.js GLB/texture URLs must also respect it.

Do not hardcode `/assets/...` assumptions.

## Routing

A static host cannot provide normal SPA rewrite rules reliably.

The Pages/browser build may use hash history:

```text
/GrowNerve/#/grow-cycles/...
```

The server-backed build may use normal history routing.

Route selection belongs in runtime bootstrap, not scattered page code.

## Browser storage

Browser domain data uses IndexedDB.

The UI exposes:

- approximate storage usage
- export all data
- import archive
- reset local farm

Do not use `localStorage` for grow cycles, events, telemetry, media, inventory, or other domain data.

## Portable backup

Browser-only mode uses versioned `.grownerve.json` archives.

The archive can include all domain data and base64 media. Import validates the full archive before replacing local state.

The same archive format should later be accepted by the full server import workflow so a Pages-only farm can graduate to PostgreSQL without manual re-entry.

## Local DNS / access

A practical full-mode home deployment should be usable from a tablet/phone on LAN through a stable hostname or IP. Remote access is optional and should use secure networking rather than exposing ESP32 controllers publicly.

## Database migrations

Application startup should not silently mutate production schema unless that is an explicit deployment decision. Prefer a dedicated migration command/container step.

Browser IndexedDB schema migrations are versioned separately and must be deterministic. Portable archive schema migration remains independent of the physical IndexedDB schema.

## Backups

Full mode documents commands/scripts for:

- database dump
- database restore
- media backup
- configuration backup

Browser mode provides:

- `.grownerve.json` export
- import/restore
- explicit media inclusion

## Updates

Server/frontend updates and ESP32 firmware updates are separate lifecycles. Normal recipe or schedule changes must not require firmware updates.

The browser-only PWA must handle application upgrades without losing IndexedDB data.

## Pilot example

A deterministic example dataset can be loaded on demand:

```text
one facility
one 3 x 3 tent
one active lettuce grow
one reservoir
light/fan/air pump
sensor values
one warning scenario
```

Example/simulated actions are always labelled and isolated from real server adapters.
