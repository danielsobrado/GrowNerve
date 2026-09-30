import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { addItem, catalog, defaultProps, duplicateItem, removeItem, initialTentLayout, layoutSummary, parseLayoutFile, parseTentLayout, placeItem, resizeTent, serializeLayoutFile, specsFor, updateItem, type LayoutItem } from './tentLayout';

const item: LayoutItem = { id: 'pot', kind: 'pot_nursery', size: [.6, .4, .3], position: [3, -2, -3], rotation: 90, props: defaultProps('pot_nursery') };
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
  it('duplicates next to the source and removes by id', () => {
    let layout = addItem(initialTentLayout(), 'pot_nursery', 'a');
    layout = updateItem(layout, { ...layout.items[0], position: [.5, 0, .5], size: [.4, .5, .3], rotation: 90 });
    layout = duplicateItem(layout, 'a', 'b');
    const [source, copy] = layout.items;
    expect(copy).toMatchObject({ id: 'b', kind: source.kind, size: source.size, rotation: 90 });
    expect(copy.position).not.toEqual(source.position);
    expect(Math.hypot(copy.position[0] - source.position[0], copy.position[2] - source.position[2])).toBeLessThan(.6);
    expect(() => duplicateItem(layout, 'missing', 'c')).toThrow();
    expect(removeItem(layout, 'a').items.map((entry) => entry.id)).toEqual(['b']);
  });
  it('gives every catalog kind a name parameter and valid defaults', () => {
    for (const entry of catalog) {
      expect(specsFor(entry.kind)[0].key).toBe('label');
      const layout = addItem(initialTentLayout(), entry.kind, 'x');
      expect(layout.items[0].props).toEqual(defaultProps(entry.kind));
    }
    expect(defaultProps('led_panel')).toMatchObject({ watts: 240, diodes: 576, output: 70, spectrum: 'full', photoperiod: 18 });
  });
  it('validates equipment parameters on update', () => {
    let layout = addItem(initialTentLayout(), 'led_panel', 'led');
    const led = layout.items[0];
    layout = updateItem(layout, { ...led, props: { ...led.props, watts: 600, label: 'Main light', unknown: 1 } });
    expect(layout.items[0].props).toMatchObject({ watts: 600, label: 'Main light' });
    expect(layout.items[0].props).not.toHaveProperty('unknown');
    expect(() => updateItem(layout, { ...led, props: { ...led.props, watts: -5 } })).toThrow(/Power draw/);
    expect(() => updateItem(layout, { ...led, props: { ...led.props, spectrum: 'ultraviolet' } })).toThrow(/Spectrum/);
  });
  it('upgrades version 1 layouts to a tent with default parameters', () => {
    const legacy = { version: 1, tent: [2.4, 2.6, 2.4], grid: .1, items: [{ id: 'a', kind: 'led_bar', size: [.58, .1, .07], position: [0, 2.4, 0], rotation: 0 }] };
    const layout = parseTentLayout(JSON.stringify(legacy));
    expect(layout).toMatchObject({ version: 2, environment: 'tent', name: 'My grow' });
    expect(layout.items[0].props).toEqual(defaultProps('led_bar'));
  });
  it('allows larger outdoor plots than tents', () => {
    const layout = initialTentLayout();
    expect(() => resizeTent(layout, [20, 3, 12])).toThrow(/Tent/);
    expect(resizeTent(layout, [20, 3, 12], 'outdoor')).toMatchObject({ environment: 'outdoor', tent: [20, 3, 12] });
    expect(() => resizeTent(layout, [40, 3, 12], 'outdoor')).toThrow(/Plot/);
  });
  it('round-trips an exported layout file and rejects foreign or invalid files', () => {
    let layout = { ...resizeTent(initialTentLayout(), [6, 3, 4], 'outdoor'), name: 'Balcony' };
    layout = addItem(layout, 'led_multi_bar', 'a');
    layout = updateItem(layout, { ...layout.items[0], props: { ...layout.items[0].props, bars: 8, watts: 720 } });
    const text = serializeLayoutFile(layout, '2026-09-30T00:00:00.000Z');
    expect(JSON.parse(text)).toMatchObject({ format: 'grownerve.layout', version: 2, exported_at: '2026-09-30T00:00:00.000Z' });
    expect(parseLayoutFile(text)).toEqual(layout);
    expect(() => parseLayoutFile('{"format":"something-else","layout":{}}')).toThrow(/not a GrowNerve layout/);
    expect(() => parseLayoutFile('not json')).toThrow(/not valid JSON/);
    const tampered = JSON.parse(text); tampered.layout.items[0].props.watts = 99999;
    expect(() => parseLayoutFile(JSON.stringify(tampered))).toThrow(/Power draw/);
  });
  it('summarises equipment and the lighting power budget', () => {
    let layout = addItem(initialTentLayout(), 'led_panel', 'a');
    layout = addItem(layout, 'led_panel', 'b');
    layout = addItem(layout, 'fan_clip', 'c');
    const summary = layoutSummary(layout);
    expect(summary.leds).toBe(2);
    expect(summary.lightWatts).toBeCloseTo(2 * 240 * .7);
    expect(summary.lightKwhPerDay).toBeCloseTo(2 * 240 * .7 * 18 / 1000);
    expect(summary.totalWatts).toBe(2 * 240 + 20);
    expect(summary.counts).toEqual([{ name: 'LED panel', count: 2 }, { name: 'Clip fan', count: 1 }]);
  });
});
