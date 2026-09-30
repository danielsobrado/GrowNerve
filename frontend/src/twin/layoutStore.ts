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

/**
 * Replaces every saved layout with the given set (an archive import replaces the whole farm, so layouts
 * of the previous farm must not survive). Storage failures are ignored: the farm data is already imported.
 */
export function writeStoredLayouts(layouts: StoredLayout[]): void {
  clearStoredLayouts();
  try { for (const entry of layouts) localStorage.setItem(layoutStorageKey(entry.facility_id), JSON.stringify(entry.layout)); }
  catch { /* Storage unavailable or full: the layouts are browser-side extras. */ }
}

export function clearStoredLayouts(): void {
  try { for (const key of storageKeys()) localStorage.removeItem(key); } catch { /* Storage unavailable: nothing to clear. */ }
}
