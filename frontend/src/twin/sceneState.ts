import type { FarmData, SceneEntity, SceneLayout } from "../domain/model";

export const entityKey = (entityType: string, entityId: string) => `${entityType}:${entityId}`;

export function buildSceneIndex(layout: SceneLayout): Map<string, SceneEntity> {
  return new Map(layout.entities.map((binding) => [entityKey(binding.entity_type, binding.entity_id), binding]));
}

const profileActions: Record<string, string[]> = {
  sensor: ["Inspect", "History", "Calibrate", "Alerts", "Configure"],
  plant: ["Inspect", "Observe", "Photo", "History", "Harvest"],
  fan: ["Inspect", "Set output", "Override", "History", "Maintenance"],
  light: ["Inspect", "Set state", "Override", "History", "Maintenance"],
  reservoir: ["Inspect", "Chemistry", "Add input", "Refill", "History"],
  zone: ["Inspect", "Conditions", "Grow cycles", "History"],
  controller: ["Inspect", "History", "Configure"],
  air_pump: ["Inspect", "Set state", "History", "Maintenance"],
  hydroponic_tower: ["Inspect", "Conditions", "Grow cycles", "History"],
};

export function actionsForProfile(profile: string): string[] {
  return profileActions[profile] ?? ["Inspect", "History"];
}

/** Supply missing pilot hardware visuals for saved layouts without changing farm data. */
export function sceneBindings(data: FarmData): SceneEntity[] {
  const entities = data.scene_layouts[0]?.entities ?? [];
  const keys = new Set(entities.map((entry) => entityKey(entry.entity_type, entry.entity_id)));
  const extra: SceneEntity[] = [];
  for (const device of data.devices) {
    if (keys.has(entityKey("device", device.id))) continue;
    if (device.type !== "controller" && device.type !== "air_pump") continue;
    const zone = entities.find((entry) => entry.entity_type === "zone" && entry.entity_id === device.zone_id && entry.profile === "zone");
    if (!zone) continue;
    const offset = device.type === "controller" ? [0.34, 0.07, -0.44] : [0.37, -0.43, 0.06];
    const size = (device.type === "controller" ? 0.42 : 0.48) * Math.min(zone.scale[0] / 4, zone.scale[1] / 3, zone.scale[2] / 4);
    extra.push({
      entity_type: "device", entity_id: device.id, profile: device.type,
      position: zone.position.map((value, axis) => value + offset[axis] * zone.scale[axis]) as [number, number, number],
      scale: [size, size, size],
    });
  }
  return [...entities, ...extra];
}
