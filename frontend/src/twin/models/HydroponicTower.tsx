import { useGLTF } from "@react-three/drei";
import { useThree } from "@react-three/fiber";
import { useEffect, useMemo } from "react";
import { Mesh, Object3D } from "three";

function towerLevelCount(levels: number): number {
  return Number.isFinite(levels) ? Math.max(3, Math.min(8, Math.round(levels))) : 6;
}

/** Reuses three Blender-authored modules; each tier supplies three angled planting sites. */
export function HydroponicTower({ levels = 6 }: { levels?: number }) {
  const count = towerLevelCount(levels);
  const { scene } = useGLTF(`${import.meta.env.BASE_URL}models/blender/hydroponic-tower.glb`);
  const canvas = useThree((state) => state.gl.domElement);
  const modules = useMemo(() => {
    const source = new Map<string, Object3D>();
    scene.traverse((object) => { if (object.userData.towerModule) source.set(object.userData.towerModule as string, object); });
    const copy = (role: string) => {
      const original = source.get(role);
      if (!original) throw new Error(`Tower asset is missing its ${role} module`);
      const object = original.clone(true);
      object.traverse((child) => { if (child instanceof Mesh) { child.castShadow = true; child.receiveShadow = true; } });
      return object;
    };
    return { base: copy("base"), crown: copy("crown"), tiers: Array.from({ length: count }, () => copy("tier")) };
  }, [scene, count]);
  useEffect(() => {
    canvas.dataset.towerLevels = String(count);
    canvas.dataset.towerSites = String(count * 3);
    return () => { delete canvas.dataset.towerLevels; delete canvas.dataset.towerSites; };
  }, [canvas, count]);
  return <group name="Hydroponic tower" dispose={null}>
    <primitive object={modules.base} />
    {modules.tiers.map((tier, index) => <group key={index} position={[0, 0.65 + index * 0.3, 0]} rotation={[0, index % 2 * Math.PI / 3, 0]}><primitive object={tier} /></group>)}
    <group position={[0, 0.5 + count * 0.3, 0]}><primitive object={modules.crown} /></group>
  </group>;
}
