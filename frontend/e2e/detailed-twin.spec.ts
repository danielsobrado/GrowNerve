import { expect, test } from "@playwright/test";

test("renders detailed equipment and opens the reservoir cutaway", async ({ page }, testInfo) => {
  test.setTimeout(90_000);
  const errors: string[] = [];
  const textures = new Set<string>();
  page.on("response", (response) => {
    if (response.url().includes("/textures/cc0/") && response.url().endsWith(".jpg") && response.ok()) textures.add(response.url());
  });
  page.on("pageerror", (error) => { errors.push(error.message); console.error("Twin browser error:", error.message); });
  page.on("console", (message) => {
    if (message.type() === "error" && /PTL material preparation failed|CC0 texture loading failed/.test(message.text())) errors.push(message.text());
  });
  await page.goto("./");
  await page.getByRole("button", { name: "Load pilot example" }).click();
  await expect(page.getByRole("heading", { name: "Farm overview" })).toBeVisible();
  if ((page.viewportSize()?.width ?? 1280) <= 780) await page.getByRole("button", { name: "Open navigation" }).click();
  await page.locator(".gn-sidebar").getByRole("button", { name: "3D Twin", exact: true }).click();
  await expect(page.locator(".gn-renderer-badge")).toContainText(/WebGPU|WebGL/, { timeout: 60_000 });
  await expect(page.locator("canvas")).toBeVisible();
  await expect.poll(() => textures.size, { timeout: 30_000 }).toBe(15);
  await page.locator("canvas").click();
  await expect(page.locator(".gn-context-target strong")).toHaveText(/.+/);
  await expect(page.getByRole("toolbar", { name: / actions$/ })).toBeVisible();
  await page.mouse.move(0, 0);
  await page.screenshot({ path: testInfo.outputPath("detailed-twin.png"), fullPage: true });
  await page.getByRole("button", { name: "Look inside reservoir" }).click();
  await expect(page.getByRole("button", { name: "Close reservoir" })).toHaveAttribute("aria-pressed", "true");
  await page.screenshot({ path: testInfo.outputPath("reservoir-cutaway.png"), fullPage: true });
  await page.getByRole("button", { name: "Close reservoir" }).click();
  await expect(page.getByRole("button", { name: "Look inside reservoir" })).toHaveAttribute("aria-pressed", "false");
  if (testInfo.project.name === "chromium") {
    await page.locator("canvas").hover();
    await page.mouse.wheel(0, -420);
    // OrbitControls eases the dolly over several rendered frames.
    await page.evaluate(() => new Promise<void>((resolve) => {
      let frames = 0;
      const next = () => { if (++frames === 30) resolve(); else requestAnimationFrame(next); };
      requestAnimationFrame(next);
    }));
    await page.mouse.move(0, 0);
    await page.screenshot({ path: testInfo.outputPath("texture-detail.png"), fullPage: true });
  }
  expect(errors).toEqual([]);
});
