# 09 — Automation and Safety

## Purpose

Automation in GrowNerve must be understandable, bounded, auditable, and safe under partial failure. The platform should favor explicit state machines and limits over clever but opaque behavior.

## Automation classes

### Schedules

Deterministic time-based behavior.

Examples:

```text
Light ON 06:00
Light OFF 22:00
Fan minimum 35%
Air pump always ON
```

Essential schedules are mirrored to the edge controller.

### Threshold rules

Examples:

```text
if water temperature > 24 C for 10 min -> warning
if water level below emergency threshold -> critical alert
if device heartbeat stale for 5 min -> warning
```

### Control rules

Examples:

```text
maintain fan output based on configured environmental policy
```

### Chemical control

Deferred until V1.5 and protected by stronger interlocks and dose-state machines.

## Rule structure

A rule version should contain:

```text
trigger
conditions
required data freshness
required data quality
action(s)
cooldown
severity/reason
scope
```

Rules are versioned. Historical automation events reference the exact version that caused them.

## Safety architecture

A rule does not directly publish MQTT commands.

```text
Rule evaluates
   -> proposes action
      -> command application service
         -> authorization
         -> safety policy
         -> rate/limit policy
         -> durable command
         -> MQTT outbox
```

Manual actions from the UI follow the same command path.

## Safety policy examples

### Common actuator checks

- target exists and is controllable
- physical provider is online where required
- command value is within capability limits
- command has not expired
- current manual/emergency override permits it

### Pump checks

- maximum continuous runtime
- minimum off interval where required
- leak state
- reservoir state

### Chemical dosing checks

All must pass:

- automation explicitly enabled for this reservoir
- fresh pH/EC readings as applicable
- quality is `good`
- required calibration is valid
- water level above configured minimum
- no active leak condition
- no conflicting dose/mix cycle
- pump calibration/flow data valid
- requested dose below per-event maximum
- rolling hourly dose below maximum
- rolling daily dose below maximum
- last dose older than minimum interval
- emergency stop inactive

Failure is a rejection with a reason code, never silent fallback.

## Dosing state machine

Automatic chemistry control must follow a bounded sequence such as:

```text
IDLE
 -> VALIDATE
 -> DOSE_SMALL_AMOUNT
 -> MIX
 -> WAIT_STABILIZE
 -> MEASURE
 -> ASSESS
 -> COMPLETE
```

Possible exits:

```text
REJECTED
ABORTED
FAULT
LIMIT_REACHED
```

Never implement `EC low -> pump ON until EC target` as a continuous unbounded loop.

## Manual override

Manual overrides should:

- be explicit
- record operator/reason
- have an optional or required expiry
- display prominently while active
- not bypass hardware emergency limits

Example:

```text
Fan fixed at 70% for 2 hours
```

At expiry, normal automation resumes deterministically.

## Emergency stop

Design a farm/zone emergency stop concept even before automatic dosing.

An emergency stop can:

- inhibit dosing/fill/drain actions
- drive configured outputs to safe state
- remain latched until explicitly cleared
- create a critical event

For hazards requiring immediate hardware reaction, a physical/local interlock is preferable to server-only logic.

## Data freshness

Automation must declare how fresh input data must be.

Example:

```text
water_temp stale after 3 min
pH stale after 2 min for dosing
level stale after 1 min for dosing
```

Stale data is not equivalent to the last known good value.

## Hysteresis and duration

Avoid alert/control chatter.

Example:

```text
warn high water temperature when > 24 C for 10 min
resolve when < 23.5 C for 10 min
```

## Command audit

Every actuator change should be explainable:

```text
who/what requested it
rule version if automated
input values used
safety checks performed
requested value
applied value
acknowledgement
start/end state
timestamps
```

## Simulation mode

Before a rule may control real equipment, allow it to run in observe-only mode:

```text
condition matched
would issue fan=60%
action suppressed because rule mode=observe
```

This is especially important for dosing.

## Safe defaults

Configuration must define safe behavior per actuator. There is no universal assumption that OFF is safest: for example, an air pump may need to remain ON during server loss.

Safe state therefore belongs to equipment configuration and edge policy.
