# 26 — Component Taxonomy, Capabilities, and Information Surfaces

## Purpose

This document defines the vocabulary GrowNerve uses to describe what a reusable digital-twin component **is**, what it **can do**, what information it **can expose**, how it **connects**, and how the UI should present it.

It extends the architecture in `24-component-plugin-system.md`. Document 24 defines identity, revisions, storage, validation, scene bindings, and packs. This document defines the semantic catalog that those schemas may reference.

The main design rule is:

```text
category     -> where humans find the component
subtype      -> what specific kind of thing it is
capabilities -> what GrowNerve may do with it
channel slots-> which existing GrowNerve channels may bind to it
ports        -> what physically/topologically connects to it
anchors      -> where/how it may be mounted or snapped
properties   -> reusable technical characteristics
state        -> current operational/render state
UI hints     -> how standard GrowNerve information is prioritized
```

Do not encode behavior only in `category`.

Bad:

```ts
if (component.category === "sensor") showHistory();
```

Target:

```text
capability telemetry + compatible bound measurement channel
    -> GrowNerve may offer current value/history
```

Categories remain intentionally broad. Subtypes, tags, capabilities, and channel bindings carry most of the useful semantics.

## Relationship to the current code

The generic component system is planned work. Today the implemented twin still uses the profile vocabulary:

```text
zone
reservoir
light
fan
plant
```

The first built-in component definitions should reproduce those existing visuals and behaviors. This taxonomy is the target vocabulary used when `profile` is retired as described in documents 12 and 24.

Existing GrowNerve domain concepts remain authoritative:

- `Device` identifies installed controllers/lights/fans/pumps/sensors.
- `Channel` identifies logical measurement/state/command/counter streams.
- `Reservoir`, `Zone`, `PlantPosition`, and other domain entities keep their existing UUIDs.
- telemetry quality remains the existing `good`, `suspect`, `stale`, `calibrating`, `fault`, `unknown` vocabulary.
- command authorization and safety remain outside component definitions.

## Taxonomy model

A component definition may conceptually contain:

```json
{
  "category": "lighting",
  "subtype": "led_panel",
  "tags": ["indoor", "hydroponics", "dimmable", "240v"],
  "capabilities": ["switchable", "variable_output", "scheduled", "power_monitoring"]
}
```

### Category

A small, stable, product-controlled top-level classification used for browsing, icons, filtering, and sensible UI defaults.

### Subtype

A more specific machine-readable classification within a category. Subtypes may expand over time without forcing a new top-level category.

Examples:

```text
category sensor   + subtype ph
category sensor   + subtype water_temperature
category lighting + subtype led_panel
category air      + subtype circulation_fan
category structure+ subtype grow_tent
```

### Tags

Open-ended search/discovery labels. Tags are not trusted for behavior, permissions, or safety.

Examples:

```text
dwc
nft
aeroponics
indoor
rack
lettuce
tomato
pwm
submersible
12v
24v
240v
food_safe
```

### Capabilities

Product-controlled feature identifiers describing generic behavior GrowNerve understands.

A component pack may only use capabilities supported by its declared schema version. Unknown capabilities are validation errors for closed contract fields; community experimentation belongs in tags until GrowNerve formally adopts a capability.

## Official top-level categories

The target category vocabulary is deliberately small enough to remain understandable while covering controlled-environment farms.

| Category | Purpose | Representative subtypes |
|---|---|---|
| `structure` | Physical environment and supporting frames | room, grow_tent, rack, shelf, bench, frame, cabinet |
| `container` | Vessels that hold water, nutrient solution, media, or plants | reservoir, bucket, tray, tank, nft_channel, grow_bed, grow_bag |
| `plant_support` | Hardware that physically holds/supports crops | net_pot, basket, trellis, stake, clip, raft |
| `plant` | Crop/plant visualization bound to agricultural state | lettuce, tomato, pepper, herb, generic_crop |
| `lighting` | Horticultural illumination | led_panel, led_bar, led_strip, uv, far_red, bulb |
| `air` | Air movement and aeration distribution | circulation_fan, exhaust_fan, intake_fan, blower, air_pump, duct |
| `water` | Water movement, distribution, filtration, and drainage | water_pump, hose, pipe, manifold, filter, drain |
| `nutrient` | Nutrient/chemical storage, mixing, and dosing hardware | stock_tank, nutrient_container, dosing_pump, mixer, injection_point |
| `sensor` | Physical measurement instruments | temperature, humidity, ph, ec, co2, level, flow, leak |
| `actuator` | Generic physical output devices not better represented elsewhere | relay, valve, motor, servo, linear_actuator |
| `climate` | Equipment controlling temperature/humidity/gas environment | air_conditioner, heater, humidifier, dehumidifier, co2_injector, vent |
| `electrical` | Power distribution, conversion, protection, and metering | smart_plug, power_strip, psu, breaker, rcd, ups, battery, power_meter |
| `controller` | Computing/control nodes providing channels or local logic | esp32, raspberry_pi, plc, io_module, custom_controller |
| `network` | Connectivity/gateway infrastructure | router, access_point, ethernet_switch, cellular_router, lora_gateway |
| `vision` | Cameras and imaging systems | rgb_camera, timelapse_camera, depth_camera, thermal_camera, multispectral_camera |
| `safety` | Dedicated safety/interlock equipment | leak_detector, smoke_detector, emergency_stop, overflow_sensor, shutoff_valve |
| `storage` | Farm consumable/tool storage locations | seed_bin, chemical_cabinet, tool_shelf, spare_parts_bin |
| `virtual` | Non-physical calculated/logical scene representations | vpd, dli, zone_climate, reservoir_chemistry, energy_summary, alarm_group |
| `assembly` | Reusable composition of multiple component definitions/bindings | dwc_module, aeration_system, rack_level, dosing_station |

Do not create categories for every product family. For example, a peristaltic pump used for dosing is `nutrient/dosing_pump`; a circulation pump is `water/water_pump`. Shared behavior such as `switchable` is a capability.

## Structure components

Structures define the spatial context other components occupy or attach to.

Representative subtypes:

```text
room
grow_tent
rack
shelf
bench
table
frame
cabinet
enclosure
platform
greenhouse_bay
```

Reusable properties may include:

```text
width_m
depth_m
height_m
material
max_load_kg
max_load_kg_per_level
level_count
level_spacing_m
pole_diameter_m
enclosure_rating
```

Useful capabilities:

```text
contains_components
mountable
has_anchors
load_bearing
resizable
```

Potential information:

```text
name/type
dimensions
contained component count
plant count
connected electrical load
active alerts
per-level status
```

### Racks and shelves

Racks deserve explicit metadata because multi-level indoor farms organize lighting, trays, plants, and sensors by level.

A rack may expose level anchors:

```text
level_1
level_2
level_3
level_4
left_post
right_post
top_frame
```

A selected rack could summarize:

```text
Rack A
4 levels
16 occupied plant positions
620 W current lighting load
1 active warning on level 2
```

Do not duplicate child telemetry into the rack. Rack summaries are projections over contained/bound domain entities.

## Container components

Representative subtypes:

```text
reservoir
bucket
tray
propagation_tray
nft_channel
grow_bed
tank
mixing_tank
stock_tank
drain_tank
grow_bag
```

Reusable properties may include:

```text
nominal_capacity_l
recommended_working_capacity_l
width_m
depth_m
height_m
material
opaque
food_safe
lid_type
grow_position_count
drain_height_m
overflow_height_m
```

Possible capabilities:

```text
contains_liquid
level_monitoring
refillable
drainable
mixable
supports_plants
maintenance_tracked
```

Current volume/level belongs to runtime/domain state rather than immutable reusable definition metadata.

A reservoir information surface may show:

```text
DWC Reservoir 01
21.6 / 30 L
Water temperature 20.3 °C · good · 8 s ago
pH 6.1 · good · 12 s ago
EC 1.5 mS/cm · good · 9 s ago
4 occupied plant positions
```

Only show pH/EC when compatible channels are actually bound and have valid measurements.

## Plant-support components

Plant supports remain distinct from plants because hardware survives crop replacement.

Representative subtypes:

```text
net_pot
grow_basket
raft
grow_cup
trellis
stake
clip
plant_cage
```

Useful properties:

```text
diameter_m
height_m
mesh_size_m
material
supported_media
max_plant_mass_kg
```

Possible capabilities:

```text
supports_plants
mountable
replaceable
maintenance_tracked
```

The intended domain relationship is conceptually:

```text
physical net pot/support
        |
        v
PlantPosition
        |
        v
current GrowCycle / crop
```

The plant visualization changes with the grow. The support does not.

## Plant components

Representative subtypes may use crop families or generic visual types:

```text
generic_crop
lettuce
tomato
pepper
basil
kale
strawberry
```

Do not turn reusable component definitions into a second crop database. Crop, variety, grow-cycle, stage, observations, and harvest remain agricultural domain data.

Plant capabilities:

```text
biological
growth_stage
observable
photographable
harvestable
```

Possible visual stages:

```text
seed
germinating
seedling
vegetative
mature
flowering
fruiting
harvest_ready
harvested
dead
```

A crop definition does not need to support every visual stage. The visual-state map is explicit.

A plant tooltip may show:

```text
P3 · Bibb Lettuce
Vegetative · day 18
Health: attention
Last observation: leaf edge curl
```

Displayed plant geometry is an operational representation. It must not imply precise biological simulation unless GrowNerve later implements and validates one.

## Lighting components

Representative subtypes:

```text
led_panel
led_bar
led_strip
horticulture_bulb
uv
far_red
supplemental_light
```

Reusable properties may include:

```text
rated_power_w
rated_voltage_v
ppf_umol_s
efficacy_umol_j
beam_angle_deg
coverage_width_m
coverage_depth_m
minimum_mount_height_m
maximum_mount_height_m
minimum_output_percent
maximum_output_percent
spectrum_reference
```

Possible capabilities:

```text
switchable
variable_output
scheduled
power_monitoring
maintenance_tracked
```

Channel slots may accept:

```text
state command/state
output_percent command/state
power_w measurement
fixture_temperature measurement
ppfd measurement (when a real measurement source is associated)
```

A selected light might show:

```text
240 W LED
On · 72%
168 W measured power
Schedule 06:00–22:00
Runtime today 14 h 23 min
```

Future scene overlays may visualize coverage/PPFD only when the data source and assumptions are explicit. A decorative light cone must not be presented as measured PPFD.

## Air components

Representative subtypes:

```text
circulation_fan
exhaust_fan
intake_fan
blower
air_pump
air_stone
air_hose
duct
```

Potential reusable properties:

```text
rated_power_w
rated_airflow_m3_h
rated_pressure_pa
diameter_m
variable_speed
noise_db
connector_diameter_m
```

Possible capabilities:

```text
switchable
variable_output
scheduled
airflow_source
aeration_source
power_monitoring
maintenance_tracked
```

Runtime visualization may include bounded fan rotation or a small airflow indicator while active. Avoid continuous particle effects unless they add operational value and meet performance budgets.

## Water components

Representative subtypes:

```text
water_pump
submersible_pump
inline_pump
pipe
hose
manifold
filter
drain
sprayer
emitter
check_valve
flow_regulator
```

Reusable properties may include:

```text
rated_flow_l_min
max_head_m
rated_power_w
rated_voltage_v
inner_diameter_m
outer_diameter_m
max_pressure_kpa
material
food_safe
```

Possible capabilities:

```text
switchable
variable_output
flow_producing
flow_monitoring
pressure_monitoring
maintenance_tracked
```

Pipes and hoses are often connection geometry. Their rendered length/path may be generated from endpoint ports rather than stored as a fixed authored mesh.

## Nutrient and chemistry components

Representative subtypes:

```text
nutrient_container
stock_tank
dosing_pump
peristaltic_pump
mixing_tank
agitator
static_mixer
injection_point
calibration_solution_container
```

Reusable properties may include:

```text
rated_flow_ml_min
container_capacity_l
tube_diameter_m
chemical_compatibility
rated_power_w
```

Installed-item/runtime data may include:

```text
material/solution identity
remaining quantity
lot/batch
expiry
pump calibration
last service
```

That installed metadata must not be copied into a reusable generic definition.

Possible capabilities:

```text
switchable
variable_output
calibration
refillable
maintenance_tracked
```

The component never decides dosing. Recommendations and automatic dosing remain GrowNerve domain/automation/safety workflows.

## Sensor components

Sensors may represent one dedicated probe or a multi-sensor instrument. The reusable component declares compatible channel slots; the farm binds those slots to existing logical `Channel` UUIDs.

### Environmental sensor subtypes

```text
air_temperature
relative_humidity
co2
barometric_pressure
voc
lux
ppfd
par
uv
leaf_temperature
```

### Water/nutrient sensor subtypes

```text
water_temperature
ph
ec
tds
orp
dissolved_oxygen
water_level
flow
pressure
turbidity
```

### Substrate/plant sensor subtypes

```text
substrate_moisture
substrate_ec
substrate_temperature
leaf_wetness
sap_flow
stem_diameter
load_cell
```

### Safety-oriented sensor subtypes

```text
leak
overflow
smoke
over_temperature
door_contact
current_anomaly
power_failure
```

Possible capabilities:

```text
telemetry
calibration
historical
threshold_monitoring
maintenance_tracked
replaceable
```

### Sensor information quality is mandatory

Do not display a measurement as trustworthy solely because a number exists.

Preferred information:

```text
Reservoir pH
6.20
Target 5.8–6.3
Quality good
Updated 12 s ago
Last calibration 9 days ago
```

Use the existing GrowNerve measurement-quality vocabulary:

```text
good
suspect
stale
calibrating
fault
unknown
```

Offline device state is separate from measurement quality and may additionally affect presentation.

## Generic actuator components

Representative subtypes:

```text
relay
solenoid_valve
motor
servo
linear_actuator
motorized_damper
motorized_vent
```

Possible capabilities:

```text
switchable
variable_output
positionable
scheduled
maintenance_tracked
```

A reusable actuator definition may describe physical limits, but real command safety limits remain authoritative in GrowNerve channels/interlocks.

## Climate components

Representative subtypes:

```text
air_conditioner
heater
humidifier
dehumidifier
co2_injector
motorized_vent
heat_exchanger
```

Possible capabilities:

```text
switchable
variable_output
scheduled
heating
cooling
humidifying
dehumidifying
ventilation
power_monitoring
maintenance_tracked
```

Possible information:

```text
mode
setpoint
current associated environment measurement
output state
power
runtime
active schedule
alerts
```

Do not infer room temperature from an AC component unless an actual compatible temperature channel is bound or the domain supplies that projection.

## Electrical components

Representative subtypes:

```text
smart_plug
relay_module
power_strip
breaker
rcd_gfci
power_supply
dc_converter
ups
battery
solar_inverter
power_meter
junction_box
```

Reusable properties may include:

```text
rated_voltage_v
rated_current_a
rated_power_w
outlet_count
ac_dc
ip_rating
connector_type
```

Possible capabilities:

```text
switchable
power_monitoring
energy_monitoring
load_bearing_electrical
safety
maintenance_tracked
```

Future derived views may summarize connected load versus rated capacity, but GrowNerve must label calculated/design values separately from live measurements.

## Controller components

Representative subtypes:

```text
esp32
esp8266
raspberry_pi
plc
io_module
custom_controller
```

Reusable properties:

```text
manufacturer
model
io_count
supported_supply_voltage
physical_dimensions
```

Runtime/domain information may show:

```text
online/offline
firmware version
last heartbeat
active configuration version
bound channel count
uptime
supply voltage
signal strength
```

Possible capabilities:

```text
controller
channel_provider
configuration_sync
firmware_reporting
networked
maintenance_tracked
```

Network addresses and credentials are deployment/runtime data and do not belong in reusable community component definitions.

## Network components

Representative subtypes:

```text
router
access_point
ethernet_switch
cellular_router
lora_gateway
zigbee_coordinator
poe_switch
protocol_gateway
```

Possible capabilities:

```text
networked
connectivity_provider
power_monitoring
maintenance_tracked
```

Potential runtime information:

```text
online/offline
link type
signal quality
connected device count
latency/health where measured
```

Secrets, Wi-Fi passwords, API keys, broker credentials, and private keys are never component-definition properties.

## Vision components

Representative subtypes:

```text
rgb_camera
timelapse_camera
macro_camera
depth_camera
thermal_camera
multispectral_camera
```

Possible capabilities:

```text
capture_image
timelapse
stream
vision_analysis
thermal
depth
maintenance_tracked
```

Potential information:

```text
online/offline
last image time
capture schedule
latest approved analysis result
camera target/zone
```

Derived values such as canopy coverage must identify their algorithm/source and observation timestamp; component UI must not present model inference as direct sensor truth.

## Safety components

Representative subtypes:

```text
leak_detector
smoke_detector
overflow_sensor
emergency_stop
breaker
rcd_gfci
water_shutoff_valve
over_temperature_sensor
door_interlock
```

Possible capabilities:

```text
safety
alarm_source
interlock_source
shutdown
manual_reset
```

Safety-related visual state should be clear but restrained. Avoid flashing/bloom-heavy effects as the default status mechanism.

Component metadata never grants or weakens a safety interlock.

## Storage components

Representative subtypes:

```text
seed_bin
nutrient_shelf
chemical_cabinet
tool_shelf
spare_parts_bin
cold_storage
```

Possible capabilities:

```text
contains_inventory
capacity_tracking
maintenance_tracked
```

The physical storage component references/informs inventory UX; actual stock balances remain GrowNerve inventory domain data.

## Virtual components

Not every useful scene object has physical geometry.

Representative subtypes:

```text
vpd
dli
dew_point
zone_climate
reservoir_chemistry
energy_summary
alarm_group
automation_summary
canopy_coverage
```

A virtual component may render as an overlay, badge, card anchor, or scene marker rather than as a physical mesh.

Potential capabilities:

```text
telemetry
historical
calculated
alertable
```

A virtual value must record its source inputs/algorithm or be traceable to an existing domain projection. Do not create opaque calculated numbers inside component JSON.

## Assemblies

Assemblies compose reusable component definitions and topology without merging their domain identities.

Examples:

```text
aeration_system
complete_dwc_module
nft_channel_module
seedling_station
rack_level
four_bucket_dwc_system
climate_control_package
esp32_sensor_node
dosing_station
```

An aeration assembly might contain:

```text
air pump
  -> air hose A -> air stone A
  -> air hose B -> air stone B
```

Assemblies may store:

```text
child component references
relative transforms
internal topology connections
default optional bindings/placeholders
```

Assemblies do not create a new command/safety authority. Actions resolve to underlying domain entities/channels.

## Capability vocabulary

Capabilities should remain product-controlled and composable. They are grouped here by meaning; implementation may stage them over multiple schema revisions.

### V0/core capability subset

The component refactor should begin only with capabilities needed to replace the current profile-based twin:

```text
telemetry
calibration
switchable
variable_output
growth_stage
harvestable
```

Adding dozens of capabilities before a consumer exists violates YAGNI.

### Interaction/layout capabilities

Candidate future capabilities:

```text
selectable
configurable
movable
rotatable
resizable
mountable
contains_components
has_anchors
connectable
```

Basic selection/movement may be scene-editor behavior rather than a component capability when universally available. Add a capability only when its presence meaningfully varies by definition.

### Telemetry/measurement capabilities

```text
telemetry
historical
threshold_monitoring
calibration
quality_aware
staleness_aware
```

Quality/staleness are already domain measurement concepts; component capability flags should only exist if they alter available workflow, not duplicate universally enforced UI rules.

### Control capabilities

```text
switchable
variable_output
positionable
scheduled
manual_override
```

These describe available product workflows. Actual command availability also requires appropriate bound command channels, runtime mode, authorization, and safety acceptance.

### Resource/process capabilities

```text
contains_liquid
refillable
drainable
mixable
flow_producing
flow_monitoring
pressure_monitoring
level_monitoring
airflow_source
aeration_source
```

### Environmental-control capabilities

```text
heating
cooling
humidifying
dehumidifying
ventilation
```

### Crop capabilities

```text
biological
growth_stage
observable
photographable
harvestable
supports_plants
```

### Electrical capabilities

```text
power_monitoring
energy_monitoring
load_bearing_electrical
```

### Lifecycle/maintenance capabilities

```text
maintenance_tracked
replaceable
firmware_reporting
configuration_sync
```

### Controller/connectivity capabilities

```text
controller
channel_provider
networked
connectivity_provider
```

### Vision capabilities

```text
capture_image
timelapse
stream
vision_analysis
thermal
depth
```

### Safety capabilities

```text
safety
alarm_source
interlock_source
shutdown
manual_reset
```

### Logical capabilities

```text
calculated
alertable
contains_inventory
capacity_tracking
```

## Capability governance

Before adding a new capability, require all of the following:

1. At least one concrete component needs it.
2. At least one GrowNerve UI/application workflow consumes it.
3. Its semantics cannot already be expressed by existing channel slots, properties, or capabilities.
4. Validation rules can be stated precisely.
5. It does not duplicate authorization/safety policy.

A capability is not a marketing feature flag and is not an arbitrary community string.

## Channel-slot patterns

Component definitions describe compatible slots; installed scene bindings map those slots to real GrowNerve channel UUIDs.

Common slot patterns include:

### Measurement

```text
value_type number/boolean/enum
kind measurement
semantic dimension/unit constraints
```

Examples:

```text
air_temperature
relative_humidity
water_temperature
ph
ec
co2
water_level
flow
power
energy
```

### State

Examples:

```text
power_state
output_percent
position
mode
```

### Command

Examples:

```text
set_state
set_output_percent
set_position
```

The component system must not invent alternate command semantics. Binding validation reconciles the slot with the existing `Channel.kind`, `value_type`, unit/dimension, and safe range.

## Standard information surfaces

GrowNerve should present component information progressively rather than putting every field into every tooltip.

### Hover tooltip

Purpose: immediate operational recognition.

Recommended maximum content:

```text
name
current status
one or two primary values
highest relevant warning
freshness/quality when a live value is shown
```

Example:

```text
Reservoir pH
6.13 · good
Target 5.8–6.3
Updated 12 s ago
```

### Selected-component card

Purpose: answer "what is happening with this thing now?"

May include:

```text
identity/name
operational state
key bound telemetry
active alerts
assigned zone/grow/reservoir
power/output/runtime
maintenance/calibration status
connection summary
```

### Full inspector

Purpose: detailed operations/configuration/history.

Sections may include:

```text
Overview
Live state
Telemetry/history
Commands/automation
Alerts
Grow/crop context
Maintenance
Calibration
Connections
Installed-item metadata
Component definition/revision
Dimensions/properties
Events/audit
```

Only relevant sections appear. A shelf does not get a calibration tab; a pH sensor does not get a plant-harvest tab.

### Scene overlays

Operational overlays may include:

```text
reservoir water level
selected/highlight state
alert marker
sensor value label
light state
bounded fan rotation
flow direction/active connection
plant health/stage
coverage area when supported by real/calculated data
```

Future aggregate overlays may include temperature/PPFD/airflow heatmaps, but only when backed by explicit measurement or simulation data with clear provenance.

## Standard action catalog

Components may request/display only GrowNerve-owned standard actions. Packs do not define executable actions.

Candidate action families:

### Generic

```text
inspect
history
configure
move
rotate
show_connections
```

### Control

```text
turn_on
turn_off
set_output
override
schedule
```

### Sensor/maintenance

```text
calibrate
mark_serviced
replace
```

### Reservoir/process

```text
record_refill
record_drain
record_input
```

### Crop

```text
observe
add_photo
harvest
```

### Vision

```text
open_camera
capture_image
```

The application action resolver considers:

```text
component capabilities
bound channels
entity type/current state
runtime mode
user role/authorization
safety policy
```

A component declaring `switchable` does not create a power button unless a compatible command path exists.

## Generic operational state

Prefer a small shared presentation state vocabulary:

```text
normal
active
inactive
warning
critical
offline
unknown
maintenance
disabled
pending
fault
```

This presentation state is derived from domain data; it is not a replacement for specific domain states such as command status or measurement quality.

Examples:

```text
fan presentation = active
fan output = 47%

sensor presentation = warning
measurement quality = good
measurement value = 23.2 °C
alert = water temperature high

light presentation = pending
command status = published
acknowledged physical state still = off
```

Never present a real actuator as changed merely because a command was requested.

## Visual-state vocabulary

The renderer may support a small set of predefined safe visual bindings, for example:

```text
visibility
tint/status accent
emissive on/off
bounded rotation speed
normalized fill height
normalized output intensity
status marker
model variant by named state
```

Component JSON must not become a scripting/expression language.

Do not support arbitrary:

```text
JavaScript
shader code
TSL expressions
WebAssembly
callbacks
network fetches
formula execution
```

Complex render behavior belongs in reviewed GrowNerve renderer features referenced by stable capability/visual-state identifiers.

## Physical ports

Ports model topology/resource/electrical connection points, not telemetry channels.

Recommended initial vocabulary:

```text
water.input
water.output
water.drain

air.input
air.output

nutrient.input
nutrient.output

electric.ac_input
electric.ac_output
electric.dc_input
electric.dc_output

network.ethernet
network.usb

pipe.connection
hose.connection
```

A port may include compatibility properties such as:

```text
diameter_m
connector_standard
voltage_v
max_current_a
max_pressure_kpa
flow_direction
```

Do not attempt to model every electrical/network protocol in V0. Add compatibility fields only when GrowNerve has an actual validator/editor use case.

## Spatial anchors

Anchors model physical placement/snap locations.

Recommended vocabulary:

```text
mount.wall
mount.ceiling
mount.floor
mount.surface
mount.shelf
mount.tent_pole
mount.rack_post
mount.reservoir_lid
mount.reservoir_rim
water.submersible
attach.pipe
attach.hose
attach.net_pot
attach.sensor_probe
```

Examples for a tent:

```text
top_front_left_pole
top_front_right_pole
top_rear_left_pole
top_rear_right_pole
ceiling_center
floor_center
```

Persist the resulting explicit transform after snapping. Farm archives must remain deterministic without replaying hidden editor logic.

## Dimensions and units

Physical dimensions are first-class because GrowNerve may later support layout validation and planning.

Canonical stored units use SI:

```text
length        m
mass          kg
time          s (or explicit domain duration fields)
voltage       V
current       A
power         W
energy        Wh/kWh where appropriate
pressure      Pa/kPa
flow          L/min or m3/s according to domain convention
volume        L for farm-facing liquid volumes
temperature   degC for current GrowNerve domain compatibility
```

The UI may convert/display user-preferred units without changing stored values.

Physical definition properties may include:

```text
width_m
height_m
depth_m
diameter_m
mass_kg
mounting_clearance_m
service_clearance_m
```

All numeric physical values must be finite and semantically validated.

## Definition metadata versus installed-item metadata

Reusable definitions and one installed physical item must remain separate.

### Reusable definition metadata

May include:

```text
manufacturer
model
manufacturer_sku
product_family
datasheet reference
rated power
physical dimensions
connector specifications
material
IP rating
```

### Installed-item metadata

Belongs to the farm/domain binding or a future installed-asset record, not the reusable definition:

```text
serial_number
purchase_date
purchase_price
warranty_end
installed_at
last_serviced_at
asset_tag
local nickname
network identity
calibration history
```

Do not publish a user's serial number or farm-specific settings inside a reusable pack.

## Generic versus vendor-specific components

Both are valid.

Generic definitions:

```text
grownerve.sensor.ph.generic
grownerve.lighting.led_panel.generic
grownerve.structure.grow_tent.generic
```

Vendor-specific definitions:

```text
com.vendor.sensor.model-x
com.vendor.light.model-y
```

Use generic components when exact product specifications do not matter. Use vendor-specific components when dimensions, connectors, ratings, model assets, or maintenance data materially improve the digital twin.

A vendor-specific definition must not claim specifications that are not sourced/verified by its publisher.

## Maintenance and lifecycle information

Maintenance is cross-cutting rather than a top-level category.

Candidate installed-item fields/projections:

```text
installed_at
last_serviced_at
next_service_due
runtime_hours
cleaning_due
replacement_due
calibration_due
expected_life_hours
```

Examples:

```text
pH probe calibration due
air stone cleaning due
filter replacement due
fan service runtime
pump tube replacement due
```

Reusable definitions may provide manufacturer-recommended intervals as defaults. Farm-specific maintenance history remains runtime/domain data.

## Derived and calculated information

GrowNerve may show values derived from measurements, configuration, or history, for example:

```text
VPD
DLI
energy today
energy this grow
estimated operating cost
rack total electrical load
reservoir estimated volume
canopy coverage
```

Every derived value must have explicit provenance:

```text
source channels/input data
calculation/version
observation/calculation time
quality/freshness where applicable
```

Never make calculated/design values visually indistinguishable from direct measurements.

## Rich component examples

### Generic LED panel

```json
{
  "schema_version": 1,
  "component_id": "grownerve.lighting.led_panel.generic",
  "version": "1.0.0",
  "name": "Generic LED Grow Panel",
  "category": "lighting",
  "subtype": "led_panel",
  "tags": ["indoor", "grow_light"],
  "capabilities": ["switchable", "variable_output"],
  "properties": {
    "rated_power_w": 240,
    "minimum_output_percent": 0,
    "maximum_output_percent": 100
  },
  "channel_slots": [
    {
      "slot": "state_command",
      "kind": "command",
      "value_type": "boolean"
    },
    {
      "slot": "output_command",
      "kind": "command",
      "value_type": "number",
      "unit": "%"
    },
    {
      "slot": "power",
      "kind": "measurement",
      "value_type": "number",
      "unit": "W",
      "optional": true
    }
  ],
  "anchors": [
    {
      "anchor_id": "mount",
      "type": "mount.ceiling",
      "position_m": [0, 0, 0]
    }
  ]
}
```

`scheduled` and `power_monitoring` should only be added to the formal capability enum when their consumers are implemented; the channel slot may exist before the convenience capability if the schema allows it.

### Generic pH probe

```json
{
  "schema_version": 1,
  "component_id": "grownerve.sensor.ph.generic",
  "version": "1.0.0",
  "name": "Generic pH Probe",
  "category": "sensor",
  "subtype": "ph",
  "capabilities": ["telemetry", "calibration"],
  "channel_slots": [
    {
      "slot": "ph",
      "semantic": "water.ph",
      "kind": "measurement",
      "value_type": "number",
      "dimension": "ph",
      "unit": "pH"
    }
  ],
  "anchors": [
    {
      "anchor_id": "probe_tip",
      "type": "water.submersible",
      "position_m": [0, -0.09, 0]
    }
  ]
}
```

### Four-level rack

A rack definition may carry structural geometry/properties and anchors while child lights/trays/plants remain separate bindings:

```json
{
  "category": "structure",
  "subtype": "rack",
  "properties": {
    "width_m": 1.2,
    "depth_m": 0.6,
    "height_m": 1.9,
    "level_count": 4,
    "max_load_kg_per_level": 50
  }
}
```

Do not embed sixteen plant states and four light states into the rack definition. The UI derives rack-level summaries from contained farm entities.

## Category and subtype governance

Top-level categories are product-controlled and change rarely.

Before adding a category, ask:

1. Can an existing category + new subtype represent it clearly?
2. Does it need materially different browsing/UX/default behavior?
3. Will several real component families use it?
4. Is it stable enough to become part of the portable schema contract?

Prefer adding a subtype over adding a category.

Subtype names use lower `snake_case` and should describe function, not a specific vendor name.

Vendor/product identity belongs in `manufacturer`, `model`, component ID namespace, and tags.

## Search and browsing facets

The component browser should eventually support independent filters for:

```text
category
subtype
capability
tag
manufacturer
model type (primitive/GLB)
pack/namespace
physical dimensions/ranges where useful
required connection type
```

Search results should show only summary metadata initially; detailed specifications/assets load when a component is inspected.

## Security and trust considerations

Taxonomy metadata is descriptive, not authoritative.

A third-party pack cannot gain privileges by declaring:

```text
safety
switchable
shutdown
interlock_source
```

All real operations still require:

```text
valid domain entity/channel binding
valid user authorization
runtime-mode support
server/edge safety policy where physical control is involved
```

Likewise, manufacturer/model/specification metadata is publisher-supplied information. Future remote catalogs may add signatures/trust levels, but GrowNerve must not assume community metadata is manufacturer-certified.

## Implementation order

Do not implement this entire catalog at once.

### Taxonomy T1 — Current-twin replacement

Implement only what is required for the current pilot:

Categories/subtypes:

```text
structure/zone or grow_tent
container/reservoir
lighting/led_panel
air/circulation_fan
plant/lettuce
```

Capabilities:

```text
telemetry
switchable
variable_output
growth_stage
harvestable
calibration (ready for later pH/EC sensor definitions)
```

Information surfaces:

```text
selection
basic tooltip
current state
freshness-aware measurement display
radial actions resolved by GrowNerve
```

### Taxonomy T2 — Reference hardware completeness

Add definitions for:

```text
air pump
air stones
ESP32 controller
air temperature/RH sensor
water temperature sensor
water level sensor
net pots
basic hoses/connections where useful
```

### Taxonomy T3 — Layout/topology

Only after component rendering is generic:

```text
ports
anchors
rack/shelf containment
pipes/hoses
basic connection visualization
```

### Taxonomy T4 — Extended equipment

Add additional subtypes/capabilities driven by real GrowNerve use:

```text
climate equipment
power equipment
vision
network
dosing hardware
maintenance tracking
virtual/derived components
assemblies
```

## Acceptance principles

The taxonomy is working when:

- a new LED, sensor, reservoir, rack, tent, fan, controller, or plant visual can be added without adding a renderer category branch.
- categories remain useful for people while capabilities drive behavior.
- a pH component binds to the existing GrowNerve `Channel` model rather than creating another telemetry system.
- a net pot can remain while the plant/grow occupying it changes.
- a rack can summarize children without owning or duplicating their telemetry.
- current values always communicate quality/freshness where relevant.
- community metadata cannot grant permissions or bypass safety.
- component definitions remain portable between browser and server runtimes.
- simple components can use primitives while exact/vendor components can later use validated GLBs.
- the schema remains small enough to implement incrementally despite the broader documented vocabulary.

## Non-goals

This taxonomy does not mean V0 should implement:

- every listed category/subtype
- a full electrical-design package
- fluid-dynamics simulation
- PPFD simulation presented as measured data
- HVAC engineering calculations
- a BIM/CAD ontology
- arbitrary community-defined capabilities
- automatic protocol discovery
- arbitrary component scripts
- automatic dosing behavior inside component definitions
- a second inventory/crop/telemetry/domain model

The broad taxonomy preserves a coherent direction. Actual schema fields and capabilities are added only when GrowNerve has a concrete consumer and testable use case.
