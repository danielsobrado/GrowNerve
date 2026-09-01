import { emptyFarmData, type FarmCommand, type FarmData } from "../domain/model";
import type { CommandIntent } from "./simulator";
import type { FarmRepository } from "./browserRepository";

export class ServerFarmRepository implements FarmRepository {
  private readonly listeners = new Set<() => void>();
  private etag?: string;
  private poller?: number;

  constructor(private readonly baseURL = "http://127.0.0.1:8080") {}

  async load(): Promise<FarmData | undefined> {
    const response = await fetch(`${this.baseURL}/api/v1/state`, { headers: { Accept: "application/json" } });
    if (response.status === 204) return undefined;
    if (!response.ok) throw new Error(`Server state failed: ${response.status}`);
    this.etag = response.headers.get("ETag") ?? undefined;
    return await response.json() as FarmData;
  }

  async replace(data: FarmData): Promise<void> {
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (this.etag) headers["If-Match"] = this.etag;
    const response = await fetch(`${this.baseURL}/api/v1/state`, { method: "PUT", headers, body: JSON.stringify(data) });
    if (response.status === 409) throw new Error("Farm state changed on the server; reload before saving");
    if (!response.ok) throw new Error(`Server state save failed: ${response.status}`);
    this.etag = response.headers.get("ETag") ?? undefined;
    this.emit();
  }

  async update(mutator: (draft: FarmData) => void): Promise<FarmData> {
    const current = await this.load();
    if (!current) throw new Error("No server farm exists");
    const next = structuredClone(current);
    mutator(next);
    await this.replace(next);
    return next;
  }

  async clear(): Promise<void> { await this.replace(emptyFarmData()); }

  async issueCommand(intent: CommandIntent): Promise<FarmCommand> {
    const response = await fetch(`${this.baseURL}/api/v1/commands`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify({ targetChannelId: intent.targetChannelId, value: intent.value, reason: intent.reason }) });
    const result = await response.json() as FarmCommand & { detail?: string };
    if (!response.ok && response.status !== 422) throw new Error(result.detail ?? `Command failed: ${response.status}`);
    this.emit();
    return result;
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    if (!this.poller && typeof window !== "undefined") this.poller = window.setInterval(() => this.emit(), 10_000);
    return () => { this.listeners.delete(listener); if (this.listeners.size === 0 && this.poller) { window.clearInterval(this.poller); this.poller = undefined; } };
  }

  private emit() { this.listeners.forEach((listener) => listener()); }
}
