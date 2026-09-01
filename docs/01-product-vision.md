# 01 — Product Vision

## Purpose

GrowNerve is a local-first operating system for controlled-environment crop production. It combines crop records, sensor telemetry, equipment control, automation, observations, media, inventory, harvest history, and an interactive 3D digital twin in one coherent model.

The system is designed first for small indoor farms, grow tents, hydroponic systems, racks, reservoirs, and LED-based environments. The architecture should later scale to larger indoor farms and greenhouses without forcing the first version to carry commercial-farm complexity prematurely.

## Product promise

GrowNerve should answer five questions quickly:

1. What is growing now?
2. What are the environmental and reservoir conditions now?
3. Is anything outside its target range?
4. What equipment is running and why?
5. What happened during this grow, and how did it affect the outcome?

Later releases should also answer:

- What action is recommended?
- Which recipe or environmental pattern produced the best result?
- Which changes improved yield, quality, energy use, water use, or crop time?

## Core product surfaces

### Operational overview

A concise current-state view of zones, crop cycles, reservoirs, alerts, equipment, and upcoming work.

### Digital twin

A Three.js scene representing the actual facility layout and managed entities. Users can select plants, sensors, reservoirs, lights, fans, controllers, pumps, and zones directly in 3D.

### Crop history

Grow cycles connect stage history, recipe targets, observations, photos, inputs, telemetry, alerts, actions, and harvest results.

### Control

Schedules and automation rules issue commands to edge devices. Control remains bounded by explicit safety policies.

### Analysis

Historical data can compare grows and correlate environmental conditions, nutrient management, equipment operation, observations, and outcomes.

## Product principles

### Local-first, not cloud-dependent

A farm must continue operating during Internet loss. The local server is authoritative for farm state. Edge controllers retain essential operating schedules and safe states.

### A farm record should explain itself

A graph that shows pH changing is useful. A graph that shows pH changing alongside the water addition, nutrient input, calibration, alert, and resulting crop observation is much more useful. GrowNerve records actions and context around measurements.

### Telemetry is not the domain

Sensor values support crop and control decisions. The product is not a generic IoT dashboard.

### 3D must improve operations

The digital twin must make spatial understanding, inspection, control, commissioning, and troubleshooting faster. Any 3D feature that only looks impressive but is slower than the normal UI should not be prioritized.

### Human-assisted before autonomous

Monitoring comes first. Recommendations come next. Automatic chemical dosing comes only after calibration, validation, safeguards, and real operational experience exist.

### Opinionated domain, extensible hardware

Crop-cycle concepts should be explicit. Hardware should be abstracted through devices and channels so sensors and actuators can change without rewriting the crop model.

## Initial users

### Home / enthusiast grower

Runs one or a few tents or hydroponic systems and wants reliable control, history, and experimentation without a large commercial platform.

### Small indoor farm operator

Runs multiple zones/racks and needs alarms, crop cycles, equipment visibility, repeatable recipes, inventory, and maintenance history.

### Research / experimentation user

Needs controlled grow histories and comparisons without the heavier field-trial workflow of a full agronomic research platform.

## What GrowNerve is not

- not a generic home-automation replacement
- not a farm ERP
- not an outdoor GIS-first farm-management platform
- not a generic SCADA replacement
- not a scientific LIMS
- not an AI chatbot wrapped around sensor charts

It may integrate with those categories later, but its center is controlled-environment growing.

## Success criteria for the first real installation

GrowNerve V0 is successful when one real grow can be managed end to end with:

- a configured facility, zone, reservoir, and plant positions
- a crop cycle and stage-aware recipe
- live environmental and reservoir telemetry
- light/fan/air-pump state and control
- alerts for failed or unsafe conditions
- observations and photos
- a complete event timeline
- a useful harvest record
- a 3D scene that represents and selects the real components
- operation that continues safely during Internet loss
