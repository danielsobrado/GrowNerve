import { RoundedBox } from "@react-three/drei";
import { useEffect, useLayoutEffect, useMemo, useRef, type ReactNode } from "react";
import { CatmullRomCurve3, Object3D, TubeGeometry, Vector3, type InstancedMesh } from "three";
import { CC0Material, type Surface } from "../materials/CC0Material";

export type Vec3 = [number, number, number];
export interface Instance { position: Vec3; scale: Vec3; rotation?: Vec3 }

/** Repeated hardware uses a single draw call per material. */
export function Batch({ items, color, metalness = 0, roughness = 0.6, emissive = "#000000", intensity = 0, sphere = false, shadows = false, surface }: {
  items: Instance[]; color: string; metalness?: number; roughness?: number; emissive?: string; intensity?: number; sphere?: boolean; shadows?: boolean; surface?: Surface;
}) {
  const ref = useRef<InstancedMesh>(null);
  useLayoutEffect(() => {
    if (!ref.current) return;
    const object = new Object3D();
    items.forEach((item, index) => {
      object.position.set(...item.position);
      object.scale.set(...item.scale);
      object.rotation.set(...(item.rotation ?? [0, 0, 0]));
      object.updateMatrix();
      ref.current!.setMatrixAt(index, object.matrix);
    });
    ref.current.instanceMatrix.needsUpdate = true;
    ref.current.computeBoundingSphere();
  }, [items]);
  return <instancedMesh ref={ref} args={[undefined, undefined, items.length]} castShadow={shadows} receiveShadow>
    {sphere ? <sphereGeometry args={[1, 16, 12]} /> : <boxGeometry />}
    {surface ? <CC0Material surface={surface} color={color} metalness={metalness} roughness={roughness} grain={0.2} />
      : <meshStandardMaterial color={color} metalness={metalness} roughness={roughness} emissive={emissive} emissiveIntensity={intensity} />}
  </instancedMesh>;
}

export function Casing({ size, position, color = "#26352f", radius = 0.035, metalness = 0.1, children, surface = "plastic" }: {
  size: Vec3; position?: Vec3; color?: string; radius?: number; metalness?: number; children?: ReactNode; surface?: Surface;
}) {
  return <RoundedBox args={size} radius={radius} smoothness={5} position={position} castShadow receiveShadow>
    <CC0Material surface={surface} color={color} roughness={0.62} metalness={metalness} grain={0.22} />{children}
  </RoundedBox>;
}

export function Tube({ points, radius = 0.012, color = "#28362f", metalness = 0, segments = 28 }: {
  points: Vec3[]; radius?: number; color?: string; metalness?: number; segments?: number;
}) {
  const key = JSON.stringify(points);
  const geometry = useMemo(() => new TubeGeometry(
    new CatmullRomCurve3((JSON.parse(key) as Vec3[]).map((point) => new Vector3(...point))), segments, radius, 6, false,
  ), [key, radius, segments]);
  useEffect(() => () => geometry.dispose(), [geometry]);
  return <mesh geometry={geometry} castShadow><meshStandardMaterial color={color} metalness={metalness} roughness={0.45} /></mesh>;
}

export function Ring({ radius, tube = 0.012, position = [0, 0, 0], rotation = [0, 0, 0], color = "#84958b", metalness = 0.7, segments = 40 }: {
  radius: number; tube?: number; position?: Vec3; rotation?: Vec3; color?: string; metalness?: number; segments?: number;
}) {
  return <mesh position={position} rotation={rotation} castShadow>
    <torusGeometry args={[radius, tube, 10, segments]} />
    <meshStandardMaterial color={color} metalness={metalness} roughness={0.38} />
  </mesh>;
}
