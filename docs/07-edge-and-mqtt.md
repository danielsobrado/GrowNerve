# 07 — Edge and MQTT

## Purpose

The edge layer connects physical farm hardware to GrowNerve without making the farm dependent on the central server for basic survival.

ESP32 controllers publish telemetry, receive commands, enforce local hardware limits, retain essential schedules, and report health.

## Broker

Use a local Mosquitto broker for the initial system.

The browser never connects to MQTT directly.

## Topic structure

Prefer stable UUID/device identifiers in protocol topics and human-readable aliases only for debugging.

Suggested pattern:

```text
grownerve/v1/devices/{deviceId}/telemetry     device -> server
grownerve/v1/devices/{deviceId}/state         device -> server
grownerve/v1/devices/{deviceId}/health        device -> server
grownerve/v1/devices/{deviceId}/acks          device -> server
grownerve/v1/devices/{deviceId}/config/ack    device -> server
grownerve/v1/devices/{deviceId}/commands      server -> device
grownerve/v1/devices/{deviceId}/config        server -> device, retained
```

The direction column is the ACL. `deployments/mosquitto/acl.example` grants each
controller write access to its own device-to-server topics and read access to its
own server-to-device topics, and nothing else, so a compromised controller can
neither publish telemetry attributed to another device nor read another device's
commands.

`config` is the one retained topic. Retention is what lets a controller that
reboots during a server outage recover its schedules from the broker instead of
waiting for a server that may not come back. Everything else is transient: a
retained command would be re-delivered on every reconnect, which is exactly the
duplicate actuation the command identity and expiry exist to prevent.

Avoid one MQTT topic per measured scalar if that creates excessive subscription/configuration complexity. A device may publish compact batches with channel IDs.

## Telemetry envelope

Conceptual payload:

```json
{
  "protocolVersion": 1,
  "deviceId": "...",
  "bootId": "...",
  "sequence": 1822,
  "observedAt": "2026-09-01T09:00:00Z",
  "samples": [
    {"channelId": "...", "value": 23.4, "unit": "degC", "quality": "good"},
    {"channelId": "...", "value": 68.1, "unit": "%RH", "quality": "good"}
  ]
}
```

The server validates channel ownership, units, plausible range, ordering metadata, and timestamps before persistence.

## Device state

State messages describe current equipment state, not commands:

```json
{
  "deviceId": "...",
  "sequence": 130,
  "states": [
    {"channelId": "...", "value": true},
    {"channelId": "...", "value": 45}
  ]
}
```

## Commands

Command messages carry a durable command UUID.

```json
{
  "protocolVersion": 1,
  "commandId": "...",
  "targetChannelId": "...",
  "type": "set_percent",
  "value": 45,
  "issuedAt": "...",
  "expiresAt": "..."
}
```

The edge controller must reject:

- expired commands
- unsupported command types
- unknown target channels
- invalid ranges
- duplicated commands already applied
- commands violating hardware-local limits

## Acknowledgements

Every command receives a durable acknowledgement result:

```text
accepted
applied
rejected
failed
```

Include reason codes and applied value.

The server considers lack of acknowledgement a timeout, not success.

## Configuration synchronization

The server sends versioned device configuration containing only what the device needs:

- channel bindings
- sampling intervals
- essential schedules
- safe output states
- hardware limits
- calibration coefficients where edge-applied
- heartbeat interval

Configuration contains a version/hash. Device health reports the active version.

## Local schedules

Essential examples:

```text
Light: ON 06:00, OFF 22:00
Air pump: always ON
Fan: minimum 35%
```

The device persists the latest accepted essential configuration to non-volatile storage and resumes it after restart.

Server-side automation may supersede local schedules while connected, but a clear precedence model is required.

Recommended precedence:

```text
hardware emergency interlock
  > local safety limit
  > explicit emergency/manual safe command
  > server temporary override
  > server automation
  > persisted essential schedule
  > configured default safe state
```

## Offline behavior

### Broker/server disconnected

- continue essential schedules
- keep safe outputs
- mark server disconnected locally
- optionally buffer a bounded telemetry window
- never allow telemetry buffering to exhaust memory or destabilize control
- reject operations that require fresh central authorization if designed that way

### Time uncertainty

Schedules require a clock strategy. Devices should synchronize using local network time when available and retain reasonable RTC/timekeeping between syncs. If clock confidence becomes insufficient, use documented fail-safe behavior rather than silently shifting photoperiod.

## Heartbeats

Health should include:

```text
firmwareVersion
protocolVersion
bootId
uptime
RSSI
freeHeap
resetReason
activeConfigVersion
lastServerContact
sensor fault summary
output state summary
```

## Firmware architecture

Keep firmware modules small:

```text
network
mqtt
config
scheduler
sensors
actuators
watchdog
storage
health
```

Hardware drivers implement interfaces; business-safe behavior lives above drivers.

## Firmware updates

OTA can be added after the first stable firmware. Updates must support rollback or safe recovery. Firmware deployment should never be required merely to change normal farm schedules or recipe setpoints.

## Security

Use authenticated broker connections. Per-device credentials are preferred over one shared farm-wide credential. Production deployment should support TLS on networks where threat model requires it.

## Simulator

A software device simulator is a first-class development tool. It should:

- connect to MQTT
- emulate telemetry
- emulate actuator state
- acknowledge commands
- inject sensor failures
- go offline/reconnect
- simulate stale values and command rejection

This enables backend/UI development before hardware is connected and enables CI coverage of protocol behavior.

## Edge configuration envelope

Published retained by the server whenever a device's desired configuration
version differs from the version it reports as active.

```json
{
  "protocolVersion": 1,
  "deviceId": "...",
  "configVersion": "pilot-v3",
  "issuedAt": "2026-09-01T09:00:00Z",
  "config": {
    "photoperiod": {"onHour": 6, "onMinute": 0, "offHour": 22, "offMinute": 0, "channelId": "..."},
    "fanMinimumPercent": 30,
    "airPumpAlwaysOn": true,
    "safeOutputs": {"<lightChannelId>": 0, "<fanChannelId>": 30, "<airPumpChannelId>": 100},
    "telemetryIntervalSeconds": 10,
    "commandTimeoutSeconds": 300
  }
}
```

Only behaviour a controller can run alone belongs here. Anything needing server
judgement is deliberately absent, because a controller must never be left
half-autonomous.

`safeOutputs` is the value each output falls back to when nothing else applies.
For aeration that value is *running*: "off" is not a safe default for a
deep-water crop.

### Configuration acknowledgement

```json
{
  "protocolVersion": 1,
  "deviceId": "...",
  "configVersion": "pilot-v3",
  "accepted": true,
  "acknowledgedAt": "2026-09-01T09:00:02Z"
}
```

A controller that rejects a configuration keeps running the last one it accepted
and reports `accepted: false` with a reason. The server tracks adoption by the
version the device reports in its health messages, not by the version it sent, so
a configuration that never reached the hardware is visible as such rather than
assumed applied.

## Output precedence

Every output is resolved in this fixed order, on the controller, every cycle:

```text
hardware interlock
local safety limit
emergency stop
manual override (until it expires)
automation
essential schedule
default safe value
```

The order is implemented twice on purpose: `internal/edge` in Go, which CI
exercises, and `firmware/esp32/include/edge_policy.h`, which drives the relays.
See ADR-036. An override is additionally bounded by the controller's own
`commandTimeoutSeconds`, so a server that disappears mid-override cannot latch an
output indefinitely.
