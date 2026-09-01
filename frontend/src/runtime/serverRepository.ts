import { emptyFarmData, type FarmCommand, type FarmData, type Measurement } from "../domain/model";
import type { CommandIntent } from "./simulator";
import type { FarmRepository } from "./browserRepository";

export interface MeasurementBucket {
  started_at: string;
  average: number;
  minimum: number;
  maximum: number;
  samples: number;
}

export interface HistoryQuery {
  channelId: string;
  from?: Date;
  to?: Date;
  limit?: number;
  bucketSeconds?: number;
}

export interface ServerRepositoryOptions {
  /** Read the current bearer credential for every request. */
  token?: string | (() => string | undefined);
  reconnectDelay?: number;
  onUnauthorized?: () => void;
}

export class ServerFarmRepository implements FarmRepository {
  private readonly listeners = new Set<() => void>();
  private readonly options: ServerRepositoryOptions;
  private version?: string;
  private streamAbort?: AbortController;
  private reconnectTimer?: ReturnType<typeof setTimeout>;
  private unauthorizedHandled = false;

  constructor(private readonly baseURL = "http://127.0.0.1:8080", options: ServerRepositoryOptions = {}) {
    this.options = { reconnectDelay: 3000, ...options };
  }

  private token(): string | undefined {
    return typeof this.options.token === "function" ? this.options.token() : this.options.token;
  }

  private headers(extra: Record<string, string> = {}): Record<string, string> {
    const headers: Record<string, string> = { Accept: "application/json", ...extra };
    const token = this.token();
    if (token) headers.Authorization = `Bearer ${token}`;
    return headers;
  }

  private unauthorized(): void {
    if (this.unauthorizedHandled) return;
    this.unauthorizedHandled = true;
    this.closeStream();
    this.options.onUnauthorized?.();
  }

  private async failure(response: Response, fallback: string): Promise<Error> {
    if (response.status === 401) this.unauthorized();
    try {
      const problem = await response.json() as { detail?: string; code?: string };
      if (problem.detail) return new Error(problem.detail);
    } catch {
      // Non-problem responses fall through to the status-specific message.
    }
    if (response.status === 401) return new Error("Sign in again to continue.");
    if (response.status === 403) return new Error("Your role does not permit that action.");
    if (response.status === 429) return new Error("Too many requests. Wait a moment and try again.");
    return new Error(`${fallback}: ${response.status}`);
  }

  async load(): Promise<FarmData | undefined> {
    const response = await fetch(`${this.baseURL}/api/v1/state`, { headers: this.headers() });
    if (response.status === 204) {
      this.version = undefined;
      return undefined;
    }
    if (!response.ok) throw await this.failure(response, "Server state failed");
    this.unauthorizedHandled = false;
    this.version = response.headers.get("X-Farm-Version") ?? undefined;
    return await response.json() as FarmData;
  }

  async replace(data: FarmData): Promise<void> {
    const headers = this.headers({ "Content-Type": "application/json" });
    if (this.version) headers["X-Farm-Version"] = this.version;
    else headers["If-None-Match"] = "*";
    const response = await fetch(`${this.baseURL}/api/v1/state`, { method: "PUT", headers, body: JSON.stringify(data) });
    if (response.status === 409 || response.status === 428) throw new Error("Farm state changed on the server; reload before saving");
    if (!response.ok) throw await this.failure(response, "Server state save failed");
    this.unauthorizedHandled = false;
    this.version = response.headers.get("X-Farm-Version") ?? undefined;
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
    const response = await fetch(`${this.baseURL}/api/v1/commands`, {
      method: "POST",
      headers: this.headers({ "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }),
      body: JSON.stringify({ targetChannelId: intent.targetChannelId, value: intent.value, reason: intent.reason }),
    });
    if (!response.ok && response.status !== 422) throw await this.failure(response, "Command failed");
    this.unauthorizedHandled = false;
    this.emit();
    return await response.json() as FarmCommand;
  }

  async history(query: HistoryQuery): Promise<Measurement[]> {
    const response = await fetch(this.historyURL(query), { headers: this.headers() });
    if (!response.ok) throw await this.failure(response, "History failed");
    this.unauthorizedHandled = false;
    const payload = await response.json() as { measurements?: Measurement[] };
    return payload.measurements ?? [];
  }

  async historyBuckets(query: HistoryQuery & { bucketSeconds: number }): Promise<MeasurementBucket[]> {
    const response = await fetch(this.historyURL(query), { headers: this.headers() });
    if (!response.ok) throw await this.failure(response, "History failed");
    this.unauthorizedHandled = false;
    const payload = await response.json() as { buckets?: MeasurementBucket[] };
    return payload.buckets ?? [];
  }

  private historyURL(query: HistoryQuery): string {
    const parameters = new URLSearchParams({ channelId: query.channelId });
    if (query.from) parameters.set("from", query.from.toISOString());
    if (query.to) parameters.set("to", query.to.toISOString());
    if (query.limit) parameters.set("limit", String(query.limit));
    if (query.bucketSeconds) parameters.set("bucketSeconds", String(query.bucketSeconds));
    return `${this.baseURL}/api/v1/measurements/history?${parameters}`;
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    this.openStream();
    return () => {
      this.listeners.delete(listener);
      if (this.listeners.size === 0) this.closeStream();
    };
  }

  private openStream(): void {
    if (this.streamAbort || typeof fetch === "undefined" || this.unauthorizedHandled) return;
    const abort = new AbortController();
    this.streamAbort = abort;
    void this.readStream(abort);
  }

  private async readStream(abort: AbortController): Promise<void> {
    try {
      const response = await fetch(`${this.baseURL}/api/v1/stream`, {
        headers: this.headers({ Accept: "text/event-stream" }),
        signal: abort.signal,
      });
      if (!response.ok) throw await this.failure(response, "Live updates failed");
      if (!response.body) throw new Error("Live updates returned no response body");
      this.unauthorizedHandled = false;
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const frames = buffer.split("\n\n");
        buffer = frames.pop() ?? "";
        for (const frame of frames) {
          if (frame.includes("event: change")) this.emit();
        }
      }
      throw new Error("stream closed");
    } catch (error) {
      if (abort.signal.aborted || this.unauthorizedHandled) return;
      this.scheduleReconnect(abort, error);
    }
  }

  private scheduleReconnect(abort: AbortController, error: unknown): void {
    if (this.streamAbort !== abort) return;
    this.streamAbort = undefined;
    if (this.listeners.size === 0 || this.unauthorizedHandled) return;
    this.emit();
    console.warn("GrowNerve live updates interrupted; reconnecting", error);
    this.reconnectTimer = setTimeout(() => this.openStream(), this.options.reconnectDelay);
  }

  private closeStream(): void {
    this.streamAbort?.abort();
    this.streamAbort = undefined;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = undefined;
  }

  private emit() { this.listeners.forEach((listener) => listener()); }
}
