import type { FarmData, Measurement, Quality } from "../domain/model";

const UNIT_LABELS: Record<string, string> = {
  degC: "°C",
  "%RH": "% RH",
  "%": "%",
};

export interface TelemetryReading {
  key: string;
  label: string;
  value: number;
  unit: string;
  displayValue: string;
  observedAt: string;
  quality: Quality;
  stale: boolean;
}

const formatNumber = (value: number) => Math.abs(value) >= 100 || Number.isInteger(value) ? value.toFixed(0) : value.toFixed(1);

export const formatTelemetryValue = (value: number, unit?: string) => {
  const formattedUnit = unit ? UNIT_LABELS[unit] ?? unit : "";
  return `${formatNumber(value)}${formattedUnit ? ` ${formattedUnit}` : ""}`;
};

export function latestMeasurementsByChannel(data: FarmData): Map<string, Measurement> {
  const latest = new Map<string, Measurement>();
  for (const measurement of data.measurements) {
    const current = latest.get(measurement.channel_id);
    if (!current || measurement.observed_at > current.observed_at) latest.set(measurement.channel_id, measurement);
  }
  return latest;
}

export function latestReadingByKey(data: FarmData, key: string, now = Date.now()): TelemetryReading | undefined {
  const channel = data.channels.find((entry) => entry.key === key);
  if (!channel) return undefined;
  const measurement = latestMeasurementsByChannel(data).get(channel.id);
  if (!measurement) return undefined;
  const ageMs = Math.max(0, now - new Date(measurement.observed_at).getTime());
  return {
    key,
    label: channel.name,
    value: measurement.value,
    unit: measurement.unit || channel.unit || "",
    displayValue: formatTelemetryValue(measurement.value, measurement.unit || channel.unit),
    observedAt: measurement.observed_at,
    quality: measurement.quality,
    stale: ageMs > channel.stale_after_seconds * 1000 || measurement.quality === "stale",
  };
}
