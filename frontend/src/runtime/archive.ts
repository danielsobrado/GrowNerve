import { z } from "zod";
import { emptyFarmData, farmDataKeys, integrationProviders, type ArchiveLayout, type FarmData, type GrowNerveArchive, type MediaObject } from "../domain/model";
import { validateLayout } from "../twin/tentLayout";

const DEFAULT_LAYOUT_OWNER = "default";

const uuid = z.string().uuid();
const archiveEnvelope = z.object({
  format: z.literal("grownerve"),
  schema_version: z.number().int(),
  exported_at: z.string().datetime(),
  app_version: z.string().min(1),
  export_id: uuid,
  data: z.record(z.string(), z.array(z.unknown())),
  media: z.array(z.object({ id: uuid, mime_type: z.string(), sha256: z.string(), filename: z.string(), data_base64: z.string() })),
  layouts: z.array(z.object({ facility_id: z.string().min(1).max(64), layout: z.unknown() })).max(100).optional(),
});

function stableData(data: FarmData): FarmData {
  const result = emptyFarmData();
  for (const key of farmDataKeys) {
    (result[key] as Array<{ id?: string }>).push(...[...(data[key] as Array<{ id?: string }>)].sort((left, right) => (left.id ?? "").localeCompare(right.id ?? "")));
  }
  return result;
}

export function createArchive(data: FarmData, metadata: { now?: string; exportId?: string; media?: MediaObject[]; layouts?: ArchiveLayout[] } = {}): GrowNerveArchive {
  return {
    format: "grownerve",
    schema_version: 1,
    exported_at: metadata.now ?? new Date().toISOString(),
    app_version: "0.1.0",
    export_id: metadata.exportId ?? crypto.randomUUID(),
    data: stableData(structuredClone(data)),
    media: [...(metadata.media ?? [])].sort((left, right) => left.id.localeCompare(right.id)),
    layouts: structuredClone(metadata.layouts ?? []).sort((left, right) => left.facility_id.localeCompare(right.facility_id)),
  };
}

/** Each layout must belong to a facility in this archive (or the default owner) and pass full layout validation. */
function validateLayouts(entries: Array<{ facility_id: string; layout: unknown }>, data: FarmData): ArchiveLayout[] {
  const facilities = new Set(data.facilities.map((facility) => facility.id)), seen = new Set<string>();
  return entries.map(({ facility_id, layout }) => {
    if (facility_id !== DEFAULT_LAYOUT_OWNER && !facilities.has(facility_id)) throw new Error(`Layout references unknown facility ${facility_id}`);
    if (seen.has(facility_id)) throw new Error(`Archive contains two layouts for ${facility_id}`);
    seen.add(facility_id);
    try { return { facility_id, layout: validateLayout(layout) }; }
    catch (cause) { throw new Error(`Layout for ${facility_id} is invalid: ${cause instanceof Error ? cause.message : String(cause)}`); }
  });
}

function assertUniqueIds(data: FarmData) {
  for (const key of farmDataKeys) {
    const seen = new Set<string>();
    for (const record of data[key] as Array<{ id?: string }>) {
      if (!record.id || !uuid.safeParse(record.id).success) throw new Error(`${key} contains an invalid UUID`);
      if (seen.has(record.id)) throw new Error(`${key} contains duplicate ID ${record.id}`);
      seen.add(record.id);
    }
  }
}

function assertReferences(data: FarmData) {
  const ids = <T extends { id: string }>(values: T[]) => new Set(values.map((value) => value.id));
  const facilityIds = ids(data.facilities), zoneIds = ids(data.zones), cropIds = ids(data.crops), varietyIds = ids(data.varieties), recipeIds = ids(data.recipes), recipeVersionIds = ids(data.recipe_versions), stageIds = ids(data.recipe_stages), deviceIds = ids(data.devices), channelIds = ids(data.channels), growIds = ids(data.grow_cycles), eventIds = ids(data.events), inventoryIds = ids(data.inventory_items);
  for (const zone of data.zones) if (!facilityIds.has(zone.facility_id)) throw new Error(`Zone ${zone.id} references unknown facility ${zone.facility_id}`);
  for (const reservoir of data.reservoirs) if (!zoneIds.has(reservoir.zone_id)) throw new Error(`Reservoir ${reservoir.id} references unknown zone`);
  for (const variety of data.varieties) if (!cropIds.has(variety.crop_id)) throw new Error(`Variety ${variety.id} references unknown crop`);
  for (const grow of data.grow_cycles) if (!facilityIds.has(grow.facility_id) || !cropIds.has(grow.crop_id) || !varietyIds.has(grow.variety_id) || !recipeVersionIds.has(grow.recipe_version_id)) throw new Error(`Grow cycle ${grow.id} has an unknown reference`);
  for (const recipe of data.recipes) if (!cropIds.has(recipe.crop_id)) throw new Error(`Recipe ${recipe.id} references unknown crop`);
  for (const version of data.recipe_versions) if (!recipeIds.has(version.recipe_id)) throw new Error(`Recipe version ${version.id} references unknown recipe`);
  for (const stage of data.recipe_stages) if (!recipeVersionIds.has(stage.recipe_version_id)) throw new Error(`Recipe stage ${stage.id} references unknown version`);
  for (const setpoint of data.setpoints) if (!stageIds.has(setpoint.stage_id)) throw new Error(`Setpoint ${setpoint.id} references unknown stage`);
  for (const device of data.devices) if (!zoneIds.has(device.zone_id)) throw new Error(`Device ${device.id} references unknown zone`);
  for (const channel of data.channels) if (!deviceIds.has(channel.device_id)) throw new Error(`Channel ${channel.id} references unknown device`);
  for (const measurement of data.measurements) if (!channelIds.has(measurement.channel_id)) throw new Error("Measurement references unknown channel");
  for (const observation of data.observations) if (!growIds.has(observation.grow_cycle_id)) throw new Error(`Observation ${observation.id} references unknown grow`);
  for (const quantity of data.event_quantities) if (!eventIds.has(quantity.event_id)) throw new Error(`Event quantity ${quantity.id} references unknown event`);
  for (const adjustment of data.inventory_adjustments) if (!inventoryIds.has(adjustment.item_id)) throw new Error(`Inventory adjustment ${adjustment.id} references unknown item`);
}

/** Integration bindings are optional, but when present they must name a known provider and be unambiguous. */
function assertIntegrationBindings(data: FarmData) {
  const seen = new Set<string>();
  for (const device of data.devices) {
    const binding = device.integration as unknown;
    if (binding === undefined) continue;
    const { provider, external_id: externalId } = (binding ?? {}) as { provider?: unknown; external_id?: unknown };
    if (!integrationProviders.includes(provider as never)) throw new Error(`Device ${device.id} has an unknown integration provider`);
    if (typeof externalId !== "string" || !externalId || externalId.length > 256) throw new Error(`Device ${device.id} has an invalid integration identity`);
    const identity = JSON.stringify([provider, externalId]);
    if (seen.has(identity)) throw new Error(`Two devices are bound to ${String(provider)} device ${externalId}`);
    seen.add(identity);
  }
  for (const channel of data.channels) {
    const key = channel.integration_key as unknown;
    if (key !== undefined && (typeof key !== "string" || !key || key.length > 256)) throw new Error(`Channel ${channel.id} has an invalid integration key`);
  }
}

export function validateArchive(input: unknown): GrowNerveArchive {
  const parsed = archiveEnvelope.parse(input);
  if (parsed.schema_version !== 1) throw new Error(`Unsupported archive schema ${parsed.schema_version}`);
  for (const key of farmDataKeys) if (!Array.isArray(parsed.data[key])) throw new Error(`Archive is missing data.${key}`);
  const archive = parsed as unknown as GrowNerveArchive;
  assertUniqueIds(archive.data);
  assertReferences(archive.data);
  assertIntegrationBindings(archive.data);
  return { ...archive, layouts: validateLayouts(parsed.layouts ?? [], archive.data) };
}

export function serializeArchive(archive: GrowNerveArchive): string {
  return `${JSON.stringify(archive, null, 2)}\n`;
}
