import { useGLTF } from "@react-three/drei";
import { useFrame, useThree } from "@react-three/fiber";
import { useEffect, useMemo, useRef } from "react";
import { CanvasTexture, Group, Mesh, Object3D, Sprite, SpriteMaterial } from "three";

export interface ClimateConfig { category: "fan" | "humidifier" | "irrigation"; variant: string; running: boolean; output: number }

export function ClimateSystems({ category, variant, running, output }: ClimateConfig) {
  const { scene } = useGLTF(`${import.meta.env.BASE_URL}models/blender/climate-systems.glb`);
  const canvas = useThree((state) => state.gl.domElement);
  const rotor = useRef<Group>(null);
  const mist = useRef<Group>(null);
  const mistTexture = useMemo(() => {
    const image = document.createElement("canvas"); image.width = 64; image.height = 64;
    const context = image.getContext("2d");
    if (context) {
      const gradient = context.createRadialGradient(32, 32, 0, 32, 32, 32);
      gradient.addColorStop(0, "rgba(225,245,250,0.8)");
      gradient.addColorStop(0.35, "rgba(225,245,250,0.35)");
      gradient.addColorStop(1, "rgba(225,245,250,0)");
      context.fillStyle = gradient; context.fillRect(0, 0, 64, 64);
    }
    return new CanvasTexture(image);
  }, []);
  useEffect(() => () => mistTexture.dispose(), [mistTexture]);
  const models = useMemo(() => {
    const sources = new Map<string, Object3D>();
    scene.traverse((object) => { if (object.userData.equipmentModule) sources.set(object.userData.equipmentModule as string, object); });
    const copy = (role: string) => {
      const source = sources.get(role);
      if (!source) throw new Error(`Missing climate module ${role}`);
      const object = source.clone(true);
      object.traverse((child) => { if (child instanceof Mesh) { child.castShadow = true; child.receiveShadow = true; } });
      return object;
    };
    return { body: copy(`${category}_${variant}`), rotor: category === "fan" ? copy(`rotor_${variant}`) : undefined };
  }, [scene, category, variant]);
  useFrame(({ clock }, delta) => {
    if (rotor.current && running) rotor.current.rotation.z += Math.min(delta, 0.05) * output / 100 * 15;
    if (mist.current) mist.current.children.forEach((particle, i) => {
      const phase = (clock.elapsedTime * 0.45 + i / 12) % 1;
      particle.position.set(0.035 + Math.sin(i * 2.4 + phase) * phase * 0.035, 0.372 + phase * 0.18, Math.cos(i * 2.4) * phase * 0.025);
      particle.scale.setScalar(0.018 + phase * 0.075);
      if (particle instanceof Sprite) (particle.material as SpriteMaterial).opacity = (1 - phase) * output / 100 * 0.35;
    });
  });
  useEffect(() => {
    canvas.dataset.climateSystem = JSON.stringify({ category, variant, running, output });
    return () => { delete canvas.dataset.climateSystem; };
  }, [canvas, category, variant, running, output]);
  return <group>
    <primitive object={models.body} />
    {models.rotor && <group ref={rotor} position={[0, 0.32, 0.015]} dispose={null}><primitive object={models.rotor} /></group>}
    {category === "humidifier" && variant === "ultrasonic" && running && output > 0 && <group ref={mist}>{Array.from({ length: 12 }, (_, i) => <sprite key={i}><spriteMaterial map={mistTexture} transparent depthWrite={false} /></sprite>)}</group>}
  </group>;
}
