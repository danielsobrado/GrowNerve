# 23 — Development and Operations

## Prerequisites

- Go 1.25.13 or newer compatible patch release
- Node.js 22.13 and npm 10.9 or newer
- Docker with Compose v2 for the full local stack
- optional generation tools: `sqlc` and `oapi-codegen`

## Browser-only runtime

```bash
cd frontend
npm ci
npm run dev:browser
```

Open `http://127.0.0.1:5173/GrowNerve/`, then create an empty farm, load the deterministic pilot, or import a `.grownerve.json` archive. Browser control is visibly simulated and is never a physical acknowledgement.

Build the static PWA with `npm run build:browser`. `npm run deploy:pages` prepares the GitHub Pages artifact without publishing it.

## Full local stack

```bash
cp .env.example .env
docker compose up -d --build --wait
docker compose --profile simulator up -d simulator
cd frontend
npm ci
npm run dev
```

The default endpoints are API `127.0.0.1:8080`, PostgreSQL `127.0.0.1:5432`, MQTT `127.0.0.1:1883`, and frontend `127.0.0.1:5173`. All infrastructure ports bind to loopback. Change the example database credentials for any persistent/shared installation.

Useful checks:

```bash
curl -fsS http://127.0.0.1:8080/health/live
curl -fsS http://127.0.0.1:8080/health/ready
curl -fsS http://127.0.0.1:8080/version
docker compose logs -f api mosquitto simulator
```

Stop services with `docker compose down`. `docker compose down --volumes` permanently removes local PostgreSQL and broker data and should only be used for a deliberate development reset.

## Quality gates

```bash
go test -race ./...
go vet ./...
cd frontend
npm run typecheck
npm run lint
npm run test:coverage
npm run test:e2e
npm run build
npm run build:browser
npm audit --audit-level=high
```

Validate OpenAPI with `npx --yes @redocly/cli@1.34.5 lint api/openapi.yaml`. Regenerate code with `task gen` or `make gen`, then confirm there is no unexpected generated drift.

## Backup and restore

Browser farms use Settings → Export for a complete versioned archive. Test restore in a separate browser profile before depending on a backup. Server deployments should combine PostgreSQL-native backups with exported farm archives where portability is required.

## Commissioning gate

Before connecting real equipment:

1. identify the exact relay/PWM, voltage, current, normally-open/closed, and fail-state behavior of every output;
2. provision a unique broker identity and topic ACL per controller;
3. verify each sensor independently, record calibration metadata, and force stale/disconnected/out-of-range scenarios;
4. prove watchdog reset, retained edge configuration, server-loss schedules, command expiry, duplicate handling, and the precedence order in `07-edge-and-mqtt.md`;
5. operate light, fan, and air pump manually under observation before enabling any schedule;
6. keep nutrient and pH dosing physically disconnected until the later safety entry criteria are signed off.

Record evidence, firmware/config versions, wiring diagrams, rollback steps, and the responsible operator for every commissioning session.

## Incident defaults

On uncertain physical state, stop issuing commands, use the hardware emergency/isolation control, preserve logs and the database, and compare desired state with local controller state. Do not clear an emergency state merely to restore UI control. Recovery must revalidate sensors, output state, active edge configuration, and time synchronization.
