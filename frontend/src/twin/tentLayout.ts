export type Vec3 = [number, number, number];
export type Environment = "tent" | "outdoor";
export type PropValue = string | number;
export interface LayoutItem { id: string; kind: string; size: Vec3; position: Vec3; rotation: number; props: Record<string, PropValue> }
/** `tent` holds the bounds of the growing space: a tent's inside, or an outdoor plot's footprint and height limit. */
export interface TentLayout { version: 2; name: string; environment: Environment; tent: Vec3; grid: number; items: LayoutItem[] }

export const catalog: { kind: string; name: string; size: Vec3 }[] = [
  { kind: "pot_nursery", name: "Nursery pot", size: [.3, .3, .3] },
  { kind: "pot_fabric", name: "Fabric pot", size: [.38, .35, .3] },
  { kind: "pot_ceramic", name: "Ceramic pot", size: [.32, .3, .32] },
  { kind: "led_panel", name: "LED panel", size: [.58, .1, .38] },
  { kind: "led_bar", name: "LED linear bar", size: [.58, .1, .07] },
  { kind: "led_multi_bar", name: "LED multi-bar", size: [.58, .1, .46] },
  { kind: "fan_clip", name: "Clip fan", size: [.34, .49, .22] },
  { kind: "fan_inline", name: "Inline fan", size: [.34, .53, .29] },
  { kind: "humidifier_ultrasonic", name: "Ultrasonic humidifier", size: [.25, .37, .24] },
  { kind: "humidifier_evaporative", name: "Evaporative humidifier", size: [.28, .39, .26] },
  { kind: "irrigation_drip", name: "Drip irrigation", size: [.9, .34, .53] },
  { kind: "irrigation_ring", name: "Ring irrigation", size: [.94, .34, .65] },
  { kind: "tower", name: "Hydroponic tower + sensors", size: [.78, 2.46, .7] },
  { kind: "reservoir", name: "DWC reservoir", size: [.7, .45, .65] },
  { kind: "plant", name: "Lettuce plant", size: [.3, .32, .3] },
  { kind: "controller", name: "Controller + air sensor", size: [.22, .28, .08] },
  { kind: "air_pump", name: "Air pump", size: [.22, .1, .22] },
  { kind: "sensor_ph", name: "pH probe", size: [.046, .34, .046] },
  { kind: "sensor_ec", name: "EC probe", size: [.046, .32, .046] },
  { kind: "sensor_temp", name: "Water temperature probe", size: [.046, .32, .046] },
  { kind: "sensor_level", name: "Water level sensor", size: [.06, .32, .06] },
  { kind: "sensor_air", name: "Humidity / air temperature", size: [.30, .15, .1] },
  { kind: "sensor_par", name: "PAR / light sensor", size: [.22, .055, .065] },
];
export const itemName = (kind: string) => catalog.find((entry) => entry.kind === kind)?.name ?? kind;
/** The object's own name if the user gave one, otherwise its catalog name. */
export const displayName = (item: LayoutItem) => (typeof item.props.label === "string" && item.props.label.trim()) || itemName(item.kind);

// ---- Per-kind parameters ----------------------------------------------------------------

export type PropSpec =
  | { key: string; label: string; type: "number"; unit?: string; min: number; max: number; step: number; default: number }
  | { key: string; label: string; type: "select"; options: [string, string][]; default: string }
  | { key: string; label: string; type: "text"; maxLength: number; default: string };

const num = (key: string, label: string, unit: string | undefined, min: number, max: number, step: number, value: number): PropSpec => ({ key, label, type: "number", unit, min, max, step, default: value });
const watts = (value: number, max = 2000) => num("watts", "Power draw", "W", 0, max, 1, value);
const LABEL: PropSpec = { key: "label", label: "Name", type: "text", maxLength: 60, default: "" };
const SPECTRA: [string, string][] = [["full", "Full spectrum"], ["veg", "Vegetative (blue-rich)"], ["bloom", "Bloom (red-rich)"], ["white", "White 3000–4000 K"]];
const MEDIA: [string, string][] = [["soil", "Soil"], ["coco", "Coco coir"], ["peat", "Peat mix"], ["perlite", "Perlite"], ["clay", "Clay pebbles"], ["rockwool", "Rockwool"]];
const led = (w: number, diodes: number, extra: PropSpec[] = []): PropSpec[] => [watts(w), num("diodes", "Diode count", undefined, 0, 10000, 1, diodes), ...extra, num("output", "Dimmer", "%", 0, 100, 1, 70), { key: "spectrum", label: "Spectrum", type: "select", options: SPECTRA, default: "full" }, num("photoperiod", "Light hours per day", "h", 0, 24, .5, 18)];
const pot = (litres: number): PropSpec[] => [num("volume", "Volume", "L", .1, 400, .1, litres), { key: "medium", label: "Medium", type: "select", options: MEDIA, default: "soil" }, num("fill", "Fill level", "%", 0, 100, 1, 80)];
const fan = (w: number, airflow: number): PropSpec[] => [watts(w, 1000), num("airflow", "Airflow", "m³/h", 0, 10000, 1, airflow), num("speed", "Speed", "%", 0, 100, 1, 60)];
const humidifier = (w: number, rate: number): PropSpec[] => [watts(w, 500), num("rate", "Mist output", "mL/h", 0, 10000, 10, rate), num("tank", "Tank", "L", 0, 200, .1, 4)];
const irrigation = (emitters: number): PropSpec[] => [num("emitters", "Emitters", undefined, 1, 500, 1, emitters), num("flow", "Flow per emitter", "L/h", 0, 100, .1, 2), watts(15, 500)];

const SPECS: Record<string, PropSpec[]> = {
  pot_nursery: pot(11), pot_fabric: pot(19), pot_ceramic: pot(15),
  led_panel: led(240, 576), led_bar: led(65, 96), led_multi_bar: led(480, 1152, [num("bars", "Bars", undefined, 1, 16, 1, 6)]),
  fan_clip: fan(20, 150), fan_inline: fan(60, 400),
  humidifier_ultrasonic: humidifier(30, 300), humidifier_evaporative: humidifier(20, 250),
  irrigation_drip: irrigation(8), irrigation_ring: irrigation(4),
  tower: [num("levels", "Levels", undefined, 3, 8, 1, 6), num("sites", "Sites per level", undefined, 1, 12, 1, 3), watts(15, 500)],
  reservoir: [num("volume", "Volume", "L", 1, 5000, 1, 40), num("fill", "Fill level", "%", 0, 100, 1, 70)],
  plant: [{ key: "cultivar", label: "Cultivar", type: "text", maxLength: 60, default: "Bibb lettuce" }, { key: "stage", label: "Stage", type: "select", options: [["seedling", "Seedling"], ["veg", "Vegetative"], ["flower", "Flowering"], ["harvest", "Ready to harvest"]], default: "veg" }],
  controller: [watts(5, 100)],
  air_pump: [watts(5, 200), num("airflow", "Air output", "L/min", 0, 500, .5, 4)],
};

/** Every parameter an object of this kind carries, starting with its editable name. */
export const specsFor = (kind: string): PropSpec[] => [LABEL, ...(SPECS[kind] ?? [])];
export const defaultProps = (kind: string): Record<string, PropValue> => Object.fromEntries(specsFor(kind).map((spec) => [spec.key, spec.default]));

/** Fills missing parameters with defaults, drops unknown ones, and rejects out-of-range values. */
export function normalizeProps(kind: string, props: Record<string, unknown> | undefined): Record<string, PropValue> {
  const result: Record<string, PropValue> = {};
  for (const spec of specsFor(kind)) {
    const value = props?.[spec.key];
    if (value === undefined) { result[spec.key] = spec.default; continue; }
    if (spec.type === "number") {
      if (typeof value !== "number" || !Number.isFinite(value) || value < spec.min || value > spec.max) throw new Error(`${spec.label} must be between ${spec.min} and ${spec.max}${spec.unit ? ` ${spec.unit}` : ""}.`);
      result[spec.key] = Math.round(Math.round(value / spec.step) * spec.step * 1e6) / 1e6;
    } else if (spec.type === "select") {
      if (typeof value !== "string" || !spec.options.some(([option]) => option === value)) throw new Error(`${spec.label} has an unknown option.`);
      result[spec.key] = value;
    } else {
      if (typeof value !== "string") throw new Error(`${spec.label} must be text.`);
      result[spec.key] = value.slice(0, spec.maxLength);
    }
  }
  return result;
}

// ---- Geometry and placement -------------------------------------------------------------

export const gridOptions = [.05, .1, .25, .5, 1];
export const spaceLimits: Record<Environment, { min: number; max: number; maxHeight: number }> = { tent: { min: .5, max: 10, maxHeight: 10 }, outdoor: { min: .5, max: 30, maxHeight: 10 } };
export const initialTentLayout = (): TentLayout => ({ version: 2, name: "My grow", environment: "tent", tent: [2.4, 2.6, 2.4], grid: .1, items: [] });
const clean = (n: number) => Math.round(n * 1e6) / 1e6 || 0;
export function footprint(item: Pick<LayoutItem, "size" | "rotation">): Vec3 {
  return item.rotation % 180 === 0 ? [...item.size] : [item.size[2], item.size[1], item.size[0]];
}
export function fits(item: LayoutItem, tent: Vec3): boolean {
  return footprint(item).every((size, axis) => size <= tent[axis] - .04 + 1e-6);
}
export function placeItem(item: LayoutItem, tent: Vec3, grid: number): LayoutItem {
  const size = footprint(item);
  const min: Vec3 = [-tent[0] / 2 + size[0] / 2 + .02, .02, -tent[2] / 2 + size[2] / 2 + .02];
  const max: Vec3 = [tent[0] / 2 - size[0] / 2 - .02, tent[1] - size[1] - .02, tent[2] / 2 - size[2] / 2 - .02];
  // Height is measured from the floor. The 2 cm wall clearance does not raise floor items.
  min[1] = 0;
  const position = item.position.map((value, axis) => {
    const low = Math.ceil((min[axis] - 1e-8) / grid) * grid;
    const high = Math.floor((max[axis] + 1e-8) / grid) * grid;
    return clean(low <= high ? Math.max(low, Math.min(high, Math.round(value / grid) * grid)) : (min[axis] + max[axis]) / 2);
  }) as Vec3;
  return { ...item, position };
}
export function resizeTent(layout: TentLayout, tent: Vec3, environment: Environment = layout.environment): TentLayout {
  const limits = spaceLimits[environment];
  const where = environment === "tent" ? "Tent" : "Plot";
  if (![tent[0], tent[2]].every((v) => Number.isFinite(v) && v >= limits.min && v <= limits.max) || !(Number.isFinite(tent[1]) && tent[1] >= limits.min && tent[1] <= limits.maxHeight)) throw new Error(`${where} width and depth must be ${limits.min}–${limits.max} m and height ${limits.min}–${limits.maxHeight} m.`);
  if (layout.items.some((item) => !fits(item, tent))) throw new Error(`An object is too large for this ${where.toLowerCase()}. Resize or remove it first.`);
  return { ...layout, environment, tent, items: layout.items.map((item) => placeItem(item, tent, layout.grid)) };
}
export function updateItem(layout: TentLayout, item: LayoutItem): TentLayout {
  if (!item.size.every((v) => Number.isFinite(v) && v >= .01 && v <= spaceLimits.outdoor.max) || !item.position.every(Number.isFinite)) throw new Error("Enter finite dimensions of at least 1 cm.");
  if (!fits(item, layout.tent)) throw new Error("This object is too large for the space at this rotation.");
  const next = { ...item, props: normalizeProps(item.kind, item.props) };
  return { ...layout, items: layout.items.map((existing) => existing.id === item.id ? placeItem(next, layout.tent, layout.grid) : existing) };
}
function overlapsAny(layout: TentLayout, placed: LayoutItem): boolean {
  const a = footprint(placed);
  return layout.items.some((other) => {
    const b = footprint(other);
    return Math.abs(placed.position[0] - other.position[0]) < (a[0] + b[0]) / 2 + .02 && Math.abs(placed.position[2] - other.position[2]) < (a[2] + b[2]) / 2 + .02 && placed.position[1] < other.position[1] + b[1] && placed.position[1] + a[1] > other.position[1];
  });
}
/** Finds a vacant snapped location without moving existing objects, scanning from a corner or nearest to `near`. */
function withFreeSpot(layout: TentLayout, item: LayoutItem, near?: Vec3): TentLayout {
  if (layout.items.length >= 100) throw new Error("This layout supports up to 100 objects.");
  if (!fits(item, layout.tent)) throw new Error("Enlarge the space before adding this object.");
  const candidates: Vec3[] = [];
  for (let z = -layout.tent[2] / 2; z <= layout.tent[2] / 2; z += layout.grid) {
    for (let x = -layout.tent[0] / 2; x <= layout.tent[0] / 2; x += layout.grid) candidates.push([x, item.position[1], z]);
  }
  if (near) candidates.sort((a, b) => Math.hypot(a[0] - near[0], a[2] - near[2]) - Math.hypot(b[0] - near[0], b[2] - near[2]));
  for (const position of candidates) {
    const placed = placeItem({ ...item, position }, layout.tent, layout.grid);
    if (!overlapsAny(layout, placed)) return { ...layout, items: [...layout.items, placed] };
  }
  throw new Error("No free grid space. Move existing objects or enlarge the space.");
}
export function addItem(layout: TentLayout, kind: string, id: string): TentLayout {
  const entry = catalog.find((candidate) => candidate.kind === kind);
  if (!entry) throw new Error("Unknown equipment type.");
  return withFreeSpot(layout, { id, kind, size: [...entry.size], position: [0, kind.startsWith('led_') ? layout.tent[1] - .2 : 0, 0], rotation: 0, props: defaultProps(kind) });
}
export function duplicateItem(layout: TentLayout, sourceId: string, id: string): TentLayout {
  const source = layout.items.find((entry) => entry.id === sourceId);
  if (!source) throw new Error("Select an object to duplicate.");
  return withFreeSpot(layout, { ...source, id, size: [...source.size], position: [...source.position], props: { ...source.props } }, source.position);
}
export function removeItem(layout: TentLayout, id: string): TentLayout {
  return { ...layout, items: layout.items.filter((entry) => entry.id !== id) };
}

// ---- Persistence, export and import -----------------------------------------------------

/** Validates a stored or imported layout, upgrading version 1 layouts (tent only, no parameters). */
export function parseTentLayout(raw: string | null): TentLayout {
  if (!raw) return initialTentLayout();
  return validateLayout(JSON.parse(raw));
}
export function validateLayout(input: unknown): TentLayout {
  const value = input as Partial<Omit<TentLayout, "version" | "items">> & { version?: number; items?: Array<Partial<LayoutItem>> };
  const vector = (v: unknown): v is Vec3 => Array.isArray(v) && v.length === 3 && v.every((n) => typeof n === 'number' && Number.isFinite(n));
  if (!value || typeof value !== "object" || (value.version !== 1 && value.version !== 2)) throw new Error('Unsupported layout version');
  const environment: Environment = value.version === 1 ? "tent" : value.environment as Environment;
  if (environment !== "tent" && environment !== "outdoor") throw new Error('Invalid layout environment');
  if (!vector(value.tent) || typeof value.grid !== "number" || !gridOptions.includes(value.grid) || !Array.isArray(value.items) || value.items.length > 100) throw new Error('Invalid saved layout');
  const name = value.version === 1 ? initialTentLayout().name : value.name;
  if (typeof name !== "string" || name.length > 80) throw new Error('Invalid layout name');
  const ids = new Set<string>();
  const items = value.items.map((item): LayoutItem => {
    if (!item || typeof item.id !== 'string' || ids.has(item.id) || typeof item.kind !== "string" || !catalog.some((entry) => entry.kind === item.kind) || !vector(item.size) || item.size.some((n) => n < .01 || n > spaceLimits.outdoor.max) || !vector(item.position) || typeof item.rotation !== "number" || ![0, 90, 180, 270].includes(item.rotation)) throw new Error('Invalid saved object');
    ids.add(item.id);
    if (item.props !== undefined && (typeof item.props !== "object" || Array.isArray(item.props) || item.props === null)) throw new Error('Invalid saved object');
    return { id: item.id, kind: item.kind, size: item.size, position: item.position, rotation: item.rotation, props: normalizeProps(item.kind, item.props) };
  });
  return resizeTent({ version: 2, name, environment, tent: value.tent, grid: value.grid, items }, value.tent, environment);
}

export const LAYOUT_FILE_FORMAT = "grownerve.layout";
export function serializeLayoutFile(layout: TentLayout, now = new Date().toISOString()): string {
  return `${JSON.stringify({ format: LAYOUT_FILE_FORMAT, version: layout.version, exported_at: now, layout }, null, 2)}\n`;
}
/** Reads an exported layout file (or a bare layout object) and validates everything in it. */
export function parseLayoutFile(text: string): TentLayout {
  let input: unknown;
  try { input = JSON.parse(text); } catch { throw new Error("This file is not valid JSON."); }
  const envelope = input as { format?: unknown; layout?: unknown };
  if (envelope && typeof envelope === "object" && "format" in envelope) {
    if (envelope.format !== LAYOUT_FILE_FORMAT) throw new Error("This is not a GrowNerve layout file.");
    return validateLayout(envelope.layout);
  }
  return validateLayout(input);
}

// ---- Summary for replication ------------------------------------------------------------

export interface LayoutSummary { area: number; counts: { name: string; count: number }[]; lightWatts: number; lightDensity: number; totalWatts: number; lightKwhPerDay: number; leds: number }
export function layoutSummary(layout: TentLayout): LayoutSummary {
  const area = layout.tent[0] * layout.tent[2];
  const counts = new Map<string, number>();
  let lightWatts = 0, totalWatts = 0, lightKwhPerDay = 0, leds = 0;
  for (const item of layout.items) {
    counts.set(itemName(item.kind), (counts.get(itemName(item.kind)) ?? 0) + 1);
    const w = typeof item.props.watts === "number" ? item.props.watts : 0;
    totalWatts += w;
    if (item.kind.startsWith("led_")) {
      leds++;
      const draw = w * (Number(item.props.output) / 100);
      lightWatts += draw;
      lightKwhPerDay += draw * Number(item.props.photoperiod) / 1000;
    }
  }
  return { area, counts: [...counts].map(([name, count]) => ({ name, count })), lightWatts, lightDensity: area > 0 ? lightWatts / area : 0, totalWatts, lightKwhPerDay, leds };
}
