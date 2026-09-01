import type { FarmCommand, FarmData, Measurement } from "../domain/model";

export interface CommandIntent { targetChannelId: string; value: number | boolean; reason: string }

function simulatorUUID() {
  return crypto.randomUUID();
}

export function applySimulatedCommand(source: FarmData, intent: CommandIntent, at = new Date().toISOString()): FarmData {
  const data = structuredClone(source);
  const channel = data.channels.find((entry) => entry.id === intent.targetChannelId);
  const device = channel && data.devices.find((entry) => entry.id === channel.device_id);
  let status: FarmCommand["status"] = "applied";
  let reasonCode: string | undefined;
  const numericValue = typeof intent.value === "boolean" ? Number(intent.value) : intent.value;
  if (!channel || channel.kind !== "command") { status = "rejected"; reasonCode = "CHANNEL_NOT_CONTROLLABLE"; }
  else if (!device?.online) { status = "rejected"; reasonCode = "DEVICE_OFFLINE"; }
  else if (channel.safe_minimum !== undefined && numericValue < channel.safe_minimum || channel.safe_maximum !== undefined && numericValue > channel.safe_maximum) { status = "rejected"; reasonCode = "COMMAND_VALUE_OUT_OF_RANGE"; }

  const command: FarmCommand = {
    id: simulatorUUID(), target_channel_id: intent.targetChannelId,
    command_type: typeof intent.value === "boolean" ? "set_boolean" : "set_percent",
    value: intent.value, reason: intent.reason, status, requested_at: at, updated_at: at,
    simulated: true, reason_code: reasonCode,
  };
  data.commands.push(command);
  if (status === "applied" && device) {
    device.state = typeof intent.value === "boolean" ? intent.value : numericValue > 0;
    device.output_percent = typeof intent.value === "boolean" ? Number(intent.value) * 100 : numericValue;
  }
  data.events.push({ id: simulatorUUID(), type: status === "applied" ? "command.executed" : "command.rejected", occurred_at: at, actor: "Browser simulator", entity_type: "device", entity_id: device?.id ?? channel?.entity_id ?? data.facilities[0]?.id, summary: status === "applied" ? `Applied ${channel?.name}: ${String(intent.value)}` : `Rejected ${channel?.name ?? "unknown command"}: ${reasonCode}` });
  return data;
}

function pseudoRandom(seed: number) {
  const value = Math.sin(seed * 12.9898) * 43758.5453;
  return value - Math.floor(value);
}

export function tickSimulator(source: FarmData, at = new Date().toISOString(), seed = Date.now()): FarmData {
  const data = structuredClone(source);
  const activeDeviceIds = new Set(data.devices.filter((device) => device.online).map((device) => device.id));
  const offsets: Record<string, number> = { "air.temperature": 22.5, "air.humidity": 68, "water.temperature": 20.8, "water.level": data.reservoirs[0]?.level_percent ?? 70 };
  data.channels.filter((channel) => channel.kind === "measurement" && activeDeviceIds.has(channel.device_id)).forEach((channel, index) => {
    const previous = [...data.measurements].reverse().find((measurement) => measurement.channel_id === channel.id);
    const amplitude = channel.key.includes("humidity") ? 1.5 : channel.key.includes("level") ? 0.2 : 0.35;
    const value = Number(((previous?.value ?? offsets[channel.key] ?? 0) + (pseudoRandom(seed + index) - 0.5) * amplitude).toFixed(2));
    const measurement: Measurement = { id: simulatorUUID(), channel_id: channel.id, observed_at: at, received_at: at, value, unit: channel.unit ?? "", quality: "good", sequence: (previous?.sequence ?? 0) + 1, source_device_id: channel.device_id };
    data.measurements.push(measurement);
  });
  data.devices.filter((device) => device.simulated && device.online).forEach((device) => { device.last_heartbeat = at; });
  if (data.measurements.length > 5000) data.measurements = data.measurements.slice(-5000);
  return data;
}

export function setDeviceOnline(source: FarmData, deviceId: string, online: boolean, at = new Date().toISOString()): FarmData {
  const data = structuredClone(source);
  const device = data.devices.find((entry) => entry.id === deviceId);
  if (device) { device.online = online; if (online) device.last_heartbeat = at; }
  return data;
}
