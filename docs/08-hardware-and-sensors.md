# 08 — Hardware and Sensors

## Reference hardware model

GrowNerve should describe hardware by capabilities, not by one vendor-specific board. The first implementation targets ESP32-class controllers and common indoor-farm sensors/actuators.

## Initial reference installation

```text
3 x 3 ft grow tent
  LED fixture ~240 W
  circulation fan
  30 L-class DWC reservoir
  air pump + two air stones
  four plant positions
  ESP32 controller(s)
```

## V0 sensors

### Air temperature

Purpose:

- environment monitoring
- recipe target comparison
- alerting
- later VPD calculation with humidity

Requirements:

- stable digital sensor preferred
- configurable sampling interval
- plausible-range validation
- sensor-offline detection

### Relative humidity

Purpose:

- environment monitoring
- VPD support
- mold/disease-risk context

### Water temperature

Purpose:

- root-zone monitoring
- oxygen-risk context
- recipe target comparison

Waterproof probes require installation guidance and replacement tracking.

### Water level

Purpose:

- refill guidance
- low-water alarm
- later dosing/pump interlock

Do not assume a cheap level sensor is precise enough for volumetric dosing calculations. Treat capability/quality explicitly.

## V1 chemistry sensors

### pH

pH introduces calibration and drift concerns that make it qualitatively different from basic temperature sensing.

Required before control use:

- two-point calibration workflow where applicable
- calibration timestamp/history
- probe-health status
- stabilization handling
- implausible-rate-of-change checks
- configurable maximum calibration age

### EC

Requirements:

- temperature compensation policy is explicit
- calibration history
- unit normalization
- plausible-range checks
- stabilization after nutrient additions

## Later sensors

Potential channels:

- PAR/PPFD
- CO2
- dissolved oxygen
- flow
- weight/load cell
- energy/power
- door/open state
- leak detector
- camera

Each is added only with a concrete use case.

## Actuators

### V0/V0.5

- LED on/off
- fan on/off or PWM percentage
- air pump on/off

### Later

- circulation/water pump
- fill valve
- drain valve
- nutrient A pump
- nutrient B pump
- pH adjustment pump

## Channel capability model

Every channel declares enough metadata for server validation and UI rendering:

```text
kind: measurement | state | command | counter
valueType: number | boolean | enum
unit/dimension where numeric
minimum/maximum physical range
safe command range
precision/display precision
sample interval expectation
stale-after duration
```

Do not encode these values as scattered frontend constants.

## Sensor quality

A measurement quality state should be explicit:

```text
good
suspect
stale
calibrating
fault
unknown
```

The raw measurement is retained even when quality is suspect, unless parsing/authentication makes it unusable. Automation consumes quality-aware data.

## Device replacement

Physical hardware has its own identity and lifecycle. Logical channels remain stable when replacing a sensor.

Example:

```text
reservoir-01.ph
  2026-09-01 .. 2026-12-05 -> pH probe A
  2026-12-05 ..             -> pH probe B
```

This protects historical chart and recipe semantics.

## Calibration events

A calibration record should capture:

- physical sensor/device
- logical channel
- calibration method
- reference solution/standard where applicable
- reference values
- observed values
- coefficients/results
- operator
- timestamp
- pass/fail
- notes

## Installation metadata

For troubleshooting and 3D representation, device installation may include:

- facility/zone
- scene entity binding
- physical port/pin metadata
- electrical role
- installation date
- maintenance interval
- photo/manual reference

Pin assignments belong in device/firmware configuration, not hardcoded application-domain logic.

## Electrical safety boundary

GrowNerve software must assume mains-powered lights, pumps, and fans require properly rated isolation/relays/contactors and electrical installation appropriate to the environment. Low-voltage ESP32 hardware must not be treated as directly capable of safely switching arbitrary mains loads.

The software documentation should identify logical control behavior but must not hide hardware safety requirements.

## Leak handling

Leak detection should eventually be a high-priority hard interlock capable of forcing configured pumps/valves to safe states. It should not depend solely on a browser alert.

## Maintenance

Devices may expose runtime counters. Maintenance scheduling can use:

- elapsed calendar time
- actuator runtime
- number of dosing cycles
- calibration age
- user-defined interval

Examples:

```text
clean air stone
inspect pump tubing
replace pH probe
calibrate EC
clean fan filter
```
