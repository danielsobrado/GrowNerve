import { describe, expect, it } from "vitest";
import { pilotData } from "./pilotData";
import { buildAdoptRequest, defaultDraft, displayUnit, draftProblems, permitJoinRemaining, selectsCommand, type IntegrationStatus } from "./integrations";
import { light, plug } from "../test/integrationFixtures";

describe("integration adoption helpers", () => {
  it("starts with every non-diagnostic capability selected and measurements on the zone", () => {
    const draft = defaultDraft(plug, "zone-1");
    expect(draft.selections.map((selection) => selection.selected)).toEqual([true, true, false]);
    expect(draft.selections.map((selection) => selection.entity)).toEqual(["device", "zone", "zone"]);
    expect(draft).toMatchObject({ zoneId: "zone-1", name: "Sim plug", type: "controller", acknowledgeNoEdgeFailsafe: false });
  });

  it("requires the failsafe acknowledgement only when an output is chosen", () => {
    const data = pilotData(), zone = data.zones[0].id;
    const draft = defaultDraft(plug, zone);
    expect(selectsCommand(plug, draft)).toBe(true);
    expect(draftProblems(plug, draft, data)).toContain("Confirm that controllable outputs have no controller-side failsafe.");
    draft.selections[0].selected = false;
    expect(selectsCommand(plug, draft)).toBe(false);
    expect(draftProblems(plug, draft, data)).toEqual([]);
  });

  it("reports key conflicts, empty choices and out-of-range safe limits before the server does", () => {
    const data = pilotData(), zone = data.zones[0].id;
    const draft = defaultDraft(light, zone);
    draft.acknowledgeNoEdgeFailsafe = true;
    draft.selections[0].key = data.channels[0].key;
    draft.selections[0].safeMaximum = 150;
    const problems = draftProblems(light, draft, data);
    expect(problems.some((problem) => problem.includes("already used"))).toBe(true);
    expect(problems.some((problem) => problem.includes("safe range"))).toBe(true);
    draft.selections[0].selected = false;
    expect(draftProblems(light, { ...draft, zoneId: "" }, data)).toEqual(["Choose a zone.", "Choose at least one reading or output."]);
  });

  it("builds the request the adoption endpoint expects", () => {
    const draft = defaultDraft(plug, "zone-1");
    draft.acknowledgeNoEdgeFailsafe = true;
    draft.name = "  Exhaust plug ";
    draft.type = "fan";
    draft.selections[1].key = " tent.exhaust.watts ";
    expect(buildAdoptRequest(plug, draft)).toEqual({
      zone_id: "zone-1", name: "Exhaust plug", type: "fan", acknowledge_no_edge_failsafe: true,
      channels: [
        { capability_key: "state", key: "sim_plug.state", name: "Sim plug state", safe_minimum: 0, safe_maximum: 1 },
        { capability_key: "power", key: "tent.exhaust.watts", name: "Sim plug power", entity_type: "zone", entity_id: "zone-1" },
      ],
    });
  });

  it("shows canonical units the way people read them", () => {
    expect([displayUnit("degC"), displayUnit("%RH"), displayUnit("bool"), displayUnit("W"), displayUnit(undefined)]).toEqual(["°C", "% RH", "", "W", ""]);
  });

  it("counts down an open join window", () => {
    const status = { provider: "zigbee2mqtt", enabled: true, state: "connected", since: "", device_count: 0, adopted_count: 0, permit_join_supported: true, permit_join_until: "2026-09-30T12:02:00Z" } satisfies IntegrationStatus;
    expect(permitJoinRemaining(status, Date.parse("2026-09-30T12:00:00Z"))).toBe(120);
    expect(permitJoinRemaining(status, Date.parse("2026-09-30T12:05:00Z"))).toBe(0);
    expect(permitJoinRemaining(undefined)).toBe(0);
  });
});
