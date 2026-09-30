import type { components } from "../lib/api/generated/schema";
import type { Device, FarmData, IntegrationProvider } from "../domain/model";

export type IntegrationStatus = components["schemas"]["IntegrationStatus"];
export type IntegrationCapability = components["schemas"]["IntegrationCapability"];
export type DiscoveredDevice = components["schemas"]["DiscoveredDevice"];
export type AdoptRequest = components["schemas"]["AdoptRequest"];
export type AdoptChannel = components["schemas"]["AdoptChannel"];

/** The server calls the frontend needs for the Integrations screen. Only the server runtime provides it. */
export interface IntegrationClient {
  listIntegrations(): Promise<IntegrationStatus[]>;
  listDiscovered(provider: IntegrationProvider): Promise<DiscoveredDevice[]>;
  adoptDevice(provider: IntegrationProvider, externalId: string, request: AdoptRequest): Promise<void>;
  permitJoin(provider: IntegrationProvider, seconds: number): Promise<string>;
}

export const providerLabels: Record<IntegrationProvider, string> = {
  zigbee2mqtt: "Zigbee",
  matter: "Matter",
  home_assistant: "Home Assistant",
};

export const providerDescriptions: Record<IntegrationProvider, string> = {
  zigbee2mqtt: "Zigbee sensors, plugs and lights through a Zigbee2MQTT bridge.",
  matter: "Matter devices through the Open Home Foundation Matter Server.",
  home_assistant: "Entities from a Home Assistant instance.",
};

const unitLabels: Record<string, string> = { degC: "°C", "%RH": "% RH", ug_m3: "µg/m³", "ug/m3": "µg/m³", "1": "" };

/** Canonical units as people read them; on/off channels have no visible unit. */
export function displayUnit(unit?: string): string {
  if (!unit || unit === "bool") return "";
  return unitLabels[unit] ?? unit;
}

/** One capability's row in the adopt form. */
export interface CapabilitySelection {
  capabilityKey: string;
  selected: boolean;
  key: string;
  name: string;
  /** Measurements describe the zone by default so recipe setpoints and alerts can see them. */
  entity: "zone" | "device";
  safeMinimum?: number;
  safeMaximum?: number;
}

export interface AdoptDraft {
  zoneId: string;
  name: string;
  type: Device["type"];
  acknowledgeNoEdgeFailsafe: boolean;
  selections: CapabilitySelection[];
}

/** Everything except diagnostics (battery, link quality) starts ticked. */
export function defaultDraft(device: DiscoveredDevice, zoneId: string): AdoptDraft {
  return {
    zoneId,
    name: device.name,
    type: device.suggested_type,
    acknowledgeNoEdgeFailsafe: false,
    selections: device.capabilities.map((capability) => ({
      capabilityKey: capability.key,
      selected: !capability.diagnostic,
      key: capability.suggested_channel_key,
      // Channel names appear alone in command history and charts, so they carry the device name.
      name: `${device.name} ${capability.label.toLowerCase()}`,
      entity: capability.kind === "measurement" ? "zone" : "device",
      safeMinimum: capability.kind === "command" ? capability.minimum : undefined,
      safeMaximum: capability.kind === "command" ? capability.maximum : undefined,
    })),
  };
}

export function selectsCommand(device: DiscoveredDevice, draft: AdoptDraft): boolean {
  return draft.selections.some((selection) => selection.selected && device.capabilities.find((capability) => capability.key === selection.capabilityKey)?.kind === "command");
}

/** Problems the server would refuse, reported before submitting. */
export function draftProblems(device: DiscoveredDevice, draft: AdoptDraft, data: FarmData): string[] {
  const problems: string[] = [];
  const chosen = draft.selections.filter((selection) => selection.selected);
  if (!draft.zoneId) problems.push("Choose a zone.");
  if (chosen.length === 0) problems.push("Choose at least one reading or output.");
  const existingKeys = new Set(data.channels.map((channel) => channel.key)), seen = new Set<string>();
  for (const selection of chosen) {
    const key = selection.key.trim();
    if (!key) problems.push(`${selection.name} needs a channel key.`);
    else if (existingKeys.has(key)) problems.push(`Channel key “${key}” is already used by another channel.`);
    else if (seen.has(key)) problems.push(`Channel key “${key}” is used twice.`);
    seen.add(key);
    const capability = device.capabilities.find((entry) => entry.key === selection.capabilityKey);
    if (capability?.kind === "command") {
      const minimum = capability.minimum ?? 0, maximum = capability.maximum ?? 100;
      const low = selection.safeMinimum ?? minimum, high = selection.safeMaximum ?? maximum;
      if (low < minimum || high > maximum || low > high) problems.push(`${selection.name}: the safe range must lie within ${minimum}–${maximum}.`);
    }
  }
  if (selectsCommand(device, draft) && !draft.acknowledgeNoEdgeFailsafe) problems.push("Confirm that controllable outputs have no controller-side failsafe.");
  return problems;
}

export function buildAdoptRequest(device: DiscoveredDevice, draft: AdoptDraft): AdoptRequest {
  return {
    zone_id: draft.zoneId,
    name: draft.name.trim() || device.name,
    type: draft.type,
    acknowledge_no_edge_failsafe: selectsCommand(device, draft) ? draft.acknowledgeNoEdgeFailsafe : false,
    channels: draft.selections.filter((selection) => selection.selected).map((selection) => {
      const capability = device.capabilities.find((entry) => entry.key === selection.capabilityKey);
      const channel: AdoptChannel = { capability_key: selection.capabilityKey, key: selection.key.trim(), name: selection.name.trim() || undefined };
      if (selection.entity === "zone") { channel.entity_type = "zone"; channel.entity_id = draft.zoneId; }
      if (capability?.kind === "command") { channel.safe_minimum = selection.safeMinimum; channel.safe_maximum = selection.safeMaximum; }
      return channel;
    }),
  };
}

/** Seconds left in a join window, or 0 when closed. */
export function permitJoinRemaining(status: IntegrationStatus | undefined, now = Date.now()): number {
  if (!status?.permit_join_until) return 0;
  return Math.max(0, Math.ceil((new Date(status.permit_join_until).getTime() - now) / 1000));
}
