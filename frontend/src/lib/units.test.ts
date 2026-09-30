import { describe, expect, it } from "vitest";
import { formatLength, fromMeters, toMeters } from "./units";

describe("length units", () => {
  it("converts metres to centimetres and feet at display precision", () => {
    expect(fromMeters(1.2, "cm")).toBe(120);
    expect(fromMeters(0.9144, "ft")).toBe(3);
    expect(fromMeters(0.2, "ft")).toBe(0.66);
  });

  it("round-trips input values back to metres", () => {
    expect(toMeters(120, "cm")).toBeCloseTo(1.2);
    expect(toMeters(3, "ft")).toBeCloseTo(0.9144);
  });

  it("formats with the unit suffix", () => {
    expect(formatLength(0.05, "cm")).toBe("5 cm");
    expect(formatLength(1.2192, "ft")).toBe("4 ft");
  });
});
