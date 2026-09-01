# GrowNerve ESP32 reference firmware

The controller half of the phase 8 contract. It exists so essential farm
operation does not depend on the server being reachable.

## What it guarantees

- **Essential outputs keep running without a server.** The photoperiod, the fan
  minimum, and reservoir aeration are evaluated from the controller's own clock
  and its persisted configuration. Nothing in that path touches the network.
- **Configuration survives a reboot.** Accepted configuration is written to NVS
  immediately, and the server publishes it retained, so a controller that
  reboots mid-outage recovers from flash or from the broker.
- **Overrides expire.** A manual override is bounded by the controller's own
  timeout, so a server that disappears mid-override cannot latch an output.
- **Outputs fail safe.** Safe state is driven before the network exists and
  whenever the clock is untrustworthy. For aeration, safe means running.
- **A wedged controller resets.** The task watchdog reboots into safe state
  rather than leaving outputs in an unknown condition.

## What it does not do

`readSensors()` in `src/main.cpp` is deliberately unimplemented. It reports the
`unknown` quality for every channel so the server treats readings as
untrustworthy rather than charting invented numbers. Wiring, calibration, and
per-sensor fail behaviour are commissioning decisions; fill this in against the
actual hardware and record the calibration evidence the commissioning gate in
`docs/23-development-and-operations.md` requires.

Nutrient and pH dosing are not present at all, and must not be added until every
entry criterion in `docs/02-scope-and-releases.md` is satisfied.

## Build

```bash
cp include/secrets.example.h include/secrets.h
# fill in credentials, device id, and channel ids, then:
pio run
pio run -t upload
pio device monitor
```

`include/secrets.h` is git-ignored. Provision one broker identity per
controller, with the topic ACL in `deployments/mosquitto/acl.example`, so a
compromised device cannot publish telemetry attributed to another.

## Keeping the precedence logic honest

`include/edge_policy.h` is a direct port of `internal/edge` in the server
repository, and `internal/edge/controller_test.go` is what exercises it in CI.
The two must be changed together: the Go copy is the tested one, and this copy
is what drives the relays.

## Before connecting real equipment

Work through the commissioning gate in `docs/23-development-and-operations.md`.
Confirm every pin against the actual wiring first — the pin constants at the top
of `src/main.cpp` are a starting point, not a description of your build.
