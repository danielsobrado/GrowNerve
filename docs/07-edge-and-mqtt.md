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
grownerve/v1/devices/{deviceId}/telemetry
grownerve/v1/devices/{deviceId}/state
grownerve/v1/devices/{deviceId}/health
grownerve/v1/devices/{deviceId}/commands
grownerve/v1/devices/{deviceId}/acks
grownerve/v1/devices/{deviceId}/config
```

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
