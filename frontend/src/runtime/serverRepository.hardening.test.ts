import { afterEach, describe, expect, it, vi } from "vitest";
import { pilotData } from "./pilotData";
import { ServerFarmRepository } from "./serverRepository";

afterEach(() => vi.unstubAllGlobals());

describe("ServerFarmRepository hardening", () => {
  it("reuses one idempotency key when a command transport attempt is ambiguous", async () => {
    const accepted = { id: crypto.randomUUID(), status: "pending" };
    const fetch = vi.fn()
      .mockRejectedValueOnce(new TypeError("network interrupted"))
      .mockResolvedValueOnce(new Response(JSON.stringify(accepted), {
        status: 202,
        headers: { "Content-Type": "application/json" },
      }));
    vi.stubGlobal("fetch", fetch);
    const repository = new ServerFarmRepository("http://api/");

    await repository.issueCommand({ targetChannelId: crypto.randomUUID(), value: 50, reason: "test" });

    expect(fetch).toHaveBeenCalledTimes(2);
    const first = (fetch.mock.calls[0][1] as RequestInit).headers as Record<string, string>;
    const second = (fetch.mock.calls[1][1] as RequestInit).headers as Record<string, string>;
    expect(first["Idempotency-Key"]).toBeTruthy();
    expect(second["Idempotency-Key"]).toBe(first["Idempotency-Key"]);
    expect(String(fetch.mock.calls[0][0])).toBe("http://api/api/v1/commands");
    expect(String(fetch.mock.calls[1][0])).toBe("http://api/api/v1/commands");
  });

  it("fails closed when a state read loses the concurrency version header", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify(pilotData()), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    })));
    const repository = new ServerFarmRepository("http://api");

    await expect(repository.load()).rejects.toThrow("farm version");
  });

  it("fails closed when a successful state write loses the new version header", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 204 })));
    const repository = new ServerFarmRepository("http://api");

    await expect(repository.replace(pilotData())).rejects.toThrow("farm version");
  });
});
