import { afterEach, describe, expect, it } from "vitest";
import { BrowserFarmRepository } from "./browserRepository";
import { createArchive } from "./archive";
import { pilotData } from "./pilotData";

describe("BrowserFarmRepository contract", () => {
  afterEach(async () => {
    await new BrowserFarmRepository("grownerve-test").destroy();
  });

  it("persists state and restores an exported archive", async () => {
    const repository = new BrowserFarmRepository("grownerve-test");
    await repository.replace(pilotData());
    const before = await repository.load();
    const archive = createArchive(before!, { now: "2026-09-01T12:00:00Z", exportId: "01990a20-6a00-7000-8000-000000000001" });
    await repository.clear();
    await repository.importReplace(archive);
    expect(await repository.load()).toEqual(before);
  });

  it("leaves existing state unchanged after a failed import", async () => {
    const repository = new BrowserFarmRepository("grownerve-test");
    await repository.replace(pilotData());
    const before = await repository.load();
    await expect(repository.importReplace({ format: "wrong" })).rejects.toThrow();
    expect(await repository.load()).toEqual(before);
  });
});
