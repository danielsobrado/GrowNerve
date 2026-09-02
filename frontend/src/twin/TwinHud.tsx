import { Droplets, Gauge, Thermometer, Waves, type LucideIcon } from "lucide-react";
import { useMemo } from "react";
import type { EntityType, FarmData } from "../domain/model";
import { latestMeasurementsByChannel, readingByKey } from "./telemetry";

interface HudMetric {
  key: string;
  label: string;
  icon: LucideIcon;
  scope: EntityType;
}

const HUD_METRICS: HudMetric[] = [
  { key: "air.temperature", label: "Air", icon: Thermometer, scope: "zone" },
  { key: "air.humidity", label: "Humidity", icon: Droplets, scope: "zone" },
  { key: "water.temperature", label: "Water", icon: Waves, scope: "reservoir" },
  { key: "water.level", label: "Level", icon: Gauge, scope: "reservoir" },
];

export function TwinHud({ data }: { data: FarmData }) {
  const latest = useMemo(() => latestMeasurementsByChannel(data), [data]);
  const sceneEntities = data.scene_layouts[0]?.entities ?? [];
  const readings = HUD_METRICS.map((metric) => {
    const target = sceneEntities.find((entry) => entry.entity_type === metric.scope);
    return {
      metric,
      reading: readingByKey(data, latest, metric.key, target ? { entityType: target.entity_type, entityId: target.entity_id } : { entityType: metric.scope }),
    };
  }).filter((entry) => entry.reading);
  const liveCount = readings.filter(({ reading }) => reading && !reading.stale && reading.quality === "good").length;
  const state = liveCount === readings.length && readings.length > 0 ? "Live conditions" : liveCount > 0 ? "Mixed freshness" : "Last known values";

  if (readings.length === 0) return null;

  return <section className="gn-twin-hud" aria-label="Digital twin telemetry summary">
    <header><div><span className={`gn-twin-live ${liveCount > 0 ? "is-live" : ""}`} />Telemetry</div><strong>{state}</strong></header>
    <div className="gn-twin-hud-grid">{readings.map(({ metric, reading }) => {
      if (!reading) return null;
      const Icon = metric.icon;
      return <div className={`gn-twin-hud-metric ${reading.stale ? "is-stale" : ""}`} key={metric.key} title={`${reading.label} · ${reading.quality}`}>
        <Icon size={14} strokeWidth={1.8} />
        <span>{metric.label}</span>
        <strong>{reading.displayValue}</strong>
      </div>;
    })}</div>
  </section>;
}
