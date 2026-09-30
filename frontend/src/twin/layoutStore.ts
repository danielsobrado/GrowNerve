import { parseTentLayout, type TentLayout } from "./tentLayout";

/** Layouts live in browser storage, one per facility, next to the IndexedDB farm snapshot. */
const PREFIX = "grownerve:tent-layout:v1:";
export const DEFAULT_LAYOUT_OWNER = "default";
export const layoutStorageKey = (facilityId?: string) => `${PREFIX}${facilityId ?? DEFAULT_LAYOUT_OWNER}`;

export interface StoredLayout { facility_id: string; layout: TentLayout }

function storageKeys(): string[] {
  try { return Array.from({ length: localStorage.length }, (_, index) => localStorage.key(index)).filter((key): key is string => key?.startsWith(PREFIX) ?? false); }
  catch { return []; }
}

/** Every valid saved layout, sorted by owner. Unreadable drafts are skipped rather than failing an export. */
export function readStoredLayouts(): StoredLayout[] {
  const layouts: StoredLayout[] = [];
  for (const key of storageKeys()) {
    try { layouts.push({ facility_id: key.slice(PREFIX.length), layout: parseTentLayout(localStorage.getItem(key)) }); } catch { /* Skip invalid drafts. */ }
  }
  return layouts.sort((left, right) => left.facility_id.localeCompare(right.facility_id));
}

export function writeStoredLayouts(layouts: StoredLayout[]): void {
  for (const entry of layouts) localStorage.setItem(layoutStorageKey(entry.facility_id), JSON.stringify(entry.layout));
}

export function clearStoredLayouts(): void {
  try { for (const key of storageKeys()) localStorage.removeItem(key); } catch { /* Storage unavailable: nothing to clear. */ }
}
