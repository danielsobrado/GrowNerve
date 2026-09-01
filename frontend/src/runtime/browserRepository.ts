import Dexie, { type EntityTable } from "dexie";
import type { FarmData } from "../domain/model";
import { validateArchive } from "./archive";

interface Snapshot { key: "current"; data: FarmData; updated_at: string }

export interface FarmRepository {
  load(): Promise<FarmData | undefined>;
  replace(data: FarmData): Promise<void>;
  update(mutator: (draft: FarmData) => void): Promise<FarmData>;
  clear(): Promise<void>;
  subscribe(listener: () => void): () => void;
}

export class BrowserFarmRepository implements FarmRepository {
  private readonly database: Dexie & { snapshots: EntityTable<Snapshot, "key"> };
  private readonly changes = new EventTarget();

  constructor(name = "grownerve") {
    this.database = new Dexie(name) as Dexie & { snapshots: EntityTable<Snapshot, "key"> };
    this.database.version(1).stores({ snapshots: "key,updated_at" });
  }

  async load(): Promise<FarmData | undefined> {
    return structuredClone((await this.database.snapshots.get("current"))?.data);
  }

  async replace(data: FarmData): Promise<void> {
    await this.database.transaction("rw", this.database.snapshots, async () => {
      await this.database.snapshots.put({ key: "current", data: structuredClone(data), updated_at: new Date().toISOString() });
    });
    this.changes.dispatchEvent(new Event("changed"));
  }

  async update(mutator: (draft: FarmData) => void): Promise<FarmData> {
    let next: FarmData | undefined;
    await this.database.transaction("rw", this.database.snapshots, async () => {
      const current = await this.database.snapshots.get("current");
      if (!current) throw new Error("No local farm exists");
      next = structuredClone(current.data);
      mutator(next);
      await this.database.snapshots.put({ key: "current", data: next, updated_at: new Date().toISOString() });
    });
    this.changes.dispatchEvent(new Event("changed"));
    return structuredClone(next!);
  }

  async importReplace(input: unknown): Promise<void> {
    const archive = validateArchive(input);
    await this.replace(archive.data);
  }

  async clear(): Promise<void> {
    await this.database.transaction("rw", this.database.snapshots, () => this.database.snapshots.clear());
    this.changes.dispatchEvent(new Event("changed"));
  }

  subscribe(listener: () => void): () => void {
    this.changes.addEventListener("changed", listener);
    return () => this.changes.removeEventListener("changed", listener);
  }

  async storageEstimate(): Promise<{ usage: number; quota: number }> {
    const estimate = await navigator.storage?.estimate?.();
    return { usage: estimate?.usage ?? 0, quota: estimate?.quota ?? 0 };
  }

  async destroy(): Promise<void> {
    this.database.close();
    await Dexie.delete(this.database.name);
  }
}
