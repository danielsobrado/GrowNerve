import { afterEach, describe, expect, it, vi } from "vitest";
import { pilotData } from "./pilotData";
import { ServerFarmRepository } from "./serverRepository";

afterEach(() => vi.unstubAllGlobals());

/** Builds a response body that streams the given server-sent-event frames. */
function eventStream(frames: string[]): Response {
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      const encoder = new TextEncoder();
      for (const frame of frames) controller.enqueue(encoder.encode(frame));
      controller.close();
    },
  });
  return new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

/** Waits until predicate holds, so tests assert on outcomes rather than timing. */
async function eventually(predicate: () => boolean, message: string): Promise<void> {
  for (let attempt = 0; attempt < 200; attempt += 1) {
    if (predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  throw new Error(message);
}

describe("ServerFarmRepository contract", () => {
  it("loads absent and present state while retaining the ETag", async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(pilotData()), { status: 200, headers: { ETag: '"v1"', "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");
    expect(await repository.load()).toBeUndefined();
    expect((await repository.load())?.facilities[0].name).toBe("Home Indoor Farm");
  });

  it("saves, updates, clears, and reports concurrency conflicts", async () => {
    const data = pilotData();
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(data), { status: 200, headers: { ETag: '"v1"', "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(null, { status: 204, headers: { ETag: '"v2"' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(data), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(null, { status: 409 }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");
    await repository.load();
    await repository.replace(data);
    expect((await repository.update((draft) => { draft.facilities[0].name = "Changed"; })).facilities[0].name).toBe("Changed");
    await repository.clear();
    await expect(repository.replace(data)).rejects.toThrow("changed");
  });

  it("sends the ETag it read back as If-Match so a stale write is refused", async () => {
    const data = pilotData();
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(data), { status: 200, headers: { ETag: '"v7"', "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(null, { status: 204, headers: { ETag: '"v8"' } }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");
    await repository.load();
    await repository.replace(data);
    const [, init] = fetch.mock.calls[1] as [string, RequestInit];
    expect((init.headers as Record<string, string>)["If-Match"]).toBe('"v7"');
  });

  it("issues accepted and rejected commands and emits change notifications", async () => {
    const command = { id: crypto.randomUUID(), status: "pending" };
    // The stream and the command share one mock, so it answers by URL rather
    // than by call order.
    let commandCalls = 0;
    const fetch = vi.fn().mockImplementation((url: string) => {
      if (String(url).includes("/stream")) return Promise.resolve(eventStream(["event: ready\ndata: {}\n\n"]));
      commandCalls += 1;
      return Promise.resolve(commandCalls === 1
        ? new Response(JSON.stringify(command), { status: 202, headers: { "Content-Type": "application/json" } })
        : new Response(JSON.stringify({ detail: "The physical provider is offline" }), { status: 503, headers: { "Content-Type": "application/json" } }));
    });
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api", { reconnectDelay: 10_000 });
    let calls = 0;
    const unsubscribe = repository.subscribe(() => { calls += 1; });
    expect((await repository.issueCommand({ targetChannelId: crypto.randomUUID(), value: 50, reason: "test" })).status).toBe("pending");
    expect(calls).toBeGreaterThanOrEqual(1);
    await expect(repository.issueCommand({ targetChannelId: crypto.randomUUID(), value: 50, reason: "test" })).rejects.toThrow("offline");
    unsubscribe();
  });

  it("returns a safety refusal to the caller instead of throwing", async () => {
    // A 422 is a real answer the operator has to see, not a transport failure.
    const refusal = { id: crypto.randomUUID(), status: "rejected", reason_code: "COMMAND_VALUE_OUT_OF_RANGE" };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      new Response(JSON.stringify(refusal), { status: 422, headers: { "Content-Type": "application/json" } })));
    const repository = new ServerFarmRepository("http://api");
    const result = await repository.issueCommand({ targetChannelId: crypto.randomUUID(), value: 5, reason: "too low" });
    expect(result.status).toBe("rejected");
    expect(result.reason_code).toBe("COMMAND_VALUE_OUT_OF_RANGE");
  });

  it("explains authentication and throttling failures in words an operator can act on", async () => {
    for (const [status, expected] of [[401, "Sign in again"], [403, "does not permit"], [429, "Too many requests"]] as const) {
      vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("", { status })));
      const repository = new ServerFarmRepository("http://api");
      await expect(repository.load()).rejects.toThrow(expected);
    }
  });

  it("sends the bearer credential on every request, including the change stream", async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(pilotData()), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api", { token: "secret-token" });
    await repository.load();
    const unsubscribe = repository.subscribe(() => undefined);
    await eventually(() => fetch.mock.calls.length >= 2, "the stream was never opened");
    for (const [url, init] of fetch.mock.calls as [string, RequestInit][]) {
      expect((init.headers as Record<string, string>).Authorization).toBe("Bearer secret-token");
      // A credential in the URL would leak into proxy and server logs.
      expect(url).not.toContain("secret-token");
    }
    unsubscribe();
  });

  it("refreshes on a change event from the live stream rather than on a timer", async () => {
    const fetch = vi.fn().mockResolvedValue(eventStream([
      "event: ready\ndata: {}\n\n",
      'event: change\ndata: {"topic":"measurements"}\n\n',
      'event: change\ndata: {"topic":"alerts"}\n\n',
    ]));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api", { reconnectDelay: 10_000 });
    let notifications = 0;
    const unsubscribe = repository.subscribe(() => { notifications += 1; });
    await eventually(() => notifications >= 2, `change events did not reach the listener (${notifications})`);
    unsubscribe();
  });

  it("reassembles change events split across network chunks", async () => {
    // A frame arriving in pieces must not be missed or double-counted.
    const fetch = vi.fn().mockResolvedValue(eventStream(["event: cha", 'nge\ndata: {"topic":"alerts"}', "\n\n"]));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api", { reconnectDelay: 10_000 });
    let notifications = 0;
    const unsubscribe = repository.subscribe(() => { notifications += 1; });
    await eventually(() => notifications >= 1, "a split change frame was dropped");
    unsubscribe();
  });

  it("stops streaming when the last subscriber leaves", async () => {
    const aborted = vi.fn();
    const fetch = vi.fn().mockImplementation((_url: string, init: RequestInit) => {
      init.signal?.addEventListener("abort", aborted);
      return Promise.resolve(eventStream(["event: ready\ndata: {}\n\n"]));
    });
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api", { reconnectDelay: 10_000 });
    const unsubscribe = repository.subscribe(() => undefined);
    await eventually(() => fetch.mock.calls.length >= 1, "the stream was never opened");
    unsubscribe();
    expect(aborted).toHaveBeenCalled();
  });

  it("reads bounded history and server-side aggregates", async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ channelId: "c1", measurements: [{ channel_id: "c1", value: 21 }] }), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ channelId: "c1", bucketSeconds: 600, buckets: [{ started_at: "2026-05-04T12:00:00Z", average: 21, minimum: 20, maximum: 22, samples: 10 }] }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");

    const from = new Date("2026-05-04T00:00:00Z");
    expect(await repository.history({ channelId: "c1", from, limit: 100 })).toHaveLength(1);
    expect(String(fetch.mock.calls[0][0])).toContain("channelId=c1");
    expect(String(fetch.mock.calls[0][0])).toContain("limit=100");

    const buckets = await repository.historyBuckets({ channelId: "c1", bucketSeconds: 600 });
    expect(buckets[0].samples).toBe(10);
    expect(String(fetch.mock.calls[1][0])).toContain("bucketSeconds=600");
  });
});
