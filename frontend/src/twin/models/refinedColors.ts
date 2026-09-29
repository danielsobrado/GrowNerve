import { BufferGeometry, Float32BufferAttribute } from "three";

/** Reproject live vertex colors through the correspondence saved by Blender. */
export function withSourceColors(refined: BufferGeometry, source: BufferGeometry): BufferGeometry | undefined {
  const sourceVertices = refined.getAttribute("_source_vertex");
  const sourceColors = source.getAttribute("color");
  if (!sourceVertices || !sourceColors) return undefined;
  const colored = new BufferGeometry();
  colored.setIndex(refined.index);
  Object.entries(refined.attributes).forEach(([name, attribute]) => colored.setAttribute(name, attribute));
  const colors = new Float32Array(sourceVertices.count * 3);
  for (let i = 0; i < sourceVertices.count; i++) {
    const index = Math.round(sourceVertices.getX(i));
    colors[i * 3] = sourceColors.getX(index);
    colors[i * 3 + 1] = sourceColors.getY(index);
    colors[i * 3 + 2] = sourceColors.getZ(index);
  }
  colored.setAttribute("color", new Float32BufferAttribute(colors, 3));
  colored.boundingSphere = refined.boundingSphere?.clone() ?? null;
  return colored;
}
