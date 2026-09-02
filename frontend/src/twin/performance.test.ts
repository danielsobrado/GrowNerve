import { describe, expect, it } from "vitest";
import { selectTwinPerformanceProfile } from "./performance";

describe("selectTwinPerformanceProfile", () => {
  it("uses the full desktop profile for capable wide displays", () => {
    const profile = selectTwinPerformanceProfile({
      width: 1440,
      height: 900,
      deviceMemoryGb: 16,
      cpuCores: 12,
      coarsePointer: false,
    });

    expect(profile.name).toBe("desktop");
    expect(profile.dpr[1]).toBe(1.75);
    expect(profile.ptlResolution).toBe(128);
    expect(profile.shadows).toBe(true);
  });

  it("uses the mobile profile on capable phones", () => {
    const profile = selectTwinPerformanceProfile({
      width: 390,
      height: 844,
      deviceMemoryGb: 8,
      cpuCores: 8,
      coarsePointer: true,
    });

    expect(profile.name).toBe("mobile");
    expect(profile.dpr[1]).toBe(1.2);
    expect(profile.ptlResolution).toBe(96);
    expect(profile.touchOptimized).toBe(true);
  });

  it("keeps landscape phones on the mobile profile", () => {
    const profile = selectTwinPerformanceProfile({
      width: 844,
      height: 390,
      deviceMemoryGb: 8,
      cpuCores: 8,
      coarsePointer: true,
    });

    expect(profile.name).toBe("mobile");
  });

  it("uses the low-power profile when memory or CPU is constrained", () => {
    const profile = selectTwinPerformanceProfile({
      width: 390,
      height: 844,
      deviceMemoryGb: 4,
      cpuCores: 8,
      coarsePointer: true,
    });

    expect(profile.name).toBe("low-power");
    expect(profile.dpr[1]).toBe(1);
    expect(profile.ptlResolution).toBe(64);
    expect(profile.shadows).toBe(false);
    expect(profile.antialias).toBe(false);
  });
});
