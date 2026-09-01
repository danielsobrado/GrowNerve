import { expect, test, type Page } from "@playwright/test";

async function openPilot(page: Page) {
  await page.goto("./");
  await page.getByRole("button", { name: "Load pilot example" }).click();
  await expect(page.getByRole("heading", { name: "Farm overview" })).toBeVisible();
  await expect(page.getByText("Browser simulator is active.")).toBeVisible();
}

async function navigate(page: Page, label: string) {
  const navigation = page.locator(".gn-sidebar");
  if ((page.viewportSize()?.width ?? 1280) <= 780) {
    await page.getByRole("button", { name: "Open navigation" }).click();
  }
  await navigation.getByRole("button", { name: label }).click();
}

test("runs the complete browser-only operational journey", async ({ page }) => {
  await openPilot(page);

  await navigate(page, "Grow Cycles");
  await page.getByPlaceholder("What changed or needs attention?").fill("Roots inspected: healthy and well aerated");
  await page.getByRole("button", { name: "Add observation" }).click();
  await expect(page.getByText("Roots inspected: healthy and well aerated")).toBeVisible();

  await navigate(page, "Devices");
  await page.getByLabel("Circulation Fan output").fill("60");
  await page.getByRole("button", { name: "Apply" }).click();
  await expect(page.getByRole("cell", { name: "applied" }).first()).toBeVisible();
  await expect(page.getByRole("cell", { name: "60" }).first()).toBeVisible();

  await navigate(page, "Alerts");
  await page.getByRole("button", { name: "Acknowledge" }).first().click();
  await expect(page.getByText("acknowledged", { exact: true }).first()).toBeVisible();

  await navigate(page, "Settings");
  await expect(page.getByText("Archive schema")).toBeVisible();
  await expect(page.getByText("Version 1", { exact: true })).toBeVisible();
});

test("renders the digital twin with an accessible non-3D inspector", async ({ page }) => {
  await openPilot(page);
  await navigate(page, "3D Twin");

  await expect(page.getByRole("heading", { name: "3D Twin" })).toBeVisible();
  await expect(page.locator("canvas")).toBeVisible();
  await expect(page.getByText("Select an object")).toBeVisible();
  await expect(page.getByText(/WebGPU available|Fallback renderer/)).toBeVisible();
});

test("keeps the primary workflow usable on a phone viewport", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-chromium", "Mobile-only journey");
  await openPilot(page);
  await navigate(page, "Farm");
  await page.getByRole("textbox", { name: "Search farm" }).fill("reservoir");
  await expect(page.getByText("DWC Reservoir 01", { exact: true })).toBeVisible();
});
