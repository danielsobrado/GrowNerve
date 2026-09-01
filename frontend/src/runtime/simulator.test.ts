import { describe, expect, it } from "vitest";
import { pilotData } from "./pilotData";
import { applySimulatedCommand, setDeviceOnline, tickSimulator } from "./simulator";

describe("browser simulator", () => {
  it("applies a bounded fan command through the lifecycle", () => {
    const data = pilotData();
    const result = applySimulatedCommand(data, { targetChannelId: data.channels.find((channel) => channel.key === "fan.speed.command")!.id, value: 55, reason: "test" }, "2026-09-01T12:00:00Z");
    expect(result.commands.at(-1)?.status).toBe("applied");
    expect(result.devices.find((device) => device.type === "fan")?.output_percent).toBe(55);
  });

  it("records rejection instead of bypassing a safety limit", () => {
    const data = pilotData();
    const result = applySimulatedCommand(data, { targetChannelId: data.channels.find((channel) => channel.key === "fan.speed.command")!.id, value: 10, reason: "test" }, "2026-09-01T12:00:00Z");
    expect(result.commands.at(-1)).toMatchObject({ status: "rejected", reason_code: "COMMAND_VALUE_OUT_OF_RANGE" });
  });

  it("generates quality-aware telemetry without changing channel identity", () => {
    const data = pilotData();
    const channelIds = data.channels.map((channel) => channel.id);
    const next = tickSimulator(data, "2026-09-01T12:00:00Z", 44);
    expect(next.measurements.length).toBeGreaterThan(data.measurements.length);
    expect(next.channels.map((channel) => channel.id)).toEqual(channelIds);
  });

  it("rejects unknown and offline targets and applies boolean commands", () => {
    const data = pilotData();
    expect(applySimulatedCommand(data, { targetChannelId: crypto.randomUUID(), value: 1, reason: "test" }).commands.at(-1)?.reason_code).toBe("CHANNEL_NOT_CONTROLLABLE");
    const fan = data.devices.find((entry) => entry.type === "fan")!;
    const offline = setDeviceOnline(data, fan.id, false);
    const fanChannel = offline.channels.find((entry) => entry.device_id === fan.id && entry.kind === "command")!;
    expect(applySimulatedCommand(offline, { targetChannelId: fanChannel.id, value: 50, reason: "test" }).commands.at(-1)?.reason_code).toBe("DEVICE_OFFLINE");
    const lightChannel = data.channels.find((entry) => entry.key === "light.state.command")!;
    expect(applySimulatedCommand(data, { targetChannelId: lightChannel.id, value: false, reason: "test" }).devices.find((entry) => entry.type === "light")?.state).toBe(false);
    expect(setDeviceOnline(offline, fan.id, true, "2026-09-01T12:00:00Z").devices.find((entry) => entry.id === fan.id)?.last_heartbeat).toBe("2026-09-01T12:00:00Z");
  });
});
