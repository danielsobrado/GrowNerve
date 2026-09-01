# 10 — Grow Cycles and Recipes

## GrowCycle as the central operating record

A GrowCycle is the main agricultural context in GrowNerve. It connects what is being grown, where it is growing, which recipe applies, plant positions/cohort, environmental history, reservoir history, observations, inputs, alerts, actions, and harvest results.

## Lifecycle

Suggested states:

```text
planned
active
completed
abandoned
```

Typical flow:

```text
plan
 -> seed
 -> germination/seedling
 -> transplant
 -> vegetative growth
 -> harvest window
 -> harvested/completed
```

Stages are crop/recipe concepts, not hardcoded global states. Different recipes may define different stage names.

## GrowCycle fields

Conceptually:

```text
id
facility_id
crop_id
variety_id
recipe_version_id
name/code
status
planned_start
actual_start
completed_at
plant_count
notes
```

Related records define zones, reservoirs, positions, observations, and harvests.

## Cohorts vs individual plants

The default should be cohort-based to reduce data-entry burden.

Example:

```text
Grow Cycle #42
Bibb Lettuce
4 positions
```

Only create individual plant-specific records/observations when there is a reason.

Plant positions can remain stable spatial objects even between grow cycles:

```text
Tent 01 / P1
Tent 01 / P2
Tent 01 / P3
Tent 01 / P4
```

This works naturally with the 3D scene.

## Recipe model

Recipes are versioned definitions of desired conditions and schedules for a crop/variety.

```text
GrowRecipe
  -> RecipeVersion
      -> Stage 1
      -> Stage 2
      -> Stage 3
```

A published version is immutable.

## Stage targets

Potential setpoints:

```text
air temperature range
relative humidity range
water temperature range
pH range
EC range
photoperiod
light intensity target where measurable
fan strategy
CO2 target later
```

Not every recipe needs every target.

## Target semantics

A target should distinguish:

- desired range
- warning range/threshold
- critical threshold where useful
- data freshness requirements
- optional evaluation duration

This prevents UI and automation from independently inventing thresholds.

## Example recipe

```yaml
name: bibb-lettuce-standard
version: 1
crop: lettuce
variety: bibb

stages:
  - key: seedling
    transition:
      guidance_days: 10
    targets:
      air_temperature_c:
        min: 20
        max: 24
      humidity_percent:
        min: 55
        max: 75
      photoperiod_hours: 16

  - key: vegetative
    targets:
      air_temperature_c:
        min: 18
        max: 24
      water_temperature_c:
        max: 22
      ph:
        min: 5.8
        max: 6.2
      ec_ms_cm:
        min: 1.2
        max: 1.6
      photoperiod_hours: 16
```

The real recipe values must be agronomically reviewed before being presented as production guidance. Seed/demo data should be labeled accordingly.

## Stage transitions

V0 supports:

- manual transition
- suggested date based on guidance duration

Later versions may support measurement or observation-based transition recommendations, but automatic stage progression should not be introduced without a concrete need.

## Recipe assignment

When a grow cycle starts with a recipe, bind it to a specific immutable recipe version. Future edits produce a new version and do not silently alter the active grow's historical reference.

## Setpoint evaluation

The current grow stage provides context for telemetry status.

Instead of displaying only:

```text
EC = 1.09 mS/cm
```

show:

```text
EC = 1.09 mS/cm
Target = 1.20–1.60
LOW
```

The same evaluated state feeds dashboard badges, 3D tooltips, alerts, and later automation.

## Grow timeline

A grow timeline merges meaningful events such as:

```text
seeded
stage changed
transplanted
reservoir refilled
nutrients added
observation recorded
photo taken
sensor calibrated
alert opened
alert resolved
maintenance performed
harvested
```

Charts can overlay timeline markers without storing every sensor measurement as an event.

## Comparison

Once multiple completed cycles exist, comparison should support normalized metrics such as:

```text
days to harvest
harvest mass
mass per plant
water used
nutrient used
energy used where available
time outside recipe target
number/duration of critical alerts
```

Do not imply causation from correlation. The system can surface patterns and comparisons without claiming unvalidated agronomic conclusions.
