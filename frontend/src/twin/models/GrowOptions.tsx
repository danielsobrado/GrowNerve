import { useGLTF } from "@react-three/drei";
import { useThree } from "@react-three/fiber";
import { useEffect, useMemo } from "react";
import { Mesh, MeshStandardMaterial, Object3D } from "three";

export interface GrowOptionsConfig { led: string; pot: string; diameter: number; height: number; fill: number; output: number }

export function GrowOptions({ led, pot, diameter, height, fill, output }: GrowOptionsConfig) {
  const { scene } = useGLTF(`${import.meta.env.BASE_URL}models/blender/grow-options.glb`);
  const canvas = useThree((state) => state.gl.domElement);
  const models = useMemo(() => {
    const sources = new Map<string, Object3D>();
    scene.traverse((object) => { if (object.userData.equipmentModule) sources.set(object.userData.equipmentModule as string, object); });
    const owned: MeshStandardMaterial[] = [];
    const copy = (role: string) => {
      const source = sources.get(role);
      if (!source) throw new Error(`Missing equipment module ${role}`);
      const object = source.clone(true);
      object.traverse((child) => {
        if (!(child instanceof Mesh)) return;
        child.castShadow = true; child.receiveShadow = true;
        const clone = (material: MeshStandardMaterial) => {
          if (!material.name.startsWith("LED ")) return material;
          const result = material.clone(); result.emissive.copy(result.color); owned.push(result); return result;
        };
        child.material = Array.isArray(child.material) ? child.material.map(clone) : clone(child.material);
      });
      return object;
    };
    return { light: copy(`led_${led}`), pot: copy(`pot_${pot}`), soil: copy("soil"), owned };
  }, [scene, led, pot]);
  useEffect(() => () => models.owned.forEach((material) => material.dispose()), [models]);
  useEffect(() => { models.owned.forEach((material) => { material.emissiveIntensity = output / 100 * 2; }); }, [models, output]);
  useEffect(() => {
    canvas.dataset.growOptions = JSON.stringify({ led, pot, diameter, height, fill, output });
    return () => { delete canvas.dataset.growOptions; };
  }, [canvas, led, pot, diameter, height, fill, output]);
  const fillFraction = fill / 100;
  const soilRadiusScale = (0.1 + 0.034 * fillFraction) / 0.13;
  return <group dispose={null}>
    <group scale={[diameter / 30, height / 30, diameter / 30]}>
      <primitive object={models.pot} />
      {fill > 0 && <group position={[0, 0.032 + 0.24 * fillFraction, 0]} scale={[soilRadiusScale, 1, soilRadiusScale]}><primitive object={models.soil} /></group>}
    </group>
    <group position={[0, height / 100 + 0.5, 0]}><primitive object={models.light} /></group>
    <pointLight position={[0, height / 100 + 0.45, 0]} intensity={output / 100 * 0.4} color="#fff2da" />
  </group>;
}
