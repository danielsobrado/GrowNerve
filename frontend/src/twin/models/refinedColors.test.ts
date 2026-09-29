import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { expect, test } from "vitest";
import { Mesh } from "three";
import { GLTFLoader } from "three/addons/loaders/GLTFLoader.js";
import { createLettuceGeometry } from "./lettuceGeometry";
import { withSourceColors } from "./refinedColors";

test("the shipped Blender lettuce preserves localized health colors after vertex reordering", async () => {
  const bytes = readFileSync(resolve("public/models/blender/lettuce.glb"));
  const data = new ArrayBuffer(bytes.length);
  new Uint8Array(data).set(bytes);
  const { scene } = await new GLTFLoader().parseAsync(data, "");
  const mesh = scene.children.find((object) => object instanceof Mesh && object.geometry.hasAttribute("_source_vertex")) as Mesh;
  expect(mesh).toBeDefined();
  const healthySource = createLettuceGeometry(1, false, 0);
  const attentionSource = createLettuceGeometry(1, true, 0);
  const healthy = withSourceColors(mesh.geometry, healthySource)!;
  const attention = withSourceColors(mesh.geometry, attentionSource)!;
  const healthColors = healthy.getAttribute("color"), attentionColors = attention.getAttribute("color");
  let changed = 0;
  for (let i = 0; i < healthColors.count; i++) {
    expect(Number.isFinite(attentionColors.getX(i))).toBe(true);
    if (healthColors.getX(i) !== attentionColors.getX(i)) changed++;
  }
  expect(changed / healthColors.count).toBeGreaterThan(0.15);
  expect(changed / healthColors.count).toBeLessThan(0.3);
  expect(healthy.getAttribute("position")).toBe(attention.getAttribute("position"));
  expect(healthy.getAttribute("color")).not.toBe(attention.getAttribute("color"));
  healthy.dispose(); attention.dispose(); healthySource.dispose(); attentionSource.dispose();
});
