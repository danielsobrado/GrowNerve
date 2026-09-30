import { afterEach, describe, expect, it } from "vitest";
import { clearStoredLayouts, layoutStorageKey, readStoredLayouts, writeStoredLayouts } from "./layoutStore";
import { initialTentLayout, resizeTent } from "./tentLayout";

afterEach(() => localStorage.clear());

describe("stored layouts", () => {
  it("writes, reads back sorted, skips invalid drafts, and clears only layouts", () => {
    const outdoor = resizeTent(initialTentLayout(), [6, 3, 4], "outdoor");
    writeStoredLayouts([{ facility_id: "b", layout: outdoor }, { facility_id: "a", layout: initialTentLayout() }]);
    localStorage.setItem(layoutStorageKey("broken"), "{bad");
    localStorage.setItem("grownerve.lengthUnit", "ft");
    expect(readStoredLayouts()).toEqual([{ facility_id: "a", layout: initialTentLayout() }, { facility_id: "b", layout: outdoor }]);
    clearStoredLayouts();
    expect(readStoredLayouts()).toEqual([]);
    expect(localStorage.getItem("grownerve.lengthUnit")).toBe("ft");
  });
});
