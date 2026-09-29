import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { addItem, catalog, initialTentLayout, parseTentLayout, placeItem, resizeTent, updateItem, type LayoutItem } from './tentLayout';

const item: LayoutItem = { id: 'pot', kind: 'pot_nursery', size: [.6, .4, .3], position: [3, -2, -3], rotation: 90 };
describe('tent layout constraints', () => {
  it('has exported Blender modules for every catalog variant', () => {
    const modules = new Set<string>();
    for (const file of ['grow-options', 'climate-systems', 'layout-sensors']) {
      const bytes = readFileSync(`public/models/blender/${file}.glb`);
      const json = JSON.parse(bytes.subarray(20, 20 + bytes.readUInt32LE(12)).toString());
      for (const node of json.nodes) if (node.extras?.equipmentModule) modules.add(node.extras.equipmentModule);
    }
    for (const entry of catalog.filter((entry) => /^(pot_|led_|fan_|humidifier_|irrigation_|sensor_)/.test(entry.kind))) expect(modules.has(entry.kind), entry.kind).toBe(true);
  });
  it('snaps rotated bounds inside walls and clamps to the floor', () => {
    const placed = placeItem(item, [1, 1, 1], .1);
    expect(placed.position).toEqual([.3, 0, -.1]);
  });
  it('keeps equipment sizes on tent resize and rejects a tent that cannot contain it', () => {
    const layout = { ...initialTentLayout(), items: [item] };
    const resized = resizeTent(layout, [1, 1, 1]);
    expect(resized.items[0].size).toEqual(item.size);
    expect(resized.items[0].position).toEqual([.3, 0, -.1]);
    expect(() => resizeTent(layout, [.5, .5, .5])).toThrow(/too large/);
  });
  it('rejects invalid dimensions without modifying the source', () => {
    const layout = { ...initialTentLayout(), items: [item] };
    expect(() => updateItem(layout, { ...item, size: [NaN, 1, 1] })).toThrow();
    expect(() => updateItem(layout, { ...item, size: [4, 1, 1] })).toThrow();
    expect(layout.items[0]).toEqual(item);
  });
  it('finds separate grid slots and round-trips all changes', () => {
    let layout = addItem(initialTentLayout(), 'pot_nursery', 'a');
    layout = addItem(layout, 'pot_nursery', 'b');
    expect(layout.items[0].position).not.toEqual(layout.items[1].position);
    expect(parseTentLayout(JSON.stringify(layout))).toEqual(layout);
    expect(() => parseTentLayout(JSON.stringify({ ...layout, items: [layout.items[0], layout.items[0]] }))).toThrow();
    expect(() => parseTentLayout('{bad')).toThrow();
  });
});
