import { emptyFarmData, type FarmCommand, type FarmData, type Measurement } from "../domain/model";
import type { CommandIntent } from "./simulator";
import type { FarmRepository } from "./browserRepository";

/** One aggregated interval of a channel's history. */
export interface MeasurementBucket {
  started_at: string;
  average: number;
  minimum: number;
  maximum: number;
  samples: number;
}

/** Bounds a history read. The server clamps anything beyond its own maximum. */
export interface HistoryQuery {
  channelId: string;
  from?: Date;
  to?: Date;
  limit?: number;
  bucketSeconds?: number;
}

export interface ServerRepositoryOptions {
  /** Bearer credential. The server rejects unauthenticated API requests. */
  token?: string;
  /**
   * Reconnect delay for the live-update stream, in milliseconds. Exposed so
   * tests do not have to wait out a real backoff.
   */
  reconnectDelay?: number;
}

/**
 * Talks to the Go server. Changes arrive over a server-sent-event stream rather
 * than by polling, so the UI reflects telemetry as it lands instead of up to a
 * poll interval late.
 */
export class ServerFarmRepository implements FarmRepository {
  private readonly listeners = new Set<() => void>();
  private readonly options: ServerRepositoryOptions;
  private etag?: string;
  private streamAbort?: AbortController;
  private reconnectTimer?: ReturnType<typeof setTimeout>;

  constructor(private readonly baseURL = "http://127.0.0.1:8080", options: ServerRepositoryOptions = {}) {
    this.options = { reconnectDelay: 3000, ...options };
  }

  private headers(extra: Record<string, string> = {}): Record<string, string> {
    const headers: Record<string, string> = { Accept: "application/json", ...extra };
    if (this.options.token) headers.Authorization = `Bearer ${this.options.token}`;
    return headers;
  }

  /** Turns a problem-details response into a message worth showing a person. */
  private async failure(response: Response, fallback: string): Promise<Error> {
    try {
      const problem = await response.json() as { detail?: string; code?: string };
      if (problem.detail) return new Error(problem.detail);
    } catch {
      // A non-JSON body is not itself the interesting failure; fall through.
    }
    if (response.status === 401) return new Error("Sign in again to continue.");
    if (response.status === 403) return new Error("Your role does not permit that action.");
    if (response.status === 429) return new Error("Too many requests. Wait a moment and try again.");
    return new Error(`${fallback}: ${response.status}`);
  }

  async load(): Promise<FarmData | undefined> {
    const response = await fetch(`${this.baseURL}/api/v1/state`, { headers: this.headers() });
    if (response.status === 204) return undefined;
    if (!response.ok) throw await this.failure(response, "Server state failed");
    this.etag = response.headers.get("ETag") ?? undefined;
    return await response.json() as FarmData;
  }

  async replace(data: FarmData): Promise<void> {
    const headers = this.headers({ "Content-Type": "application/json" });
    if (this.etag) headers["If-Match"] = this.etag;
    const response = await fetch(`${this.baseURL}/api/v1/state`, { method: "PUT", headers, body: JSON.stringify(data) });
    if (response.status === 409) throw new Error("Farm state changed on the server; reload before saving");
    if (!response.ok) throw await this.failure(response, "Server state save failed");
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
    const response = await fetch(`${this.baseURL}/api/v1/commands`, {
      method: "POST",
      headers: this.headers({ "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }),
      body: JSON.stringify({ targetChannelId: intent.targetChannelId, value: intent.value, reason: intent.reason }),
    });
    // 422 is a safety refusal, which is a real answer the operator needs to see
    // rather than a transport failure to retry.
    if (!response.ok && response.status !== 422) throw await this.failure(response, "Command failed");
    this.emit();
    return await response.json() as FarmCommand;
  }

  /** Reads bounded history for one channel. */
  async history(query: HistoryQuery): Promise<Measurement[]> {
    const url = this.historyURL(query);
    const response = await fetch(url, { headers: this.headers() });
    if (!response.ok) throw await this.failure(response, "History failed");
    const payload = await response.json() as { measurements?: Measurement[] };
    return payload.measurements ?? [];
  }

  /** Reads server-aggregated history, for charts spanning more than a few hours. */
  async historyBuckets(query: HistoryQuery & { bucketSeconds: number }): Promise<MeasurementBucket[]> {
    const response = await fetch(this.historyURL(query), { headers: this.headers() });
    if (!response.ok) throw await this.failure(response, "History failed");
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

  /**
   * Opens the change stream over fetch rather than EventSource. EventSource
   * cannot send an Authorization header, and putting a credential in the URL
   * would leak it into proxy and server logs; reading the stream with fetch
   * keeps the token where every other request carries it.
   */
  private openStream(): void {
    if (this.streamAbort || typeof fetch === "undefined") return;
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
      if (!response.ok || !response.body) throw new Error(`stream ${response.status}`);
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        // Events are separated by a blank line; anything after the last one is a
        // partial frame that has to wait for more bytes.
        const frames = buffer.split("\n\n");
        buffer = frames.pop() ?? "";
        for (const frame of frames) {
          if (frame.includes("event: change")) this.emit();
        }
      }
      throw new Error("stream closed");
    } catch (error) {
      if (abort.signal.aborted) return;
      this.scheduleReconnect(abort, error);
    }
  }

  private scheduleReconnect(abort: AbortController, error: unknown): void {
    if (this.streamAbort !== abort) return;
    this.streamAbort = undefined;
    if (this.listeners.size === 0) return;
    // A dropped stream means the client may have missed a change, so a refresh
    // is scheduled alongside the reconnect rather than waiting for the next one.
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
