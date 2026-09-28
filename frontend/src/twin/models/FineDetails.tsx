import { useEffect, useMemo } from "react";
import { CatmullRomCurve3, DoubleSide, PlaneGeometry, TubeGeometry, Vector3, type BufferGeometry } from "three";
import { mergeGeometries } from "three/addons/utils/BufferGeometryUtils.js";
import type { TwinPerformanceProfile } from "../performance";
import { CC0Material } from "../materials/CC0Material";
import { Batch, Ring, type Vec3 } from "./Parts";

export function Fasteners({ positions, radius, rotation = [0, 0, 0] }: { positions: Vec3[]; radius: number; rotation?: Vec3 }) {
  return <group>{positions.map((position, i) => <group key={i} position={position} rotation={rotation}>
    <Ring radius={radius * 1.25} tube={radius * 0.16} color="#afbcb1" segments={20} />
    <mesh rotation={[Math.PI / 2, 0, 0]} position={[0, 0, radius * 0.2]}>
      <cylinderGeometry args={[radius, radius, radius * 0.45, 6]} />
      <CC0Material surface="metal" color="#ced4ce" metalness={0.82} roughness={0.42} />
    </mesh>
    <Batch color="#24322a" items={[0, Math.PI / 2].map((angle) => ({ position: [0, 0, radius * 0.44], scale: [radius * 1.15, radius * 0.18, radius * 0.035], rotation: [0, 0, angle] }))} />
  </group>)}</group>;
}

export function HoseCollar({ position, radius, rotation }: { position: Vec3; radius: number; rotation?: Vec3 }) {
  return <group position={position} rotation={rotation}>
    {[-0.03, -0.015, 0, 0.015, 0.03].map((y) => <Ring key={y} radius={radius} tube={0.004} position={[0, y, 0]} rotation={[Math.PI / 2, 0, 0]} color="#a2ad94" segments={24} />)}
  </group>;
}

export function TentLining({ quality, position, rotation }: { quality: TwinPerformanceProfile; position: Vec3; rotation?: Vec3 }) {
  const detail = quality.geometryScale;
  const geometry = useMemo(() => {
    const segments = Math.round(84 * detail);
    const result = new PlaneGeometry(0.975, 0.98, segments, segments);
    const vertices = result.getAttribute("position");
    for (let i = 0; i < vertices.count; i++) {
      const x = vertices.getX(i), y = vertices.getY(i);
      const crease = Math.sin(x * 51 + Math.sin(y * 13)) * Math.sin(y * 47 + x * 8);
      vertices.setZ(i, 0.0018 * crease + 0.0007 * Math.sin(x * 119 + y * 73));
    }
    result.computeVertexNormals();
    return result;
  }, [detail]);
  useEffect(() => () => geometry.dispose(), [geometry]);
  return <mesh geometry={geometry} position={position} rotation={rotation} receiveShadow>
    <CC0Material surface="metal" repeat={[3, 3]} color="#c7cec5" roughness={0.5} metalness={0.65} grain={0.35} side={DoubleSide} />
  </mesh>;
}

export function RootBundle({ quality, position, seed }: { quality: TwinPerformanceProfile; position: Vec3; seed: number }) {
  const count = Math.round(12 * quality.geometryScale);
  const geometry = useMemo(() => {
    const pieces: BufferGeometry[] = [];
    for (let i = 0; i < count; i++) {
      const a = i * 2.399963 + seed;
      const curve = new CatmullRomCurve3([
        new Vector3(Math.sin(a) * 0.015, 0, Math.cos(a) * 0.015),
        new Vector3(Math.sin(a + 0.4) * 0.028, -0.2, Math.cos(a + 0.4) * 0.028),
        new Vector3(Math.sin(a + 0.8) * 0.044, -0.4, Math.cos(a + 0.8) * 0.044),
        new Vector3(Math.sin(a + 1.4) * 0.055, -0.53 - (i % 4) * 0.03, Math.cos(a + 1.4) * 0.055),
      ]);
      pieces.push(new TubeGeometry(curve, 22, 0.0018, 5, false));
      for (let branch = 1; branch <= 4; branch++) {
        const start = curve.getPoint(branch / 6);
        const end = start.clone().add(new Vector3(Math.sin(a + branch) * 0.027, -0.11, Math.cos(a + branch) * 0.027));
        pieces.push(new TubeGeometry(new CatmullRomCurve3([start, start.clone().lerp(end, 0.5).add(new Vector3(0.007, 0, 0)), end]), 7, 0.00065, 4, false));
      }
    }
    const merged = mergeGeometries(pieces);
    pieces.forEach((piece) => piece.dispose());
    return merged;
  }, [count, seed]);
  useEffect(() => () => geometry.dispose(), [geometry]);
  return <mesh geometry={geometry} position={position}><meshStandardMaterial color="#e6d8ac" roughness={0.82} /></mesh>;
}
