import { describe, expect, it } from "vitest";
import { createArchive, validateArchive } from "./archive";
import { pilotData } from "./pilotData";
import { addItem, initialTentLayout, resizeTent, updateItem } from "../twin/tentLayout";

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

  it("rejects missing collections, duplicate IDs, and invalid UUIDs", () => {
    const archive = createArchive(pilotData());
    const missing = structuredClone(archive) as unknown as Record<string, unknown>;
    delete (missing.data as Record<string, unknown>).alerts;
    expect(() => validateArchive(missing)).toThrow("alerts");
    const duplicate = structuredClone(archive);
    duplicate.data.facilities.push(structuredClone(duplicate.data.facilities[0]));
    expect(() => validateArchive(duplicate)).toThrow("duplicate");
    const invalid = structuredClone(archive);
    invalid.data.facilities[0].id = "not-a-uuid";
    expect(() => validateArchive(invalid)).toThrow("invalid UUID");
  });

  it("round-trips tent and outdoor layouts with the farm data", () => {
    const data = pilotData(), facility = data.facilities[0].id;
    let outdoor = { ...resizeTent(initialTentLayout(), [8, 3, 5], "outdoor"), name: "Backyard" };
    outdoor = addItem(outdoor, "led_panel", "led");
    outdoor = updateItem(outdoor, { ...outdoor.items[0], props: { ...outdoor.items[0].props, watts: 600 } });
    const archive = createArchive(data, { layouts: [{ facility_id: facility, layout: outdoor }, { facility_id: "default", layout: initialTentLayout() }] });
    expect(archive.layouts?.map((entry) => entry.facility_id)).toEqual([facility, "default"].sort());
    const restored = validateArchive(JSON.parse(JSON.stringify(archive)));
    expect(restored.layouts?.find((entry) => entry.facility_id === facility)?.layout).toEqual(outdoor);
  });

  it("imports archives made before layouts existed", () => {
    const legacy = structuredClone(createArchive(pilotData())) as unknown as Record<string, unknown>;
    delete legacy.layouts;
    expect(validateArchive(legacy).layouts).toEqual([]);
  });

  it("rejects layouts for unknown facilities, duplicates, and invalid layouts", () => {
    const data = pilotData(), facility = data.facilities[0].id, layout = initialTentLayout();
    const archive = createArchive(data, { layouts: [{ facility_id: facility, layout }] });
    expect(() => validateArchive({ ...archive, layouts: [{ facility_id: "01990a20-6a00-7000-8000-000000000099", layout }] })).toThrow("unknown facility");
    expect(() => validateArchive({ ...archive, layouts: [{ facility_id: facility, layout }, { facility_id: facility, layout }] })).toThrow("two layouts");
    const broken = structuredClone(archive);
    (broken.layouts![0].layout as unknown as { grid: number }).grid = 3;
    expect(() => validateArchive(broken)).toThrow(`Layout for ${facility} is invalid`);
  });
});

describe("portable archive integration bindings", () => {
  const bound = () => {
    const archive = createArchive(pilotData(), { now: "2026-09-01T12:00:00Z", exportId: "01990a20-6a00-7000-8000-000000000001" });
    archive.data.devices[0].integration = { provider: "zigbee2mqtt", external_id: "0x00158d0000beef01", adopted_at: "2026-09-30T00:00:00Z" };
    archive.data.channels[0].integration_key = "state";
    return archive;
  };

  it("round-trips devices adopted from an integration", () => {
    const restored = validateArchive(JSON.parse(JSON.stringify(bound())));
    expect(restored.data.devices[0].integration?.external_id).toBe("0x00158d0000beef01");
    expect(restored.data.channels[0].integration_key).toBe("state");
  });

  it("rejects unknown providers, empty identities and duplicate bindings", () => {
    const unknown = bound();
    (unknown.data.devices[0].integration as { provider: string }).provider = "zwave";
    expect(() => validateArchive(unknown)).toThrow("unknown integration provider");
    const empty = bound();
    empty.data.devices[0].integration!.external_id = "";
    expect(() => validateArchive(empty)).toThrow("invalid integration identity");
    const duplicate = bound();
    duplicate.data.devices[1].integration = { ...duplicate.data.devices[0].integration! };
    expect(() => validateArchive(duplicate)).toThrow("Two devices are bound");
    const badKey = bound();
    (badKey.data.channels[0] as { integration_key: unknown }).integration_key = 7;
    expect(() => validateArchive(badKey)).toThrow("invalid integration key");
  });
});
