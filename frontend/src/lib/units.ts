import { useSyncExternalStore } from "react";

export type LengthUnit = "cm" | "ft";

const STORAGE_KEY = "grownerve.lengthUnit";
const METERS_PER_FOOT = 0.3048;
const listeners = new Set<() => void>();

function readUnit(): LengthUnit {
  try { return localStorage.getItem(STORAGE_KEY) === "ft" ? "ft" : "cm"; } catch { return "cm"; }
}

let current: LengthUnit = readUnit();

function subscribe(listener: () => void) {
  listeners.add(listener);
  const onStorage = (event: StorageEvent) => { if (event.key === STORAGE_KEY) { current = readUnit(); listener(); } };
  window.addEventListener("storage", onStorage);
  return () => { listeners.delete(listener); window.removeEventListener("storage", onStorage); };
}

export function setLengthUnit(unit: LengthUnit) {
  current = unit;
  try { localStorage.setItem(STORAGE_KEY, unit); } catch { /* The preference still applies for this session. */ }
  listeners.forEach((listener) => listener());
}

export function useLengthUnit(): LengthUnit {
  return useSyncExternalStore(subscribe, () => current, () => "cm");
}

/** Input step and display precision for each unit. */
export const lengthStep: Record<LengthUnit, number> = { cm: 1, ft: 0.01 };

/** Converts metres to the display unit, rounded to that unit's precision. */
export function fromMeters(meters: number, unit: LengthUnit): number {
  return unit === "cm" ? Math.round(meters * 100) : Math.round((meters / METERS_PER_FOOT) * 100) / 100;
}

export function toMeters(value: number, unit: LengthUnit): number {
  return unit === "cm" ? value / 100 : value * METERS_PER_FOOT;
}

export function formatLength(meters: number, unit: LengthUnit): string {
  return `${fromMeters(meters, unit)} ${unit}`;
}
