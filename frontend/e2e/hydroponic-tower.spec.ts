import { expect, test } from "@playwright/test";

test("shows the modular tower and changes its level count", async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("./");
  await page.getByRole("button", { name: "Load pilot example" }).click();
  await expect(page.getByRole("heading", { name: "Farm overview" })).toBeVisible();
  if ((page.viewportSize()?.width ?? 1280) <= 780) await page.getByRole("button", { name: "Open navigation" }).click();
  await page.locator(".gn-sidebar").getByRole("button", { name: "3D Twin", exact: true }).click();
  await expect(page.locator(".gn-renderer-badge")).toContainText(/WebGPU|WebGL/, { timeout: 60_000 });
  await page.getByRole("button", { name: "Hydroponic tower", exact: true }).click();
  await expect(page.locator("canvas")).toHaveAttribute("data-tower-levels", "6", { timeout: 60_000 });
  await expect(page.locator("canvas")).toHaveAttribute("data-tower-sites", "18");
  await page.screenshot({ path: testInfo.outputPath("tower-six-levels.png"), fullPage: true });
  await page.getByLabel("Tower levels").selectOption("8");
  await expect(page.locator("canvas")).toHaveAttribute("data-tower-sites", "24");
  await page.screenshot({ path: testInfo.outputPath("tower-eight-levels.png"), fullPage: true });
  await page.getByLabel("Tower levels").selectOption("3");
  await expect(page.locator("canvas")).toHaveAttribute("data-tower-sites", "9");
  await page.getByRole("group", { name: "3D view" }).getByRole("button", { name: "Farm", exact: true }).click();
  await expect(page.getByRole("button", { name: "Look inside reservoir" })).toBeVisible();
  expect(errors).toEqual([]);
});
