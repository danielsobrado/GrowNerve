import { describe, expect, it } from "vitest";
import { pilotData } from "../runtime/pilotData";
import { formatTelemetryValue, latestMeasurementsByChannel, readingByKey } from "./telemetry";

describe("digital twin telemetry presentation", () => {
  it("formats common farm units for operators", () => {
    expect(formatTelemetryValue(20.14, "degC")).toBe("20.1 °C");
    expect(formatTelemetryValue(68, "%RH")).toBe("68 % RH");
    expect(formatTelemetryValue(72, "%")).toBe("72 %");
  });

  it("uses the newest sample and reports freshness", () => {
    const data = pilotData();
    const latest = latestMeasurementsByChannel(data);
    const fresh = readingByKey(data, latest, "water.temperature", Date.parse("2026-09-01T23:01:30.000Z"));
    const stale = readingByKey(data, latest, "water.temperature", Date.parse("2026-09-01T23:04:00.000Z"));

    expect(fresh?.displayValue).toContain("°C");
    expect(fresh?.stale).toBe(false);
    expect(stale?.stale).toBe(true);
  });
});
