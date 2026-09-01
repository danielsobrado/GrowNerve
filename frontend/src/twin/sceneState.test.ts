import { describe, expect, it } from "vitest";
import { actionsForProfile, buildSceneIndex } from "./sceneState";
import { pilotData } from "../runtime/pilotData";

describe("digital twin state adapter", () => {
  it("indexes stable domain bindings", () => {
    const data = pilotData();
    const index = buildSceneIndex(data.scene_layouts[0]);
    expect(index.get(`reservoir:${data.reservoirs[0].id}`)?.profile).toBe("reservoir");
  });

  it("keeps important actions available outside 3D", () => {
    expect(actionsForProfile("fan")).toEqual(["Inspect", "Set output", "Override", "History", "Maintenance"]);
    expect(actionsForProfile("plant")).toContain("Observe");
  });
});
