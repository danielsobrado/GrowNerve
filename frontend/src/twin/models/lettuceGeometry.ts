import { BufferGeometry, Color, Float32BufferAttribute } from "three";

/** Deterministic, cupped lettuce leaves. Coordinates match the existing plant profile. */
export function createLettuceGeometry(detail: number, attention: boolean, seed = 0): BufferGeometry {
  const rows = Math.max(8, Math.round(24 * detail));
  const columns = Math.max(8, Math.round(20 * detail));
  const leaves = Math.max(12, Math.round(24 * detail));
  const positions: number[] = [], colors: number[] = [], indices: number[] = [];
  const dark = new Color(), light = new Color(), color = new Color();

  for (let leaf = 0; leaf < leaves; leaf++) {
    const progress = leaf / (leaves - 1);
    const angle = leaf * 2.399963 + seed * 0.71;
    const length = 0.76 - progress * 0.42;
    const width = 0.34 - progress * 0.13;
    const rise = 0.3 + progress * 0.3;
    const offset = positions.length / 3;
    const aged = attention && leaf < Math.ceil(leaves * 0.23);
    dark.set(aged ? "#71812c" : leaf % 3 === 0 ? "#256b29" : "#398330");
    light.set(aged ? "#b2a653" : progress > 0.65 ? "#9ac45e" : "#74ad49");

    for (let row = 0; row <= rows; row++) {
      const t = row / rows;
      for (let column = 0; column <= columns; column++) {
        const u = column / columns * 2 - 1;
        const edge = Math.abs(u);
        const outline = Math.pow(Math.sin(Math.PI * t), 0.5);
        const ripple = Math.sin(t * 31 + leaf * 1.7 + u * 2) * 0.016 * edge ** 3 * Math.sin(Math.PI * t);
        const x = u * width * outline * (1 + 0.07 * Math.sin(t * 43 + leaf));
        const z = t * length * (1 - progress * 0.18) - edge * edge * 0.065 * outline;
        const y = -0.1 + Math.sin(t * Math.PI * 0.7) * rise
          + edge * edge * outline * (0.1 + progress * 0.08) + ripple - t ** 4 * (1 - progress) * 0.08;
        positions.push(x * Math.cos(angle) + z * Math.sin(angle), y, z * Math.cos(angle) - x * Math.sin(angle));
        const midrib = Math.exp(-u * u * 180) * (1 - t * 0.7);
        const vein = Math.pow(Math.max(0, Math.cos((t - edge * 0.19) * Math.PI * 16)), 18) * 0.13;
        color.copy(dark).lerp(light, Math.min(1, 0.24 + progress * 0.25 + midrib * 0.55 + vein + edge * 0.18));
        colors.push(color.r, color.g, color.b);
        if (row < rows && column < columns) {
          const a = offset + row * (columns + 1) + column;
          indices.push(a, a + columns + 1, a + 1, a + 1, a + columns + 1, a + columns + 2);
        }
      }
    }
  }
  const geometry = new BufferGeometry();
  geometry.setAttribute("position", new Float32BufferAttribute(positions, 3));
  geometry.setAttribute("color", new Float32BufferAttribute(colors, 3));
  geometry.setIndex(indices);
  geometry.computeVertexNormals();
  geometry.computeBoundingSphere();
  return geometry;
}
