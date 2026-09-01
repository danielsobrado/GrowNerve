# 23 — Development and Operations

## Prerequisites

- Go 1.25.13 or newer compatible patch release
- Node.js 22.13 and npm 10.9 or newer
- Docker with Compose v2 for the full local stack
- optional generation tools: `sqlc` and `oapi-codegen`
- optional, for the controller: PlatformIO (`pip install platformio`)

## Browser-only runtime

```bash
cd frontend
npm ci
npm run dev:browser
```

Open `http://127.0.0.1:5173/GrowNerve/`, then create an empty farm, load the
deterministic pilot, or import a `.grownerve.json` archive. Browser control is
visibly simulated and is never a physical acknowledgement.

Build the static PWA with `npm run build:browser`. `npm run deploy:pages`
prepares the GitHub Pages artifact without publishing it.

## Full local stack

```bash
cp .env.example .env
docker compose up -d --build --wait
docker compose --profile simulator up -d simulator
cd frontend
npm ci
npm run dev
```

The default endpoints are API `127.0.0.1:8080`, PostgreSQL `127.0.0.1:5432`,
MQTT `127.0.0.1:1883`, and frontend `127.0.0.1:5173`. All infrastructure ports
bind to loopback. Change the example database credentials for any
persistent or shared installation.

Development runs with `auth.mode: dev`, which grants an administrator principal
with no credential. That is convenient locally and refused in production.

Useful checks:

```bash
curl -fsS http://127.0.0.1:8080/health/live
curl -fsS http://127.0.0.1:8080/health/ready
curl -fsS http://127.0.0.1:8080/version
curl -fsS -N http://127.0.0.1:8080/api/v1/stream
docker compose logs -f api mosquitto simulator
```

Stop services with `docker compose down`. `docker compose down --volumes`
permanently removes local PostgreSQL, broker, and media data and should only be
used for a deliberate development reset.

## Enabling authentication locally

Local mode stores only the SHA-256 of each token, so no usable credential ever
sits in a configuration file.

```bash
TOKEN=$(openssl rand -hex 32)
printf '%s' "$TOKEN" | shasum -a 256
```

Put the digest in `.env` as `GROWNERVE_LOCAL_ACCOUNTS=alice:manager:<digest>`,
set `APP_AUTH__MODE=local`, and restart. The frontend reads its credential from
`sessionStorage` under `grownerve.token`.

Roles are `viewer`, `operator`, `manager`, and `administrator`. Reading needs a
viewer, issuing a command needs an operator, and rewriting the farm
configuration needs a manager.

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

Frontend coverage is enforced over `src/domain` and `src/runtime`. Screens and
the twin are covered by the Playwright journeys instead; this scoping is
deliberate and is recorded in `22-implementation-status.md`.

### Integration tests

Tests that need a real dependency **skip** when it is not configured, so a plain
`go test ./...` stays green on a laptop. To actually run them:

```bash
docker compose up -d postgres mosquitto
export GROWNERVE_TEST_DATABASE_URL="postgresql://postgres:postgres@127.0.0.1:5432/grownerve?sslmode=disable"
export GROWNERVE_TEST_MQTT_BROKER="tcp://127.0.0.1:1883"
go run ./cmd/migrate up
go test -race ./...
```

CI sets both variables and fails the build if any integration test reports
`SKIP`, because a silently skipped integration test is worse than none.

Validate OpenAPI with `npx --yes @redocly/cli@1.34.5 lint api/openapi.yaml`.
Regenerate code with `task gen` or `make gen`, then confirm there is no
unexpected generated drift.

## Backup and restore

Browser farms use Settings → Export for a complete versioned archive. Test
restore in a separate browser profile before depending on a backup. Server
deployments should combine PostgreSQL-native backups with exported farm archives
where portability is required. Note that measurement history lives in the
relational tables, not in the farm document, so a database backup is required to
preserve it.

Importing a browser archive into the server preserves its history: measurements
submitted through `PUT /api/v1/state` are adopted into the measurement store
rather than discarded.

## Telemetry retention

Retention defaults to keeping everything, because silently discarding a grower's
history would be worse than storing it. Choose a policy deliberately by setting
`telemetry.retention` (or `APP_TELEMETRY__RETENTION`), for example `8760h` for a
year. The retention job logs how many rows it removes.

## Production checklist

Configuration validation refuses to start a production process that gets any of
these wrong, but check them deliberately rather than relying on the error:

1. `auth.mode` is `local` or `oidc`, never `dev`;
2. `server.cors_allowed_origins` names real origins, never `*`;
3. `server.rate_limit.write_per_second` is set;
4. `mqtt.username_env` and `mqtt.password_env` are set, and the broker uses
   `deployments/mosquitto/mosquitto.production.conf` with `allow_anonymous
   false`, TLS, and the per-device ACL from `acl.example`;
5. TLS terminates at a reverse proxy in front of the API; PostgreSQL and the
   broker are never exposed directly;
6. `telemetry.retention` reflects a decision someone made.

## Commissioning gate

Before connecting real equipment:

1. identify the exact relay/PWM, voltage, current, normally-open/closed, and
   fail-state behavior of every output, and confirm every pin constant in
   `firmware/esp32/src/main.cpp` against the actual wiring;
2. provision a unique broker identity and topic ACL per controller;
3. complete `readSensors()` in the firmware, verify each sensor independently,
   record calibration metadata, and force stale/disconnected/out-of-range
   scenarios;
4. prove watchdog reset, retained edge configuration, server-loss schedules,
   command expiry, duplicate handling, and the precedence order in
   `07-edge-and-mqtt.md`. The software half of this is asserted by
   `internal/edge/controller_test.go` and the retained-configuration integration
   test; the hardware half is not, and simulator success is not commissioning
   evidence;
5. operate light, fan, and air pump manually under observation before enabling
   any schedule;
6. keep nutrient and pH dosing physically disconnected until the later safety
   entry criteria are signed off.

Record evidence, firmware/config versions, wiring diagrams, rollback steps, and
the responsible operator for every commissioning session.

## Incident defaults

On uncertain physical state, stop issuing commands, use the hardware
emergency/isolation control, preserve logs and the database, and compare desired
state with local controller state. Do not clear an emergency state merely to
restore UI control. Recovery must revalidate sensors, output state, active edge
configuration, and time synchronization.

The audit log records who requested what and why it was refused. Query it
directly when reconstructing an incident:

```sql
SELECT occurred_at, action, target_type, target_id, detail
FROM audit_log ORDER BY occurred_at DESC LIMIT 100;
```
