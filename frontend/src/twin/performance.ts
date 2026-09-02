import { useEffect, useState } from "react";
import performanceConfigYaml from "../config/twin-performance.yaml?raw";

export type TwinPerformanceProfileName = "desktop" | "tablet" | "mobile" | "low-power";

export interface TwinPerformanceProfile {
  name: TwinPerformanceProfileName;
  dpr: [number, number];
  ptlResolution: number;
  shadowMapSize: number;
  geometryScale: number;
  shadows: boolean;
  antialias: boolean;
  touchOptimized: boolean;
}

interface TwinPerformanceConfig {
  mobileMaxWidth: number;
  tabletMaxWidth: number;
  lowMemoryGb: number;
  lowCpuCores: number;
  profiles: Record<TwinPerformanceProfileName, Omit<TwinPerformanceProfile, "name" | "touchOptimized">>;
}

export interface TwinDeviceCapabilities {
  width: number;
  height: number;
  deviceMemoryGb?: number;
  cpuCores?: number;
  coarsePointer: boolean;
}

const scalarMap = (yaml: string): Map<string, string> => new Map(
  yaml
    .split(/\r?\n/)
    .map((line) => line.split("#", 1)[0].trim())
    .filter(Boolean)
    .map((line) => {
      const separator = line.indexOf(":");
      if (separator < 1) throw new Error(`Invalid twin performance config line: ${line}`);
      return [line.slice(0, separator).trim(), line.slice(separator + 1).trim()];
    }),
);

const configValues = scalarMap(performanceConfigYaml);

const numberValue = (key: string): number => {
  const value = Number(configValues.get(key));
  if (!Number.isFinite(value)) throw new Error(`Invalid numeric twin performance value: ${key}`);
  return value;
};

const booleanValue = (key: string): boolean => {
  const value = configValues.get(key);
  if (value === "true") return true;
  if (value === "false") return false;
  throw new Error(`Invalid boolean twin performance value: ${key}`);
};

const profile = (prefix: "desktop" | "tablet" | "mobile" | "low_power") => ({
  dpr: [numberValue(`${prefix}_dpr_min`), numberValue(`${prefix}_dpr_max`)] as [number, number],
  ptlResolution: numberValue(`${prefix}_ptl_resolution`),
  shadowMapSize: numberValue(`${prefix}_shadow_map_size`),
  geometryScale: numberValue(`${prefix}_geometry_scale`),
  shadows: booleanValue(`${prefix}_shadows`),
  antialias: booleanValue(`${prefix}_antialias`),
});

export const TWIN_PERFORMANCE_CONFIG: TwinPerformanceConfig = {
  mobileMaxWidth: numberValue("mobile_max_width"),
  tabletMaxWidth: numberValue("tablet_max_width"),
  lowMemoryGb: numberValue("low_memory_gb"),
  lowCpuCores: numberValue("low_cpu_cores"),
  profiles: {
    desktop: profile("desktop"),
    tablet: profile("tablet"),
    mobile: profile("mobile"),
    "low-power": profile("low_power"),
  },
};

export function selectTwinPerformanceProfile(
  capabilities: TwinDeviceCapabilities,
  config = TWIN_PERFORMANCE_CONFIG,
): TwinPerformanceProfile {
  const smallerViewportDimension = Math.min(capabilities.width, capabilities.height);
  const mobileViewport = capabilities.width <= config.mobileMaxWidth
    || capabilities.coarsePointer && smallerViewportDimension <= config.mobileMaxWidth;
  const tabletViewport = capabilities.width <= config.tabletMaxWidth
    || capabilities.coarsePointer && smallerViewportDimension <= config.tabletMaxWidth;
  const constrainedMemory = capabilities.deviceMemoryGb !== undefined && capabilities.deviceMemoryGb <= config.lowMemoryGb;
  const constrainedCpu = capabilities.cpuCores !== undefined && capabilities.cpuCores <= config.lowCpuCores;

  const name: TwinPerformanceProfileName = constrainedMemory || constrainedCpu
    ? "low-power"
    : mobileViewport
      ? "mobile"
      : tabletViewport
        ? "tablet"
        : "desktop";

  return {
    name,
    ...config.profiles[name],
    touchOptimized: capabilities.coarsePointer || mobileViewport,
  };
}

function browserCapabilities(): TwinDeviceCapabilities {
  if (typeof window === "undefined" || typeof navigator === "undefined") {
    return { width: 1440, height: 900, coarsePointer: false };
  }

  const navigatorWithMemory = navigator as Navigator & { deviceMemory?: number };
  return {
    width: window.innerWidth,
    height: window.innerHeight,
    deviceMemoryGb: navigatorWithMemory.deviceMemory,
    cpuCores: navigator.hardwareConcurrency || undefined,
    coarsePointer: window.matchMedia?.("(pointer: coarse)").matches ?? false,
  };
}

export function useTwinPerformanceProfile(): TwinPerformanceProfile {
  const [selected, setSelected] = useState(() => selectTwinPerformanceProfile(browserCapabilities()));

  useEffect(() => {
    let frame = 0;
    const update = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => setSelected(selectTwinPerformanceProfile(browserCapabilities())));
    };
    window.addEventListener("resize", update, { passive: true });
    window.addEventListener("orientationchange", update, { passive: true });
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("resize", update);
      window.removeEventListener("orientationchange", update);
    };
  }, []);

  return selected;
}
