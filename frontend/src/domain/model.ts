export type UUID = string;
export type RuntimeMode = "browser" | "server";
export type Quality = "good" | "suspect" | "stale" | "calibrating" | "fault" | "unknown";
export type EntityType = "facility" | "zone" | "reservoir" | "grow_cycle" | "plant_position" | "device" | "channel" | "inventory_item";

export interface Facility { id: UUID; name: string; timezone: string }
export interface Zone { id: UUID; facility_id: UUID; parent_zone_id?: UUID; name: string; type: "room" | "tent" | "rack" | "level" }
export interface Reservoir { id: UUID; zone_id: UUID; name: string; nominal_capacity_l: number; working_volume_l: number; level_percent: number }
export interface Crop { id: UUID; code: string; name: string }
export interface Variety { id: UUID; crop_id: UUID; code: string; name: string }
export type GrowStatus = "planned" | "active" | "completed" | "abandoned";
export interface GrowCycle { id: UUID; facility_id: UUID; crop_id: UUID; variety_id: UUID; recipe_version_id: UUID; zone_ids: UUID[]; reservoir_ids: UUID[]; name: string; status: GrowStatus; stage_key: string; planned_start: string; actual_start?: string; completed_at?: string; plant_count: number; notes: string }
export interface PlantPosition { id: UUID; zone_id: UUID; grow_cycle_id?: UUID; code: string; occupied: boolean; health: "normal" | "attention" | "unknown" }
export interface Recipe { id: UUID; crop_id: UUID; variety_id?: UUID; name: string }
export interface RecipeVersion { id: UUID; recipe_id: UUID; version: number; status: "draft" | "published"; published_at?: string }
export interface RecipeStage { id: UUID; recipe_version_id: UUID; key: string; name: string; sort_order: number; guidance_days?: number }
export interface Setpoint { id: UUID; stage_id: UUID; channel_key: string; unit: string; minimum?: number; maximum?: number; warning_duration_minutes?: number; stale_after_seconds: number }
export interface Device { id: UUID; zone_id: UUID; name: string; type: "controller" | "light" | "fan" | "air_pump" | "sensor"; online: boolean; simulated: boolean; output_percent?: number; state?: boolean; last_heartbeat: string; firmware_version: string; active_config_version: string }
export interface Channel { id: UUID; device_id: UUID; entity_type: EntityType; entity_id: UUID; key: string; name: string; kind: "measurement" | "state" | "command" | "counter"; value_type: "number" | "boolean" | "enum"; unit?: string; dimension?: string; minimum?: number; maximum?: number; safe_minimum?: number; safe_maximum?: number; stale_after_seconds: number }
export interface ChannelBinding { id: UUID; channel_id: UUID; device_id: UUID; valid_from: string; valid_to?: string }
export interface Measurement { id?: UUID; channel_id: UUID; observed_at: string; received_at?: string; value: number; unit: string; quality: Quality; sequence?: number; source_device_id?: UUID }
export interface FarmEvent { id: UUID; type: string; occurred_at: string; actor: string; entity_type: EntityType; entity_id: UUID; summary: string; notes?: string }
export interface EventQuantity { id: UUID; event_id: UUID; value: number; unit: string; material?: string }
export interface Observation { id: UUID; grow_cycle_id: UUID; target_type: EntityType; target_id: UUID; category: string; severity: "info" | "warning" | "critical"; notes: string; observed_at: string; media_ids: UUID[] }
export interface InventoryItem { id: UUID; facility_id: UUID; name: string; unit: string; reorder_level: number }
export interface InventoryAdjustment { id: UUID; item_id: UUID; occurred_at: string; quantity: number; unit: string; reason: "purchase" | "use" | "correction" | "waste"; event_id?: UUID }
export interface Harvest { id: UUID; grow_cycle_id: UUID; harvested_at: string; mass_g: number; waste_g: number; quality_notes: string }
export type AlertStatus = "open" | "acknowledged" | "resolved";
export interface Alert { id: UUID; definition_key: string; entity_type: EntityType; entity_id: UUID; severity: "warning" | "critical"; status: AlertStatus; title: string; detail: string; opened_at: string; acknowledged_at?: string; acknowledged_by?: string; resolved_at?: string }
export type CommandStatus = "pending" | "published" | "acknowledged" | "applied" | "rejected" | "timed_out" | "cancelled";
export interface FarmCommand { id: UUID; target_channel_id: UUID; command_type: "set_boolean" | "set_percent"; value: number | boolean; reason: string; status: CommandStatus; requested_at: string; updated_at: string; simulated: boolean; reason_code?: string }
export interface AutomationRule { id: UUID; zone_id: UUID; name: string; mode: "observe" | "simulate"; enabled: boolean; trigger: string; action: string; cooldown_minutes: number; last_evaluated_at?: string }
export interface SceneEntity { entity_type: EntityType; entity_id: UUID; profile: string; position: [number, number, number]; scale: [number, number, number] }
export interface SceneLayout { id: UUID; facility_id: UUID; name: string; entities: SceneEntity[]; camera_position: [number, number, number] }
export interface MediaObject { id: UUID; mime_type: string; sha256: string; filename: string; data_base64: string }

export interface FarmData {
  facilities: Facility[];
  zones: Zone[];
  reservoirs: Reservoir[];
  crops: Crop[];
  varieties: Variety[];
  grow_cycles: GrowCycle[];
  plant_positions: PlantPosition[];
  recipes: Recipe[];
  recipe_versions: RecipeVersion[];
  recipe_stages: RecipeStage[];
  setpoints: Setpoint[];
  devices: Device[];
  channels: Channel[];
  channel_bindings: ChannelBinding[];
  measurements: Measurement[];
  events: FarmEvent[];
  event_quantities: EventQuantity[];
  observations: Observation[];
  inventory_items: InventoryItem[];
  inventory_adjustments: InventoryAdjustment[];
  harvests: Harvest[];
  alerts: Alert[];
  commands: FarmCommand[];
  automation_rules: AutomationRule[];
  scene_layouts: SceneLayout[];
}

export interface GrowNerveArchive {
  format: "grownerve";
  schema_version: 1;
  exported_at: string;
  app_version: string;
  export_id: UUID;
  data: FarmData;
  media: MediaObject[];
}

export const farmDataKeys = [
  "facilities", "zones", "reservoirs", "crops", "varieties", "grow_cycles", "plant_positions",
  "recipes", "recipe_versions", "recipe_stages", "setpoints", "devices", "channels", "channel_bindings",
  "measurements", "events", "event_quantities", "observations", "inventory_items", "inventory_adjustments",
  "harvests", "alerts", "commands", "automation_rules", "scene_layouts",
] as const satisfies readonly (keyof FarmData)[];

export const emptyFarmData = (): FarmData => Object.fromEntries(farmDataKeys.map((key) => [key, []])) as unknown as FarmData;
