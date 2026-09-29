import type { BufferGeometry } from "three";

/** Stable across browser sessions and Blender round trips; excludes live vertex colors. */
export function geometryKey(geometry: BufferGeometry): string {
  let hash = 2166136261;
  for (const array of [geometry.attributes.position.array, geometry.index?.array ?? []]) {
    for (let i = 0; i < array.length; i++) {
      const value = Math.round(array[i] * 100000);
      hash = Math.imul(hash ^ value, 16777619);
    }
  }
  return `geo_${geometry.attributes.position.count}_${(hash >>> 0).toString(16)}`;
}
