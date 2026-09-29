export type Vec3 = [number, number, number];
export interface LayoutItem { id: string; kind: string; size: Vec3; position: Vec3; rotation: number }
export interface TentLayout { version: 1; tent: Vec3; grid: number; items: LayoutItem[] }
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
export const initialTentLayout = (): TentLayout => ({ version: 1, tent: [2.4, 2.6, 2.4], grid: .1, items: [] });
const clean = (n: number) => Math.round(n * 1e6) / 1e6 || 0;
export function footprint(item: LayoutItem): Vec3 {
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
export function resizeTent(layout: TentLayout, tent: Vec3): TentLayout {
  if (!tent.every((v) => Number.isFinite(v) && v >= .5 && v <= 10)) throw new Error("Tent dimensions must be 50–1000 cm.");
  if (layout.items.some((item) => !fits(item, tent))) throw new Error("An object is too large for this tent. Resize or remove it first.");
  return { ...layout, tent, items: layout.items.map((item) => placeItem(item, tent, layout.grid)) };
}
export function updateItem(layout: TentLayout, item: LayoutItem): TentLayout {
  if (!item.size.every((v) => Number.isFinite(v) && v >= .01 && v <= 10) || !item.position.every(Number.isFinite)) throw new Error("Enter finite dimensions of at least 1 cm.");
  if (!fits(item, layout.tent)) throw new Error("This object is too large for the tent at this rotation.");
  return { ...layout, items: layout.items.map((existing) => existing.id === item.id ? placeItem(item, layout.tent, layout.grid) : existing) };
}
export function addItem(layout: TentLayout, kind: string, id: string): TentLayout {
  if (layout.items.length >= 100) throw new Error("This layout supports up to 100 objects.");
  const entry = catalog.find((candidate) => candidate.kind === kind);
  if (!entry) throw new Error("Unknown equipment type.");
  const item: LayoutItem = { id, kind, size: [...entry.size], position: [0, kind.startsWith('led_') ? layout.tent[1] - .2 : 0, 0], rotation: 0 };
  if (!fits(item, layout.tent)) throw new Error("Enlarge the tent before adding this object.");
  // Find a vacant snapped location without moving existing objects.
  for (let z = -layout.tent[2] / 2; z <= layout.tent[2] / 2; z += layout.grid) {
    for (let x = -layout.tent[0] / 2; x <= layout.tent[0] / 2; x += layout.grid) {
      const placed = placeItem({ ...item, position: [x, item.position[1], z] }, layout.tent, layout.grid);
      const a = footprint(placed);
      const overlaps = layout.items.some((other) => {
        const b = footprint(other);
        return Math.abs(placed.position[0] - other.position[0]) < (a[0] + b[0]) / 2 + .02 && Math.abs(placed.position[2] - other.position[2]) < (a[2] + b[2]) / 2 + .02 && placed.position[1] < other.position[1] + b[1] && placed.position[1] + a[1] > other.position[1];
      });
      if (!overlaps) return { ...layout, items: [...layout.items, placed] };
    }
  }
  throw new Error("No free grid space. Move existing objects or enlarge the tent.");
}
export function parseTentLayout(raw: string | null): TentLayout {
  if (!raw) return initialTentLayout();
  const value = JSON.parse(raw) as TentLayout;
  const vector = (v: unknown): v is Vec3 => Array.isArray(v) && v.length === 3 && v.every((n) => typeof n === 'number' && Number.isFinite(n));
  if (value.version !== 1 || !vector(value.tent) || ![.05, .1, .25].includes(value.grid) || !Array.isArray(value.items) || value.items.length > 100) throw new Error('Invalid saved layout');
  const ids = new Set<string>();
  for (const item of value.items) {
    if (!item || typeof item.id !== 'string' || ids.has(item.id) || !catalog.some((entry) => entry.kind === item.kind) || !vector(item.size) || item.size.some((n) => n < .01 || n > 10) || !vector(item.position) || ![0, 90, 180, 270].includes(item.rotation)) throw new Error('Invalid saved object');
    ids.add(item.id);
  }
  return resizeTent(value, value.tent);
}
