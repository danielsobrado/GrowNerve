# 28 — Third-Party Device Integrations

GrowNerve can use off-the-shelf Zigbee devices through a
[Zigbee2MQTT](https://www.zigbee2mqtt.io) bridge. Matter (through the Open Home
Foundation Matter Server) and Home Assistant (import, and read-only export over
MQTT discovery) use the same core and are planned next; see "Status" below.

Integrations are server-mode only. The browser never talks to hardware
(ADR-004), and the browser-only runtime does not offer them.

## How it fits

```text
Zigbee device ── coordinator ── Zigbee2MQTT ──MQTT── zigbee2mqtt adapter ─┐
                                                                           │ readings, availability
                                          internal/integration.Manager ◄──┘
                                            │ binding index (device.integration, channel.integration_key)
                                            ├─► telemetry store (measurements, like any channel)
                                            ├─► liveness heartbeats  ─► device.online
                                            └─► pending-command tracker ─► command "applied"

POST /api/v1/commands ─► safety validation ─► command record ─► DurablePublisher ─► Router
                                                                   │ outbox on failure │
                                                                   ▼                   ├─ integration device ─► adapter.Execute
                                                              OutboxWorker ─► Router ──┘─ anything else     ─► MQTT bridge (ESP32)
```

The decisions behind this are ADR-038 and ADR-039 in
`20-architecture-decisions.md`.

- **Adopting** turns a discovered device into a normal farm device. You pick
  the zone, the device type, and which capabilities become channels. Channel
  kind, value type, unit and range come from the provider, not the browser.
  The write goes through the state committer, so the relational registry (the
  foreign-key target for measurements) is updated with the document.
- **Readings** for adopted capabilities are stored as ordinary measurements,
  in canonical units (`degC`, `%RH`, `%`, `W`, `lx`, `ppm`, …). On/off values are
  stored as `0`/`1` in unit `bool`. Readings for capabilities nobody adopted
  are dropped.
- **Commands** go through the same validation, outbox and expiry as ESP32
  commands. A command becomes `applied` only when the device reports the
  requested state; with Zigbee2MQTT it stays `published` until then and times
  out if the device never confirms. Values are absolute (on, off, 60 %), so a
  retried delivery is harmless.
- **Liveness.** While the provider reports a device available, the server gives
  it a heartbeat every `integrations.liveness_interval`. When the provider
  reports it unavailable, or the connection drops, the device goes offline at
  once and commands to it are refused with `DEVICE_OFFLINE`. If the integration
  code itself stops, heartbeats stop and the normal offline sweep takes over.

## Safety limits

Read these before adopting any output.

- **No edge failsafe.** A smart plug has no schedule, safe state or command
  expiry of its own. If the server is down it keeps its last state. Adoption of
  a controllable capability requires `acknowledge_no_edge_failsafe`. Keep
  lights, pumps and anything that must fail safe on an ESP32 controller.
- **Power-on behaviour.** Set each plug's power-on behaviour in Zigbee2MQTT
  (`power_on_behavior`) to the state that is safe after a power cut.
- **Two controllers.** Automations in Zigbee2MQTT or Home Assistant can fight
  GrowNerve's commands. Don't automate the same output in two places.
- **Sleepy sensors.** Zigbee2MQTT's passive availability timeout is long (25 h by
  default), so a battery sensor with a dead battery can look available. Rely on
  each channel's `stale_after_seconds` alert for sensors.

## Zigbee2MQTT

### Configure

```yaml
integrations:
  liveness_interval: 30s        # at most half of runtime.device_offline_after
  zigbee2mqtt:
    enabled: true
    broker: ""                  # empty: reuse mqtt.broker and its credentials
    base_topic: zigbee2mqtt
    availability_fallback: 90m  # for devices without Zigbee2MQTT availability
```

Environment overrides: `APP_INTEGRATIONS__ZIGBEE2MQTT__ENABLED`,
`APP_INTEGRATIONS__ZIGBEE2MQTT__BROKER`, `APP_INTEGRATIONS__ZIGBEE2MQTT__BASE_TOPIC`.
A separate broker takes `username_env`/`password_env`, which production requires.

Enable `availability` in Zigbee2MQTT. Without it, a device counts as available
while its last message is within `availability_fallback`.

### Run it

- **No hardware:** `docker compose --profile zigbee-sim up` runs a simulated
  bridge with a climate sensor, a plug and a dimmable light
  (`go run ./cmd/device-simulator -mode z2m` outside compose). Start the api with
  `APP_INTEGRATIONS__ZIGBEE2MQTT__ENABLED=true`.
- **Network coordinator:** `ZIGBEE_SERIAL_PORT=tcp://<host>:6638 docker compose --profile zigbee up`.
- **USB coordinator:** add `-f docker-compose.zigbee-usb.yaml` and set
  `ZIGBEE_USB_DEVICE` to the stick's `/dev/serial/by-id/…` path. Docker Desktop
  on Windows needs usbipd for this.

Zigbee2MQTT's own frontend is on port 8081.

### What is offered

| Zigbee2MQTT expose | GrowNerve capability |
|---|---|
| `numeric` published values (temperature, humidity, power, …) | measurement, canonical unit |
| `binary` `state*` inside a switch, light or fan, settable | command, `bool` |
| `brightness` of a light, settable | command, `%` (0-254 scaled) |
| other `binary` values (occupancy, contact, leak) | state, `bool` |
| battery, link quality, `diagnostic` category | offered, unticked by default |
| `config` category, `enum`, `text`, `composite` (colour), `list` | not offered |

Devices are identified by IEEE address, so renaming a device in Zigbee2MQTT does
not break the binding. Friendly names may contain `/`.

### Pairing

`POST /api/v1/integrations/zigbee2mqtt/permit-join {"seconds": 1..254}` opens
the network (0 closes it). It needs the administrator role and uses the
Zigbee2MQTT 2.x request format.

## In the application

**System → Integrations** (server runtime only) shows each provider's state,
the devices it has discovered, and a form for adopting one. The form offers every
non-diagnostic capability ticked, suggests channel keys, and refuses to submit an
output until the missing-failsafe warning has been acknowledged. Adopted devices
appear on the Devices screen with their provider as the source. The browser-only
runtime shows an explanation instead, and its simulator never invents readings
for integration devices.

## API

| Method and path | Role | Purpose |
|---|---|---|
| `GET /api/v1/integrations` | viewer | Every provider's state (`disabled`, `connecting`, `connected`, `degraded`, `error`) |
| `GET /api/v1/integrations/{provider}/devices` | manager | Discovered devices and capabilities, with `adopted_device_id` |
| `POST /api/v1/integrations/{provider}/devices/{externalId}/adopt` | manager | Create the device and chosen channels |
| `POST /api/v1/integrations/{provider}/permit-join` | administrator | Open the network for pairing |

Refusals are problem responses: `UNKNOWN_ZONE`, `ALREADY_ADOPTED`,
`CHANNEL_KEY_CONFLICT`, `UNKNOWN_CAPABILITY`, `INVALID_SAFE_RANGE`,
`EDGE_FAILSAFE_NOT_ACKNOWLEDGED`, `INTEGRATION_DISABLED`.

Integration health never affects `/health/ready`: a missing Zigbee bridge must
not take the farm's own controllers down with it.

## Broker ACL

`deployments/mosquitto/acl.example` gives the server `zigbee2mqtt/#` and gives
the Zigbee2MQTT user only `zigbee2mqtt/#`. The bridge never needs the
`grownerve/v1/devices/#` namespace, so it cannot impersonate an ESP32.

## Status

| Piece | State |
|---|---|
| Integration core, command routing, liveness, adoption API | implemented; unit tests and a real-broker test |
| Zigbee2MQTT adapter | implemented; verified against the simulated bridge and a real Postgres-backed server |
| Integrations screen (status, discovery, adoption, pairing) | implemented; unit tests and a UI run against the real server and simulated bridge |
| Home Assistant import (WebSocket API) | planned |
| Matter (OHF Matter Server WebSocket, commissioning by pairing code) | planned |
| Home Assistant export (read-only MQTT discovery) | planned |
| Releasing a device, rebinding a replacement onto existing channels | planned |

Known limits: a brightness command of 0 may never be confirmed, because some
lights report `state: OFF` and keep their previous brightness, so that command
times out. Zigbee2MQTT fans (`fan_mode`), covers and thermostats are read-only.
