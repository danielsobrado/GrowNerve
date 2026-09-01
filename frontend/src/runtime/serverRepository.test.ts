import { afterEach, describe, expect, it, vi } from "vitest";
import { pilotData } from "./pilotData";
import { ServerFarmRepository } from "./serverRepository";

afterEach(() => vi.unstubAllGlobals());

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

  it("issues accepted and rejected commands and emits change notifications", async () => {
    const command = { id: crypto.randomUUID(), status: "pending" };
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(command), { status: 202, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ detail: "offline" }), { status: 503, headers: { "Content-Type": "application/json" } }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api");
    let calls = 0;
    const unsubscribe = repository.subscribe(() => { calls += 1; });
    expect((await repository.issueCommand({ targetChannelId: crypto.randomUUID(), value: 50, reason: "test" })).status).toBe("pending");
    expect(calls).toBe(1);
    await expect(repository.issueCommand({ targetChannelId: crypto.randomUUID(), value: 50, reason: "test" })).rejects.toThrow("offline");
    unsubscribe();
  });
});
