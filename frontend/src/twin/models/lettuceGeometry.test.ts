import { describe, expect, it } from "vitest";
import { createLettuceGeometry } from "./lettuceGeometry";

describe("lettuce geometry", () => {
  it("builds deterministic finite foliage within the plant footprint", () => {
    const geometry = createLettuceGeometry(1, false, 7);
    const duplicate = createLettuceGeometry(1, false, 7);
    expect(geometry.getAttribute("position").array).toEqual(duplicate.getAttribute("position").array);
    for (const name of ["position", "normal", "color"]) {
      expect(Array.from(geometry.getAttribute(name).array).every(Number.isFinite)).toBe(true);
    }
    expect(geometry.boundingSphere!.radius).toBeLessThan(1);
    expect(geometry.getIndex()!.count / 3).toBeLessThan(25_000);
    geometry.dispose(); duplicate.dispose();
  });

  it("reduces geometry on constrained devices and localizes health coloring", () => {
    const desktop = createLettuceGeometry(1, false);
    const mobile = createLettuceGeometry(0.55, false);
    const attention = createLettuceGeometry(1, true);
    expect(mobile.getIndex()!.count).toBeLessThan(desktop.getIndex()!.count / 3);
    expect(attention.getAttribute("position").array).toEqual(desktop.getAttribute("position").array);
    expect(attention.getAttribute("color").array).not.toEqual(desktop.getAttribute("color").array);
    desktop.dispose(); mobile.dispose(); attention.dispose();
  });
});
