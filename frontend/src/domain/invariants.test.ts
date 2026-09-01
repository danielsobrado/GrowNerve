import { describe, expect, it } from "vitest";
import {
  DomainError,
  acknowledgeAlert,
  completeGrowCycle,
  evaluateSetpoint,
  resolveAlert,
  startGrowCycle,
  transitionCommand,
  validateMeasurement,
} from "./invariants";
import { pilotData } from "../runtime/pilotData";

describe("GrowNerve domain invariants", () => {
  it("starts a planned grow only with a published recipe version", () => {
    const data = pilotData();
    const grow = { ...data.grow_cycles[0], status: "planned" as const, actual_start: undefined };
    expect(startGrowCycle(grow, data.recipe_versions[0], "2026-09-01T10:00:00Z").status).toBe("active");
    expect(() => startGrowCycle(grow, { ...data.recipe_versions[0], status: "draft" }, "2026-09-01T10:00:00Z")).toThrow(DomainError);
  });

  it("does not complete a grow before it starts", () => {
    const data = pilotData();
    expect(() => completeGrowCycle(data.grow_cycles[0], "2026-01-01T00:00:00Z")).toThrow("Harvest cannot precede grow start");
  });

  it("evaluates recipe setpoint ranges", () => {
    expect(evaluateSetpoint(17, { minimum: 18, maximum: 24 })).toBe("low");
    expect(evaluateSetpoint(21, { minimum: 18, maximum: 24 })).toBe("in_range");
    expect(evaluateSetpoint(25, { minimum: 18, maximum: 24 })).toBe("high");
  });

  it("rejects a measurement with the wrong unit dimension", () => {
    const data = pilotData();
    const channel = data.channels.find((entry) => entry.key.includes("temperature"))!;
    expect(() => validateMeasurement({ channel_id: channel.id, value: 21, unit: "%RH", observed_at: new Date().toISOString(), quality: "good" }, channel)).toThrow("unit");
  });

  it("enforces alert and command lifecycle transitions", () => {
    const data = pilotData();
    expect(acknowledgeAlert(data.alerts[0], "operator", "2026-09-01T12:00:00Z").status).toBe("acknowledged");
    expect(() => transitionCommand({ ...data.commands[0], status: "pending" }, "applied", "2026-09-01T12:00:00Z")).toThrow("Invalid command transition");
  });

  it("covers invalid lifecycle and quality-aware boundary paths", () => {
    const data = pilotData();
    expect(() => startGrowCycle(data.grow_cycles[0], data.recipe_versions[0], new Date().toISOString())).toThrow("planned");
    expect(() => completeGrowCycle({ ...data.grow_cycles[0], status: "planned" }, new Date().toISOString())).toThrow("active");
    expect(evaluateSetpoint(20, {})).toBe("unknown");
    const measurementChannel = data.channels.find((entry) => entry.key === "air.temperature")!;
    expect(validateMeasurement({ channel_id: measurementChannel.id, value: 100, unit: "degC", observed_at: new Date().toISOString(), quality: "good" }, measurementChannel).quality).toBe("suspect");
    expect(() => validateMeasurement({ channel_id: measurementChannel.id, value: Number.NaN, unit: "degC", observed_at: new Date().toISOString(), quality: "good" }, measurementChannel)).toThrow("finite");
    expect(() => validateMeasurement({ channel_id: data.channels.find((entry) => entry.kind === "command")!.id, value: 1, unit: "", observed_at: new Date().toISOString(), quality: "good" }, data.channels.find((entry) => entry.kind === "command")!)).toThrow("measurements");
    expect(() => acknowledgeAlert({ ...data.alerts[0], status: "resolved" }, "operator", new Date().toISOString())).toThrow("open");
    expect(resolveAlert(data.alerts[0], new Date().toISOString()).status).toBe("resolved");
    expect(() => resolveAlert({ ...data.alerts[0], status: "resolved" }, new Date().toISOString())).toThrow("already");
  });
});
