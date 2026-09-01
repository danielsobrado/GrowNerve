# 02 — Scope and Releases

## Release strategy

GrowNerve should ship in narrow vertical slices around the real reference grow. Each release must leave the system usable; no release may depend on unfinished automation to keep plants alive.

## V0 — Digital farm + safe telemetry

Goal: represent the physical farm, record a grow, ingest sensor data, and visualize current/historical state without closed-loop chemistry control.

### Included

- facility and hierarchical zones
- reservoir
- crop catalogue and varieties
- grow cycles and plant positions/cohorts
- stage-aware grow recipes and target ranges
- devices and logical channels
- MQTT ingestion
- air temperature and relative humidity
- water temperature and water level
- equipment state telemetry
- manual observations and photos
- meaningful farm-event timeline
- alerts for stale/offline/out-of-range telemetry
- basic inventory adjustments
- harvest records
- React operational UI
- Three.js digital twin
- selectable 3D entities
- hover/focus tooltips
- click/touch radial action menus
- scene-to-domain selection synchronization
- manual actuator commands for low-risk equipment with server validation

### Not included

- automatic nutrient dosing
- automatic pH correction
- AI crop diagnosis
- computer-vision health scoring
- cloud multi-tenant SaaS
- outdoor GIS
- advanced energy optimization
- complex workflow engine

## V0.5 — Environmental control

Goal: make the farm server and ESP32 controllers jointly responsible for reliable environmental operation.

### Included

- light schedules
- fan PWM schedules and target modes
- air-pump control
- edge configuration synchronization
- command acknowledgements
- command timeout handling
- device watchdog behavior
- local persisted schedules
- manual override with expiry
- automation audit trail
- equipment runtime history
- maintenance reminders based on runtime

## V1 — Hydroponic chemistry monitoring

Goal: add reliable chemistry measurements while remaining human-controlled.

### Included

- pH sensor channels
- EC sensor channels
- calibration events and calibration status
- measurement quality flags
- drift/staleness detection
- chemistry target ranges by recipe stage
- nutrient and pH-adjustment inventory
- assisted dosing recommendations
- manual dosing records
- mixing/wait periods in recommendations
- before/after chemistry views

No automatic chemical dosing in this release.

## V1.5 — Guarded automatic dosing

Goal: automate chemistry only after monitoring and assisted dosing have proven the hardware and safety model.

### Entry criteria

- validated sensors in the target installation
- repeatable calibration procedure
- known pump flow rates
- known mixing times
- tested low-water lockout
- tested stale-sensor lockout
- tested maximum-dose limits
- tested emergency stop
- reliable command acknowledgement
- historical assisted-dosing data

### Included

- nutrient A/B dosing pumps
- pH dosing where explicitly enabled
- bounded dose calculation
- dose -> mix -> measure -> reassess state machine
- hourly/daily dose budgets
- pump continuous-runtime limit
- incompatibility/interlock rules
- operator approval mode
- automatic mode with per-zone enablement

## V2 — Analysis and vision

Potential features after core operations are stable:

- camera assets and scheduled image capture
- WebGPU/local plant-image inference
- canopy coverage estimation
- growth-rate metrics
- visual anomaly tracking
- grow-to-grow comparison
- recipe performance analysis
- energy, water, and nutrient efficiency
- recommendation engine
- farmOS import/export adapter
- optional Home Assistant integration

## Explicit scale assumptions

V0 should comfortably support:

- 1–5 facilities on one local server
- tens of zones
- hundreds of physical devices/channels
- thousands of logical plant positions
- telemetry sampled at practical environmental-control rates

Do not optimize for millions of simultaneous sensors. Do design IDs, data retention, and interfaces so a future larger installation is not blocked.

## Definition of done for each release

A release is complete only when:

- domain invariants are enforced server-side
- schema migrations exist
- OpenAPI is updated
- generated clients are current
- unit/integration tests cover critical behavior
- MQTT/device behavior is simulated in CI where practical
- 3D and 2D interactions resolve to the same domain entities
- unsafe command paths have explicit negative tests
- configuration is documented
- operational failure states are visible to the user
- the reference installation can be exercised end to end
