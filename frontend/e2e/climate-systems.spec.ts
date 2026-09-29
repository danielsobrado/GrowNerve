import { expect, test } from "@playwright/test";

test("previews fans, humidifiers and two irrigation systems", async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("./");
  await page.getByRole("button", { name: "Load pilot example" }).click();
  await expect(page.getByRole("heading", { name: "Farm overview" })).toBeVisible();
  if ((page.viewportSize()?.width ?? 1280) <= 780) await page.getByRole("button", { name: "Open navigation" }).click();
  await page.locator(".gn-sidebar").getByRole("button", { name: "3D Twin", exact: true }).click();
  await page.getByRole("button", { name: "Climate & irrigation", exact: true }).click();
  await expect(page.locator("canvas")).toHaveAttribute("data-climate-system", /"variant":"clip"/, { timeout: 60_000 });
  await page.getByRole("button", { name: "Pause animation" }).click();
  await expect(page.locator("canvas")).toHaveAttribute("data-climate-system", /"running":false/);
  for (const [category, variants] of [["fan", ["clip", "inline"]], ["humidifier", ["ultrasonic", "evaporative"]], ["irrigation", ["drip", "ring"]]] as const) {
    await page.getByLabel("Equipment category").selectOption(category);
    for (const variant of variants) {
      await page.getByLabel("Equipment variant").selectOption(variant);
      await expect(page.locator("canvas")).toHaveAttribute("data-climate-system", new RegExp(`"category":"${category}","variant":"${variant}"`));
      await page.screenshot({ path: testInfo.outputPath(`${category}-${variant}.png`), fullPage: true });
    }
  }
  await page.getByLabel("Equipment category").selectOption("humidifier");
  await page.getByRole("button", { name: "Start animation" }).click();
  await page.getByLabel("Animation intensity").selectOption("100");
  await expect(page.locator("canvas")).toHaveAttribute("data-climate-system", /"running":true,"output":100/);
  await page.screenshot({ path: testInfo.outputPath("humidifier-running.png"), fullPage: true });
  await page.getByRole("group", { name: "3D view" }).getByRole("button", { name: "Farm", exact: true }).click();
  await expect(page.getByRole("button", { name: "Look inside reservoir" })).toBeVisible();
  expect(errors).toEqual([]);
});
