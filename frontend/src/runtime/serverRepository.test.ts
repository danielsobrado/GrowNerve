import { afterEach, describe, expect, it, vi } from "vitest";
import { pilotData } from "./pilotData";
import { ServerFarmRepository } from "./serverRepository";

afterEach(() => vi.unstubAllGlobals());

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

async function eventually(predicate: () => boolean, message: string): Promise<void> {
  for (let attempt = 0; attempt < 200; attempt += 1) {
    if (predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  throw new Error(message);
}

describe("ServerFarmRepository contract", () => {
  it("loads absent and present state while retaining the farm version", async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(pilotData()), {
        status: 200,
        headers: { "X-Farm-Version": "7", "Content-Type": "application/json" },
      }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");
    expect(await repository.load()).toBeUndefined();
    expect((await repository.load())?.facilities[0].name).toBe("Home Indoor Farm");
  });

  it("sends the farm version on an existing-state replacement", async () => {
    const data = pilotData();
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(data), {
        status: 200,
        headers: { "X-Farm-Version": "7", "Content-Type": "application/json" },
      }))
      .mockResolvedValueOnce(new Response(null, { status: 204, headers: { "X-Farm-Version": "8" } }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");
    await repository.load();
    await repository.replace(data);

    const [, init] = fetch.mock.calls[1] as [string, RequestInit];
    const headers = init.headers as Record<string, string>;
    expect(headers["X-Farm-Version"]).toBe("7");
    expect(headers["If-None-Match"]).toBeUndefined();
  });

  it("uses If-None-Match for first creation instead of an unconditional write", async () => {
    const data = pilotData();
    const fetch = vi.fn().mockResolvedValue(new Response(null, {
      status: 204,
      headers: { "X-Farm-Version": "1" },
    }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");
    await repository.replace(data);

    const [, init] = fetch.mock.calls[0] as [string, RequestInit];
    const headers = init.headers as Record<string, string>;
    expect(headers["If-None-Match"]).toBe("*");
    expect(headers["X-Farm-Version"]).toBeUndefined();
  });

  it("reports state conflicts without silently retrying a whole-state overwrite", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 409 })));
    const repository = new ServerFarmRepository("http://api");
    await expect(repository.replace(pilotData())).rejects.toThrow("changed");
  });

  it("updates through a fresh read and then supports clear", async () => {
    const data = pilotData();
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(data), {
        status: 200,
        headers: { "X-Farm-Version": "10", "Content-Type": "application/json" },
      }))
      .mockResolvedValueOnce(new Response(null, { status: 204, headers: { "X-Farm-Version": "11" } }))
      .mockResolvedValueOnce(new Response(null, { status: 204, headers: { "X-Farm-Version": "12" } }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");

    const updated = await repository.update((draft) => { draft.facilities[0].name = "Changed"; });
    expect(updated.facilities[0].name).toBe("Changed");
    await repository.clear();
    const [, clearInit] = fetch.mock.calls[2] as [string, RequestInit];
    expect((clearInit.headers as Record<string, string>)["X-Farm-Version"]).toBe("11");
  });

  it("issues accepted and safety-rejected commands", async () => {
    const accepted = { id: crypto.randomUUID(), status: "pending" };
    const rejected = { id: crypto.randomUUID(), status: "rejected", reason_code: "COMMAND_VALUE_OUT_OF_RANGE" };
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(accepted), { status: 202, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(rejected), { status: 422, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");

    expect((await repository.issueCommand({ targetChannelId: crypto.randomUUID(), value: 50, reason: "test" })).status).toBe("pending");
    expect((await repository.issueCommand({ targetChannelId: crypto.randomUUID(), value: 5, reason: "test" })).reason_code).toBe("COMMAND_VALUE_OUT_OF_RANGE");
  });

  it("returns an expired session to the authentication gate once", async () => {
    const unauthorized = vi.fn();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("", { status: 401 })));
    const repository = new ServerFarmRepository("http://api", { onUnauthorized: unauthorized });

    await expect(repository.load()).rejects.toThrow("Sign in again");
    await expect(repository.load()).rejects.toThrow("Sign in again");
    expect(unauthorized).toHaveBeenCalledTimes(1);
  });

  it("does not reconnect a live stream after authentication expires", async () => {
    const unauthorized = vi.fn();
    const fetch = vi.fn().mockResolvedValue(new Response("", { status: 401 }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api", { onUnauthorized: unauthorized, reconnectDelay: 1 });

    const unsubscribe = repository.subscribe(() => undefined);
    await eventually(() => unauthorized.mock.calls.length === 1, "stream 401 did not expire the session");
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(fetch).toHaveBeenCalledTimes(1);
    unsubscribe();
  });

  it("sends the bearer credential on reads and the live stream", async () => {
    const fetch = vi.fn().mockImplementation((url: string) => {
      if (url.includes("/stream")) return Promise.resolve(eventStream(["event: ready\ndata: {}\n\n"]));
      return Promise.resolve(new Response(JSON.stringify(pilotData()), {
        status: 200,
        headers: { "X-Farm-Version": "1", "Content-Type": "application/json" },
      }));
    });
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api", { token: "secret-token", reconnectDelay: 10_000 });
    await repository.load();
    const unsubscribe = repository.subscribe(() => undefined);
    await eventually(() => fetch.mock.calls.length >= 2, "the stream was never opened");

    for (const [url, init] of fetch.mock.calls as [string, RequestInit][]) {
      expect((init.headers as Record<string, string>).Authorization).toBe("Bearer secret-token");
      expect(url).not.toContain("secret-token");
    }
    unsubscribe();
  });

  it("emits changes from complete and split server-sent-event frames", async () => {
    const fetch = vi.fn().mockResolvedValue(eventStream([
      "event: cha",
      'nge\ndata: {"topic":"alerts"}\n\n',
      'event: change\ndata: {"topic":"measurements"}\n\n',
    ]));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api", { reconnectDelay: 10_000 });
    let notifications = 0;
    const unsubscribe = repository.subscribe(() => { notifications += 1; });
    await eventually(() => notifications >= 2, "change frames were dropped");
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
      .mockResolvedValueOnce(new Response(JSON.stringify({ measurements: [{ channel_id: "c1", value: 21 }] }), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ buckets: [{ started_at: "2026-05-04T12:00:00Z", average: 21, minimum: 20, maximum: 22, samples: 10 }] }), { status: 200, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");

    const from = new Date("2026-05-04T00:00:00Z");
    expect(await repository.history({ channelId: "c1", from, limit: 100 })).toHaveLength(1);
    expect(String(fetch.mock.calls[0][0])).toContain("limit=100");
    expect((await repository.historyBuckets({ channelId: "c1", bucketSeconds: 600 }))[0].samples).toBe(10);
    expect(String(fetch.mock.calls[1][0])).toContain("bucketSeconds=600");
  });
});
