import { useEffect, useRef, useState, type ReactNode } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import { BufferGeometry, Group, Mesh } from "three";
import { GLTFLoader } from "three/addons/loaders/GLTFLoader.js";
import { geometryKey } from "./geometryKey";
import { withSourceColors } from "./refinedColors";

export type RefinedModel = "lettuce" | "tent" | "reservoir" | "light" | "fan" | "controller" | "pump";
const cache = new Map<RefinedModel, Promise<Map<string, BufferGeometry>>>();
const keys = new WeakMap<BufferGeometry, string>();

function load(model: RefinedModel) {
  let promise = cache.get(model);
  if (!promise) {
    promise = new GLTFLoader().loadAsync(`${import.meta.env.BASE_URL}models/blender/${model}.glb`).then(({ scene }) => {
      const geometries = new Map<string, BufferGeometry>();
      scene.traverse((object) => {
        if (object instanceof Mesh) {
          const key = object.userData.geometryKey as string;
          geometries.set(key, object.geometry);
          keys.set(object.geometry, key);
          object.geometry.computeBoundingSphere();
        }
      });
      return geometries;
    }).catch((error: unknown) => { cache.delete(model); throw error; });
    cache.set(model, promise);
  }
  return promise;
}

/** Exchange local geometry only: domain state, transforms, instances and materials stay live. */
export function RefinedGeometry({ model, enabled, children }: { model: RefinedModel; enabled: boolean; children: ReactNode }) {
  const group = useRef<Group>(null);
  const canvas = useThree((state) => state.gl.domElement);
  const [loaded, setLoaded] = useState<{ model: RefinedModel; geometries: Map<string, BufferGeometry> }>();
  const replacements = useRef(new Map<Mesh, { original: BufferGeometry; refined: BufferGeometry; owned: boolean }>());
  useEffect(() => {
    if (!enabled) return;
    let active = true;
    void load(model).then((geometries) => { if (active) setLoaded({ model, geometries }); })
      .catch((error: unknown) => console.error(`Blender geometry loading failed (${model})`, error));
    return () => { active = false; };
  }, [model, enabled]);
  useEffect(() => {
    const owned = replacements.current;
    return () => {
      owned.forEach(({ original, refined, owned: dispose }, mesh) => {
        if (mesh.geometry === refined) mesh.geometry = original;
        if (dispose) refined.dispose();
      });
      owned.clear();
    };
  }, [model, enabled]);
  useFrame(() => {
    if (!enabled || loaded?.model !== model || !group.current) return;
    let count = 0;
    group.current.traverse((object) => {
      if (!(object instanceof Mesh)) return;
      const original = object.geometry;
      const previous = replacements.current.get(object);
      if (previous?.refined === original) { count++; return; }
      let key = keys.get(original);
      if (!key) { key = geometryKey(original); keys.set(original, key); }
      let refined = loaded.geometries.get(key);
      if (!refined) return;
      count++;
      if (original === refined) return;
      if (previous?.owned) previous.refined.dispose();
      // Blender may split/reorder vertices. Restore live plant colors by source vertex ID.
      const colored = withSourceColors(refined, original);
      const ownsGeometry = Boolean(colored);
      if (colored) refined = colored;
      replacements.current.set(object, { original, refined, owned: ownsGeometry });
      object.geometry = refined;
    });
    group.current.userData.refinedMeshes = count;
    canvas.dataset[`blender${model[0].toUpperCase()}${model.slice(1)}`] = String(count);
  });
  return <group ref={group} name={`blender-${model}`}>{children}</group>;
}
