import { expect, test } from "@playwright/test";

test("loads LED variants and configurable soil pots", async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("./");
  await page.getByRole("button", { name: "Load pilot example" }).click();
  await expect(page.getByRole("heading", { name: "Farm overview" })).toBeVisible();
  if ((page.viewportSize()?.width ?? 1280) <= 780) await page.getByRole("button", { name: "Open navigation" }).click();
  await page.locator(".gn-sidebar").getByRole("button", { name: "3D Twin", exact: true }).click();
  await page.getByRole("button", { name: "Lights & pots", exact: true }).click();
  await expect(page.locator("canvas")).toHaveAttribute("data-grow-options", /"led":"panel"/, { timeout: 60_000 });
  for (const [led, pot] of [["bar", "fabric"], ["multi_bar", "ceramic"], ["panel", "nursery"]]) {
    await page.getByLabel("LED style").selectOption(led);
    await page.getByLabel("Pot style").selectOption(pot);
    await expect(page.locator("canvas")).toHaveAttribute("data-grow-options", new RegExp(`"pot":"${pot}"`));
    await page.screenshot({ path: testInfo.outputPath(`${led}-${pot}.png`), fullPage: true });
  }
  await page.getByLabel("Diameter", { exact: true }).selectOption("50");
  await page.getByLabel("Height", { exact: true }).selectOption("40");
  await page.getByLabel("Soil fill").selectOption("0");
  await page.getByLabel("LED brightness").selectOption("0");
  await expect(page.locator("canvas")).toHaveAttribute("data-grow-options", /"diameter":50,"height":40,"fill":0,"output":0/);
  await page.screenshot({ path: testInfo.outputPath("empty-large-pot.png"), fullPage: true });
  await page.getByLabel("Soil fill").selectOption("100");
  await page.getByLabel("LED brightness").selectOption("100");
  await expect(page.locator("canvas")).toHaveAttribute("data-grow-options", /"fill":100,"output":100/);
  await page.screenshot({ path: testInfo.outputPath("filled-large-pot.png"), fullPage: true });
  expect(errors).toEqual([]);
});
