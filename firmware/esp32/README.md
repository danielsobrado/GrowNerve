# GrowNerve ESP32 reference firmware

The controller half of the Phase 8 contract. Essential farm operation must not
depend on the server being reachable.

## What it guarantees

- **Essential outputs keep running without a server.** Photoperiod, fan schedule,
  fan minimum, and reservoir aeration are evaluated from the controller clock
  and persisted configuration. Nothing in that path depends on the network.
- **Configuration survives a reboot.** Accepted configuration is written to NVS
  immediately. The server also publishes desired edge configuration retained so
  a controller can recover from flash or the broker after a restart.
- **Overrides expire locally.** A manual override is bounded by both its absolute
  command expiry and the controller command timeout.
- **Outputs fail safe.** Safe state is applied before networking is initialized,
  whenever the clock is untrusted, and after a watchdog reset. For the reference
  DWC setup, aeration defaults to running.
- **Emergency wiring fails safe.** The reference emergency loop is normally
  closed. Pressing the switch, disconnecting it, or breaking its wire opens the
  circuit and latches the emergency state.

## Hardware configuration

Controller hardware assumptions live in `include/hardware_config.h`. Change that
file for a different PCB or relay module instead of editing control logic.

The reference mapping is:

| Function | GPIO | Reference electrical behavior |
| --- | ---: | --- |
| LED relay | 26 | Polarity set by `GN_LIGHT_RELAY_ACTIVE_HIGH` |
| Fan PWM | 27 | 25 kHz PWM output |
| Air-pump relay | 25 | Polarity set by `GN_AIR_PUMP_RELAY_ACTIVE_HIGH` |
| Emergency loop | 33 | Normally closed to GND, internal pull-up enabled |

The emergency loop is intentionally **normally closed**. Healthy wiring holds
GPIO33 LOW. The controller latches emergency when the input becomes HIGH, so an
open contact, pressed switch, loose connector, or broken wire all fail safe.

Do not connect mains loads directly to an ESP32. Relay/contactors, electrical
protection, isolation, enclosure, grounding, and load ratings are commissioning
requirements outside this reference firmware.

## Edge configuration rules

Wall-clock schedules require `timezonePosix`, not an IANA timezone name. For
example, `Asia/Dubai` is application-level IANA data and must not be sent to the
ESP32 `TZ` runtime as-is. Supply a valid POSIX TZ expression appropriate for the
installation.

The controller validates configuration before replacing the active version:

- photoperiod and fan schedule times must be valid and have a non-zero window;
- command and telemetry intervals are bounded;
- fan percentages and safe outputs must be finite and between 0 and 100;
- schedule channel IDs must be UUIDs;
- wall-clock schedules require a POSIX timezone rule.

Invalid configuration is rejected and the previous persisted configuration stays
active.

## What it does not do

`readSensors()` in `src/main.cpp` is deliberately unimplemented. It reports
`unknown` quality so the server never treats placeholder values as real sensor
measurements. Wiring, calibration, plausibility checks, and per-sensor failure
behavior must be implemented against the commissioned hardware and documented as
part of the commissioning evidence in `docs/23-development-and-operations.md`.

Nutrient and pH dosing are not implemented. Do not add unattended dosing until
the entry criteria in `docs/02-scope-and-releases.md` are satisfied.

## Build

```bash
cp include/secrets.example.h include/secrets.h
# Configure Wi-Fi, MQTT credentials, device identity, channel identities,
# broker CA certificate, and verify include/hardware_config.h for the PCB.
pio run
pio run -t upload
pio device monitor
```

`include/secrets.h` is git-ignored. Provision one broker identity per controller
and apply the topic ACL in `deployments/mosquitto/acl.example` so one compromised
controller cannot impersonate another.

## Keeping the precedence logic aligned

`include/edge_policy.h` mirrors the policy implemented by `internal/edge`. Change
both together. The Go implementation has automated regression coverage; the
firmware implementation is what ultimately drives physical outputs.

## Before connecting real equipment

Complete the commissioning gate in `docs/23-development-and-operations.md`.
Verify the actual GPIO map, relay polarity, emergency-loop continuity, output
safe states, broker ACL, TLS trust, local clock behavior, and every attached load
before enabling physical control.
