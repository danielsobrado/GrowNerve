import { describe, expect, it } from "vitest";
import { createArchive, validateArchive } from "./archive";
import { pilotData } from "./pilotData";

describe("portable archive", () => {
  it("exports all stores in deterministic ID order", () => {
    const data = pilotData();
    data.facilities = [...data.facilities].reverse();
    const archive = createArchive(data, { now: "2026-09-01T12:00:00Z", exportId: "01990a20-6a00-7000-8000-000000000001" });
    expect(archive.format).toBe("grownerve");
    expect(archive.schema_version).toBe(1);
    expect(archive.data.facilities.map((entry) => entry.id)).toEqual([...archive.data.facilities.map((entry) => entry.id)].sort());
  });

  it("rejects future schemas and broken references", () => {
    const archive = createArchive(pilotData(), { now: "2026-09-01T12:00:00Z", exportId: "01990a20-6a00-7000-8000-000000000001" });
    expect(() => validateArchive({ ...archive, schema_version: 99 })).toThrow("Unsupported archive schema");
    const broken = structuredClone(archive);
    broken.data.zones[0].facility_id = "01990a20-6a00-7000-8000-000000000099";
    expect(() => validateArchive(broken)).toThrow("unknown facility");
  });
});
