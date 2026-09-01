import type { SceneEntity, SceneLayout } from "../domain/model";

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
};

export function actionsForProfile(profile: string): string[] {
  return profileActions[profile] ?? ["Inspect", "History"];
}
