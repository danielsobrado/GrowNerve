# 16 — Deployment and Configuration

## Deployment goals

GrowNerve is local-first. The default deployment should be easy to run on a small local server while preserving a path to larger installations.

## Initial runtime

```text
GrowNerve Go server
PostgreSQL
Mosquitto
React static frontend
optional media storage
```

For local development, Docker Compose can provide PostgreSQL and Mosquitto while Go/Vite run natively or in containers.

## Production shape

A small production installation can run:

```text
reverse proxy / TLS where needed
GrowNerve server
PostgreSQL
Mosquitto
static frontend
```

Do not require Kubernetes.

## Configuration

Configuration should be YAML with environment overrides for secrets/deployment-specific values.

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

## Constants

Protocol constants, default limits, and shared keys belong in focused constants/config packages. Do not scatter magic values through handlers or React components.

Safety limits that vary by equipment/installation belong to configuration/domain records, not compile-time constants.

## GitHub Pages

The frontend may be published manually to GitHub Pages for demonstration/static mock deployments using `npm` scripts.

Important limitation: GitHub Pages can host only the static browser application. Real control requires a reachable GrowNerve API/MQTT-backed local server.

The frontend should support a demo/static data provider so the public Pages build can demonstrate the UI/3D twin without pretending to control real hardware.

Recommended manual workflow later:

```text
npm ci
npm run build
npm run deploy:pages
```

No automatic Pages publication is required initially.

## Base paths

Because GitHub Pages project sites use a repository base path, Vite/router/asset loading must support a configurable base path. Three.js GLB/texture URLs must also respect it.

Do not hardcode `/assets/...` assumptions that break under `/GrowNerve/`.

## Local DNS / access

A practical home deployment should be usable from a tablet/phone on LAN through a stable hostname or IP. Remote access is optional and should use secure networking rather than exposing ESP32 controllers publicly.

## Database migrations

Application startup should not silently mutate production schema unless that is an explicit deployment decision. Prefer a dedicated migration command/container step.

## Backups

Document commands/scripts for:

- database dump
- database restore
- media backup
- configuration backup

## Updates

Server/frontend updates and ESP32 firmware updates are separate lifecycles. Normal recipe or schedule changes must not require firmware updates.

## Demo mode

A demo provider should generate deterministic farm state for GitHub Pages and UI testing:

```text
one facility
one 3 x 3 tent
one active lettuce grow
one reservoir
light/fan/air pump
sensor values
one warning scenario
```

Demo actions should be clearly simulated and isolated from production adapters.
