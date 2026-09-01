import type { FarmData, Measurement, UUID } from "../domain/model";

const id = (value: number): UUID => `01990a20-6a00-7000-8000-${String(value).padStart(12, "0")}`;
const at = (hour: number, minute = 0) => `2026-09-01T${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}:00.000Z`;

export function pilotData(): FarmData {
  const measurements: Measurement[] = [];
  const series = [
    { channel: 31, base: 22.4, unit: "degC" },
    { channel: 32, base: 68, unit: "%RH" },
    { channel: 33, base: 20.1, unit: "degC" },
    { channel: 34, base: 72, unit: "%" },
  ];
  series.forEach(({ channel, base, unit }, seriesIndex) => {
    for (let point = 0; point < 24; point += 1) {
      measurements.push({ id: id(1000 + seriesIndex * 100 + point), channel_id: id(channel), observed_at: at(0 + point), received_at: at(0 + point), value: Number((base + Math.sin(point / 3) * (seriesIndex === 1 ? 4 : 1.2)).toFixed(2)), unit, quality: "good", sequence: point + 1, source_device_id: id(20) });
    }
  });

  return {
    facilities: [{ id: id(1), name: "Home Indoor Farm", timezone: "Asia/Dubai" }],
    zones: [{ id: id(2), facility_id: id(1), name: "Grow Room", type: "room" }, { id: id(3), facility_id: id(1), parent_zone_id: id(2), name: "Tent 01 · 3 × 3 ft", type: "tent" }],
    reservoirs: [{ id: id(4), zone_id: id(3), name: "DWC Reservoir 01", nominal_capacity_l: 30, working_volume_l: 21.6, level_percent: 72 }],
    crops: [{ id: id(5), code: "lettuce", name: "Lettuce" }],
    varieties: [{ id: id(6), crop_id: id(5), code: "bibb", name: "Bibb Lettuce" }],
    grow_cycles: [{ id: id(7), facility_id: id(1), crop_id: id(5), variety_id: id(6), recipe_version_id: id(10), zone_ids: [id(3)], reservoir_ids: [id(4)], name: "Bibb Lettuce · Cycle 01", status: "active", stage_key: "vegetative", planned_start: "2026-08-10", actual_start: "2026-08-10T06:00:00.000Z", plant_count: 4, notes: "Pilot example data — targets require agronomic review." }],
    plant_positions: [1, 2, 3, 4].map((position) => ({ id: id(40 + position), zone_id: id(3), grow_cycle_id: id(7), code: `P${position}`, occupied: true, health: position === 3 ? "attention" : "normal" })),
    recipes: [{ id: id(9), crop_id: id(5), variety_id: id(6), name: "Bibb Lettuce Standard" }],
    recipe_versions: [{ id: id(10), recipe_id: id(9), version: 1, status: "published", published_at: "2026-08-01T00:00:00.000Z" }],
    recipe_stages: [{ id: id(11), recipe_version_id: id(10), key: "seedling", name: "Seedling", sort_order: 1, guidance_days: 10 }, { id: id(12), recipe_version_id: id(10), key: "vegetative", name: "Vegetative", sort_order: 2, guidance_days: 24 }, { id: id(13), recipe_version_id: id(10), key: "harvest", name: "Harvest window", sort_order: 3, guidance_days: 5 }],
    setpoints: [
      { id: id(14), stage_id: id(12), channel_key: "air.temperature", unit: "degC", minimum: 18, maximum: 24, stale_after_seconds: 180 },
      { id: id(15), stage_id: id(12), channel_key: "air.humidity", unit: "%RH", minimum: 55, maximum: 75, stale_after_seconds: 180 },
      { id: id(16), stage_id: id(12), channel_key: "water.temperature", unit: "degC", maximum: 22, stale_after_seconds: 180 },
      { id: id(17), stage_id: id(12), channel_key: "water.level", unit: "%", minimum: 35, maximum: 100, stale_after_seconds: 90 },
    ],
    devices: [
      { id: id(20), zone_id: id(3), name: "ESP32 Tent Controller", type: "controller", online: true, simulated: true, last_heartbeat: at(23, 59), firmware_version: "0.1.0-sim", active_config_version: "pilot-v1" },
      { id: id(21), zone_id: id(3), name: "240 W LED", type: "light", online: true, simulated: true, state: true, output_percent: 100, last_heartbeat: at(23, 59), firmware_version: "relay-v1", active_config_version: "pilot-v1" },
      { id: id(22), zone_id: id(3), name: "Circulation Fan", type: "fan", online: true, simulated: true, state: true, output_percent: 47, last_heartbeat: at(23, 59), firmware_version: "pwm-v1", active_config_version: "pilot-v1" },
      { id: id(23), zone_id: id(3), name: "Air Pump", type: "air_pump", online: true, simulated: true, state: true, output_percent: 100, last_heartbeat: at(23, 59), firmware_version: "relay-v1", active_config_version: "pilot-v1" },
    ],
    channels: [
      { id: id(31), device_id: id(20), entity_type: "zone", entity_id: id(3), key: "air.temperature", name: "Air temperature", kind: "measurement", value_type: "number", unit: "degC", dimension: "temperature", minimum: -10, maximum: 60, stale_after_seconds: 180 },
      { id: id(32), device_id: id(20), entity_type: "zone", entity_id: id(3), key: "air.humidity", name: "Relative humidity", kind: "measurement", value_type: "number", unit: "%RH", dimension: "relative_humidity", minimum: 0, maximum: 100, stale_after_seconds: 180 },
      { id: id(33), device_id: id(20), entity_type: "reservoir", entity_id: id(4), key: "water.temperature", name: "Water temperature", kind: "measurement", value_type: "number", unit: "degC", dimension: "temperature", minimum: 0, maximum: 45, stale_after_seconds: 180 },
      { id: id(34), device_id: id(20), entity_type: "reservoir", entity_id: id(4), key: "water.level", name: "Water level", kind: "measurement", value_type: "number", unit: "%", dimension: "ratio", minimum: 0, maximum: 100, stale_after_seconds: 90 },
      { id: id(35), device_id: id(21), entity_type: "device", entity_id: id(21), key: "light.state.command", name: "LED command", kind: "command", value_type: "boolean", safe_minimum: 0, safe_maximum: 1, stale_after_seconds: 300 },
      { id: id(36), device_id: id(22), entity_type: "device", entity_id: id(22), key: "fan.speed.command", name: "Fan output", kind: "command", value_type: "number", unit: "%", safe_minimum: 25, safe_maximum: 100, stale_after_seconds: 300 },
      { id: id(37), device_id: id(23), entity_type: "device", entity_id: id(23), key: "air_pump.state.command", name: "Air pump command", kind: "command", value_type: "boolean", safe_minimum: 1, safe_maximum: 1, stale_after_seconds: 300 },
    ],
    channel_bindings: [31, 32, 33, 34, 35, 36, 37].map((channel, index) => ({ id: id(60 + index), channel_id: id(channel), device_id: channel >= 35 ? id(channel - 14) : id(20), valid_from: "2026-08-01T00:00:00.000Z" })),
    measurements,
    events: [
      { id: id(70), type: "crop.seeded", occurred_at: "2026-08-10T06:00:00.000Z", actor: "Demo operator", entity_type: "grow_cycle", entity_id: id(7), summary: "Four Bibb lettuce positions seeded" },
      { id: id(71), type: "crop.transplanted", occurred_at: "2026-08-20T06:00:00.000Z", actor: "Demo operator", entity_type: "grow_cycle", entity_id: id(7), summary: "Cohort moved to DWC reservoir" },
      { id: id(72), type: "reservoir.filled", occurred_at: "2026-08-20T06:20:00.000Z", actor: "Demo operator", entity_type: "reservoir", entity_id: id(4), summary: "Reservoir filled to working volume" },
    ],
    event_quantities: [{ id: id(73), event_id: id(72), value: 24, unit: "L", material: "Water" }],
    observations: [{ id: id(74), grow_cycle_id: id(7), target_type: "plant_position", target_id: id(43), category: "leaf", severity: "warning", notes: "P3 outer leaf shows mild edge curl; monitor before changing targets.", observed_at: "2026-08-31T09:30:00.000Z", media_ids: [] }],
    inventory_items: [{ id: id(80), facility_id: id(1), name: "Base nutrient A", unit: "mL", reorder_level: 250 }, { id: id(81), facility_id: id(1), name: "Base nutrient B", unit: "mL", reorder_level: 250 }, { id: id(82), facility_id: id(1), name: "Lettuce seeds", unit: "count", reorder_level: 12 }],
    inventory_adjustments: [{ id: id(83), item_id: id(80), occurred_at: "2026-08-01T10:00:00.000Z", quantity: 1000, unit: "mL", reason: "purchase" }, { id: id(84), item_id: id(80), occurred_at: "2026-08-20T06:30:00.000Z", quantity: -24, unit: "mL", reason: "use" }, { id: id(85), item_id: id(81), occurred_at: "2026-08-01T10:00:00.000Z", quantity: 1000, unit: "mL", reason: "purchase" }, { id: id(86), item_id: id(82), occurred_at: "2026-08-01T10:00:00.000Z", quantity: 50, unit: "count", reason: "purchase" }, { id: id(87), item_id: id(82), occurred_at: "2026-08-10T06:00:00.000Z", quantity: -4, unit: "count", reason: "use" }],
    harvests: [],
    alerts: [{ id: id(90), definition_key: "water_temperature_high", entity_type: "reservoir", entity_id: id(4), severity: "warning", status: "open", title: "Water temperature above target", detail: "23.2 °C exceeded the 22 °C stage target for 12 minutes.", opened_at: "2026-09-01T09:42:00.000Z" }],
    commands: [{ id: id(91), target_channel_id: id(36), command_type: "set_percent", value: 47, reason: "pilot baseline", status: "applied", requested_at: "2026-09-01T09:30:00.000Z", updated_at: "2026-09-01T09:30:01.000Z", simulated: true }],
    automation_rules: [{ id: id(92), zone_id: id(3), name: "High water temperature warning", mode: "observe", enabled: true, trigger: "water.temperature > 22 °C for 10 min", action: "Open warning alert", cooldown_minutes: 30, last_evaluated_at: "2026-09-01T09:42:00.000Z" }, { id: id(93), zone_id: id(3), name: "Essential fan minimum", mode: "simulate", enabled: true, trigger: "Always", action: "Maintain fan ≥ 35%", cooldown_minutes: 0, last_evaluated_at: "2026-09-01T09:42:00.000Z" }],
    scene_layouts: [{ id: id(95), facility_id: id(1), name: "Pilot tent", camera_position: [7, 6, 8], entities: [
      { entity_type: "zone", entity_id: id(3), profile: "zone", position: [0, 1.5, 0], scale: [4, 3, 4] },
      { entity_type: "reservoir", entity_id: id(4), profile: "reservoir", position: [0, 0.6, 0], scale: [2.5, 1, 2.5] },
      { entity_type: "device", entity_id: id(21), profile: "light", position: [0, 3.1, 0], scale: [2.4, 0.15, 1.4] },
      { entity_type: "device", entity_id: id(22), profile: "fan", position: [-1.65, 2.15, 0], scale: [0.45, 0.45, 0.35] },
      ...[1, 2, 3, 4].map((position, index) => ({ entity_type: "plant_position" as const, entity_id: id(40 + position), profile: "plant", position: [index % 2 === 0 ? -0.65 : 0.65, 1.3, index < 2 ? -0.65 : 0.65] as [number, number, number], scale: [0.75, 0.75, 0.75] as [number, number, number] })),
    ] }],
  };
}
