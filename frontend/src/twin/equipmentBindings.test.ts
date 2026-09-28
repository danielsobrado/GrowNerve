import { describe, expect, it } from "vitest";
import { pilotData } from "../runtime/pilotData";
import { entityKey, sceneBindings } from "./sceneState";

describe("built-in equipment bindings", () => {
  it("adds existing controller and pump identities without mutating stored layouts", () => {
    const data = pilotData();
    const original = structuredClone(data);
    const bindings = sceneBindings(data);
    expect(bindings).toHaveLength(data.scene_layouts[0].entities.length + 2);
    expect(new Set(bindings.map((b) => entityKey(b.entity_type, b.entity_id))).size).toBe(bindings.length);
    for (const type of ["controller", "air_pump"]) {
      expect(bindings.find((b) => b.profile === type)?.entity_id).toBe(data.devices.find((d) => d.type === type)?.id);
    }
    expect(data).toEqual(original);
  });

  it("preserves explicitly placed hardware and omits devices outside rendered zones", () => {
    const data = pilotData();
    data.scene_layouts[0].entities = sceneBindings(data);
    expect(sceneBindings(data)).toEqual(data.scene_layouts[0].entities);
    data.scene_layouts[0].entities = [];
    expect(sceneBindings(data)).toEqual([]);
  });
});
