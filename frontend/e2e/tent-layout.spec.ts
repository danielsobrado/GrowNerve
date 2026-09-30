import { expect, test } from '@playwright/test';
import { PerspectiveCamera, Vector3 } from 'three';

test('resizes a tent and objects, snaps placement, undoes and restores saved layouts', async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto('./');
  await page.getByRole('button', { name: 'Load pilot example' }).click();
  await expect(page.getByRole('heading', { name: 'Farm overview' })).toBeVisible();
  const openTwin = async () => {
    if ((page.viewportSize()?.width ?? 1280) <= 780) await page.getByRole('button', { name: 'Open navigation' }).click();
    await page.locator('.gn-sidebar').getByRole('button', { name: '3D Twin', exact: true }).click();
    await page.getByRole('button', { name: 'Tent layout', exact: true }).click();
    await expect(page.locator('canvas')).toHaveAttribute('data-tent-layout', /"version":2/, { timeout: 60_000 });
  };
  await openTwin();
  await page.getByRole('button', { name: 'Add to tent' }).click();
  const edit = async (label: string, value: string) => { await page.getByLabel(label, { exact: true }).fill(value); await page.getByLabel(label, { exact: true }).press('Enter'); };
  await edit('Object width', '45');
  await edit('Object height', '40');
  await edit('X position', '37');
  await edit('Z position', '12');
  await expect(page.getByLabel('X position', { exact: true })).toHaveValue('40');
  await expect(page.getByLabel('Z position', { exact: true })).toHaveValue('10');
  // Exercise the actual translation handle, not just its numeric alternative.
  if (testInfo.project.name === 'chromium') {
    const canvas = page.locator('canvas');
    await canvas.scrollIntoViewIfNeeded();
    const box = await canvas.boundingBox();
    if (!box) throw new Error('Missing canvas');
    const camera = new PerspectiveCamera(42, box.width / box.height, .1, 1000);
    camera.position.set(4, 3.5, 5); camera.lookAt(0, 1.3, 0); camera.updateMatrixWorld();
    const screen = (x: number) => { const point = new Vector3(x, 0, .1).project(camera); return { x: box.x + (point.x + 1) * box.width / 2, y: box.y + (1 - point.y) * box.height / 2 }; };
    const start = screen(.7), end = screen(1.1);
    await page.mouse.move(start.x, start.y); await page.mouse.down(); await page.mouse.move(end.x, end.y, { steps: 12 }); await page.mouse.up();
    await expect(page.getByLabel('X position', { exact: true })).not.toHaveValue('40');
    await edit('X position', '40');
  }
  await page.getByRole('button', { name: 'Rotate 90°' }).click();
  await expect(page.locator('canvas')).toHaveAttribute('data-tent-layout', /"rotation":90/);
  await page.getByRole('button', { name: 'Undo', exact: true }).click();
  await expect(page.locator('canvas')).toHaveAttribute('data-tent-layout', /"rotation":0/);
  await page.getByRole('button', { name: 'Redo', exact: true }).click();
  await page.getByText('Grow tent ·', { exact: false }).click();
  await edit('Tent width', '300');
  await edit('Tent depth', '200');
  await expect(page.locator('canvas')).toHaveAttribute('data-tent-layout', /"tent":\[3,2.6,2\]/);
  await page.getByLabel('Snap grid').selectOption('0.25');
  await expect(page.getByLabel('X position', { exact: true })).toHaveValue('50');
  await page.getByLabel('Add equipment type').selectOption('fan_clip');
  await page.getByRole('button', { name: 'Add to tent' }).click();
  await page.getByLabel('Add equipment type').selectOption('sensor_ph');
  await page.getByRole('button', { name: 'Add to tent' }).click();
  await page.getByRole('button', { name: 'Place on grid' }).click();
  const canvas = page.locator('canvas');
  const rect = await canvas.boundingBox();
  if (!rect) throw new Error('Missing canvas');
  const beforePlacement = await canvas.getAttribute('data-tent-layout');
  await canvas.click({ position: { x: rect.width * .52, y: rect.height * .70 } });
  await expect(canvas).not.toHaveAttribute('data-tent-layout', beforePlacement!);
  await page.screenshot({ path: testInfo.outputPath('tent-layout.png'), fullPage: true });
  const saved = await canvas.getAttribute('data-tent-layout');
  await page.reload();
  await openTwin();
  await expect(page.locator('canvas')).toHaveAttribute('data-tent-layout', saved!);
  expect(errors).toEqual([]);
});
