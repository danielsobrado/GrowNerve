import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { App } from "./App";
import { BrowserFarmRepository } from "./runtime/browserRepository";

let repository: BrowserFarmRepository | undefined;

afterEach(async () => {
  await repository?.destroy();
  repository = undefined;
});

describe("GrowNerve application", () => {
  it("does not silently seed an empty browser runtime", async () => {
    repository = new BrowserFarmRepository("grownerve-app-test-empty");
    render(<App repository={repository} runtimeMode="browser" />);
    expect(await screen.findByRole("heading", { name: "Welcome to GrowNerve" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Load pilot example" })).toBeVisible();
  });

  it("loads the pilot and exposes the operational navigation", async () => {
    repository = new BrowserFarmRepository("grownerve-app-test-pilot");
    render(<App repository={repository} runtimeMode="browser" />);
    fireEvent.click(await screen.findByRole("button", { name: "Load pilot example" }));
    await waitFor(() => expect(screen.getByRole("heading", { name: "Farm overview" })).toBeVisible());
    expect(screen.getByRole("button", { name: /3D Twin/ })).toBeVisible();
    expect(screen.getByText("Browser only")).toBeVisible();
  });
});
