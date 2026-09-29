import { useFrame } from "@react-three/fiber";
import { useEffect, useMemo, useRef } from "react";
import { DoubleSide, ExtrudeGeometry, Shape, type Group } from "three";
import { RoundedBox } from "@react-three/drei";
import type { TwinPerformanceProfile } from "../performance";
import { CC0Material } from "../materials/CC0Material";
import { Fasteners, RootBundle, TentLining, HoseCollar } from "./FineDetails";
import { createLettuceGeometry } from "./lettuceGeometry";
import { Batch, Casing, Ring, Tube, type Instance } from "./Parts";

type QualityProps = { quality: TwinPerformanceProfile };
const range = (count: number) => Array.from({ length: count }, (_, index) => index);

export function LettucePlant({ quality, attention, occupied, seed }: QualityProps & { attention: boolean; occupied: boolean; seed: number }) {
  const geometry = useMemo(() => createLettuceGeometry(quality.geometryScale, attention, 0), [quality.geometryScale, attention]);
  useEffect(() => () => geometry.dispose(), [geometry]);
  const slots = useMemo<Instance[]>(() => range(20).map((i) => {
    const angle = i * Math.PI / 10;
    return { position: [Math.sin(angle) * 0.19, -0.23, Math.cos(angle) * 0.19], scale: [0.021, 0.23, 0.022], rotation: [0, angle, 0] };
  }), []);
  const clay = useMemo<Instance[]>(() => range(quality.name === "low-power" ? 12 : 28).map((i) => {
    const angle = i * 2.399963;
    const radius = Math.sqrt((i + 0.5) / 28) * 0.165;
    return { position: [Math.sin(angle) * radius, -0.112 + (i % 3) * 0.008, Math.cos(angle) * radius], scale: [0.038, 0.027, 0.035] };
  }), [quality.name]);
  return <group>
    <Batch items={slots} color="#17231c" surface="plastic" shadows={quality.shadows} />
    {[-0.12, -0.25, -0.34].map((y) => <Ring key={y} radius={y === -0.34 ? 0.17 : 0.2} tube={0.018} position={[0, y, 0]} rotation={[Math.PI / 2, 0, 0]} color="#1c2c22" metalness={0} />)}
    <mesh position={[0, -0.14, 0]}><cylinderGeometry args={[0.19, 0.17, 0.045, 24]} /><meshStandardMaterial color="#443328" roughness={1} /></mesh>
    <Batch items={clay} color="#a47851" surface="concrete" sphere />
    {occupied && <mesh geometry={geometry} rotation={[0, seed * 0.71, 0]} castShadow={quality.shadows}>
      <CC0Material surface="leaf" color={attention ? "#e3cd92" : "#ffffff"} vertexColors roughness={0.78} grain={0.16} sheen={0.22} sheenColor="#b5d89b" clearcoat={0.05} />
    </mesh>}
  </group>;
}

export function GrowTent({ quality }: QualityProps) {
  const posts = useMemo<Instance[]>(() => [-0.495, 0.495].flatMap((x) => [-0.495, 0.495].map((z) => ({ position: [x, 0, z], scale: [0.012, 1.03, 0.012] }))), []);
  const rails = useMemo<Instance[]>(() => [-0.49, 0.5].flatMap((y) => [
    { position: [0, y, -0.495], scale: [1, 0.014, 0.014] }, { position: [0, y, 0.495], scale: [1, 0.014, 0.014] },
    { position: [-0.495, y, 0], scale: [0.014, 0.014, 1] }, { position: [0.495, y, 0], scale: [0.014, 0.014, 1] },
  ] as Instance[]), []);
  const seams = useMemo<Instance[]>(() => range(7).map((i) => ({ position: [-0.45 + i * 0.15, 0, -0.482], scale: [0.002, 0.95, 0.003] })), []);
  const diamonds = useMemo<Instance[]>(() => range(quality.name === "low-power" ? 0 : 156).map((i) => ({
    position: [-0.455 + (i % 12) * 0.082, -0.455 + Math.floor(i / 12) * 0.074, -0.48], scale: [0.043, 0.002, 0.001], rotation: [0, 0, Math.PI / 4],
  })), [quality.name]);
  return <group>
    <mesh position={[0, 0, -0.499]} receiveShadow><boxGeometry args={[1, 1, 0.008]} /><CC0Material surface="fabric" albedo={false} color="#18221c" repeat={[5, 5]} grain={0.65} roughness={0.95} /></mesh>
    <TentLining quality={quality} position={[0, 0, -0.487]} />
    <TentLining quality={quality} position={[-0.494, 0, 0]} rotation={[0, Math.PI / 2, 0]} />
    <Batch items={posts} surface="metal" color="#d3d9d2" metalness={0.8} shadows={quality.shadows} />
    <Batch items={rails} surface="metal" color="#c0cbc2" metalness={0.8} shadows={quality.shadows} />
    <Batch items={seams} color="#485c50" />
    {diamonds.length > 0 && <Batch items={diamonds} color="#a4b0a5" metalness={0.55} />}
    <mesh position={[0, -0.485, 0]} receiveShadow><boxGeometry args={[0.98, 0.025, 0.98]} /><CC0Material surface="fabric" albedo={false} repeat={[4, 4]} color="#283b30" roughness={0.8} grain={0.4} /></mesh>
    {[-0.49, 0.49].flatMap((x) => [-0.49, 0.49].map((z) => <mesh key={`${x}:${z}`} position={[x, 0.5, z]}><boxGeometry args={[0.036, 0.028, 0.036]} /><meshStandardMaterial color="#202f25" roughness={0.56} /></mesh>))}
    {[-0.25, 0.25].map((x) => <mesh key={x} position={[x, 0.505, 0]}><boxGeometry args={[0.009, 0.01, 0.98]} /><meshStandardMaterial color="#a4b0a7" metalness={0.8} roughness={0.3} /></mesh>)}
    <group position={[-0.26, 0.29, -0.477]}>
      <Ring radius={0.088} tube={0.012} color="#26382d" />
      <mesh rotation={[Math.PI / 2, 0, 0]}><cylinderGeometry args={[0.08, 0.08, 0.018, 32]} /><meshStandardMaterial color="#19271e" roughness={0.85} /></mesh>
      <Batch color="#526b59" items={range(7).map((i) => ({ position: [0, (i - 3) * 0.017, 0.012], scale: [Math.sqrt(0.075 ** 2 - ((i - 3) * 0.017) ** 2) * 2, 0.005, 0.006] }))} />
    </group>
    {/* The front and right panels are cut away for access to domain objects. */}
    <mesh position={[0.478, 0, -0.46]}><boxGeometry args={[0.014, 0.96, 0.055]} /><meshStandardMaterial color="#24362a" /></mesh>
    <Batch color="#b5bd9c" items={range(48).map((i) => ({ position: [0.488, -0.46 + i * 0.0195, -0.424], scale: [0.003, 0.008, 0.003] }))} />
  </group>;
}

export function DwcReservoir({ quality, level, cutaway = false, rootPositions = [] }: QualityProps & { level: number; cutaway?: boolean; rootPositions?: [number, number][] }) {
  const fill = Math.max(0, Math.min(1, level / 100));
  const ribs = useMemo<Instance[]>(() => [-1, 1].flatMap((side) => range(7).map((i) => ({ position: [side * 0.486, -0.04, -0.39 + i * 0.13], scale: [0.023, 0.7, 0.018] }))), []);
  return <group>
    <RoundedBox args={[0.96, 0.87, 0.94]} radius={0.035} smoothness={5} visible={!cutaway} position={[0, -0.045, 0]} castShadow={quality.shadows} receiveShadow><CC0Material surface="plastic" color="#3c5146" repeat={[2, 2]} grain={0.38} roughness={0.8} /></RoundedBox>
    <group visible={!cutaway}><Casing size={[1.04, 0.085, 1.04]} position={[0, 0.435, 0]} color="#809187" radius={0.02} /></group>
    {[-1, 1].map((side) => <group key={`gasket:${side}`}>
      <Casing size={[1.065, 0.028, 0.018]} position={[0, 0.382, side * 0.523]} color="#1d3025" radius={0.006} />
      <Casing size={[0.018, 0.028, 1.03]} position={[side * 0.523, 0.382, 0]} color="#1d3025" radius={0.006} />
    </group>)}
    <Batch items={ribs} color="#4b6153" />
    <Fasteners positions={[-0.46, 0.46].flatMap((x) => [-0.4, 0, 0.4].map((z): [number, number, number] => [x, 0.48, z]))} radius={0.01} rotation={[-Math.PI / 2, 0, 0]} />
    <Casing size={[0.97, 0.04, 0.98]} position={[0, -0.465, 0]} color="#243b2c" radius={0.015} />
    {[-1, 1].map((side) => <group key={side} position={[side * 0.505, 0.19, 0]}>
      <Casing size={[0.035, 0.09, 0.28]} radius={0.012} color="#17291e" />
      <mesh position={[side * 0.012, 0.018, 0]}><boxGeometry args={[0.015, 0.035, 0.2]} /><meshStandardMaterial color="#809285" /></mesh>
    </group>)}
    {/* External sight glass exposes the real fill ratio without making HDPE transparent. */}
    <Casing size={[0.09, 0.69, 0.036]} position={[0.32, -0.025, 0.48]} radius={0.01} color="#142c24" />
    {fill > 0 && <mesh position={[0.32, -0.335 + fill * 0.31, 0.502]}><boxGeometry args={[0.048, fill * 0.62, 0.008]} /><meshStandardMaterial color="#5ccebf" metalness={0.2} roughness={0.18} emissive="#24675d" emissiveIntensity={0.18} /></mesh>}
    <Batch color="#becdb8" items={range(9).map((i) => ({ position: [0.368, -0.32 + i * 0.075, 0.503], scale: [i % 2 ? 0.016 : 0.03, 0.005, 0.006] }))} />
    <Casing size={[0.35, 0.12, 0.012]} position={[-0.13, 0.13, 0.48]} color="#25382c" radius={0.005} />
    <Batch color="#9aac98" items={range(4).map((i) => ({ position: [-0.17 + i * 0.039, 0.13, 0.49], scale: [0.019, 0.044, 0.002] }))} />
    <group position={[-0.32, -0.32, 0.49]}>
      <Ring radius={0.031} tube={0.009} color="#899c85" />
      <Tube points={[[0, 0, 0], [0, 0, 0.065], [0, -0.07, 0.09]]} radius={0.017} />
      <mesh position={[0, 0.037, 0.047]}><boxGeometry args={[0.085, 0.018, 0.024]} /><meshStandardMaterial color="#b19959" metalness={0.5} roughness={0.4} /></mesh>
    </group>
    <Tube points={[[0.46, 0.44, -0.3], [0.55, 0.51, -0.3], [0.59, 0.12, -0.32], [0.62, -0.46, -0.22]]} radius={0.008} color="#a5bca6" />
    <Tube points={[[0.44, 0.44, -0.22], [0.53, 0.49, -0.22], [0.55, 0.03, -0.2], [0.6, -0.46, -0.16]]} radius={0.007} color="#708e77" />
    {cutaway && <group>
      <Casing size={[0.96, 0.035, 0.94]} position={[0, -0.44, 0]} color="#354c3e" radius={0.01} />
      <Casing size={[0.96, 0.84, 0.02]} position={[0, -0.025, -0.46]} color="#445e4b" radius={0.005} />
      {fill > 0 && <mesh position={[0, -0.415 + fill * 0.4, 0]}><boxGeometry args={[0.92, fill * 0.8, 0.9]} /><meshStandardMaterial color="#5aa99c" transparent opacity={0.23} depthWrite={false} roughness={0.15} side={DoubleSide} /></mesh>}
      {[-0.22, 0.22].map((x) => <group key={x}>
        <mesh position={[x, -0.38, 0]} rotation={[0, 0, Math.PI / 2]}><cylinderGeometry args={[0.038, 0.038, 0.22, 20]} /><meshStandardMaterial color="#a5ada3" roughness={0.98} /></mesh>
        <Tube points={[[0.44, 0.44, -0.25], [0.4, 0.12, -0.35], [x + 0.15, -0.32, -0.2], [x + 0.12, -0.38, 0]]} radius={0.006} color="#bdc8ae" />
      </group>)}
      {rootPositions.map(([x, z], index) => <RootBundle key={`${x}:${z}`} position={[x, 0.4, z]} seed={index} quality={quality} />)}
    </group>}
  </group>;
}

export function GrowLight({ quality, running }: QualityProps & { running: boolean }) {
  const bars = useMemo<Instance[]>(() => range(6).map((i) => ({ position: [0, -0.25, (i - 2.5) * 0.165], scale: [1, 0.48, 0.095] })), []);
  const fins = useMemo<Instance[]>(() => range(14).map((i) => ({ position: [(i - 6.5) * 0.069, 0.13, 0], scale: [0.008, 0.22, 0.93] })), []);
  const diodes = useMemo<Instance[]>(() => range(quality.name === "low-power" ? 48 : 144).map((i) => {
    const columns = quality.name === "low-power" ? 8 : 24;
    return { position: [(i % columns) / (columns - 1) * 0.92 - 0.46, -0.505, (Math.floor(i / columns) - 2.5) * 0.165], scale: [0.018, 0.025, 0.028] };
  }), [quality.name]);
  return <group position={[0, -2, 0]}>
    <RoundedBox args={[1.04, 0.1, 1.01]} radius={0.018} smoothness={4} castShadow={quality.shadows}><CC0Material surface="metal" color="#d5dfd8" metalness={0.82} roughness={0.42} repeat={[2, 1]} grain={0.35} /></RoundedBox>
    <Batch items={bars} color="#c4c9aa" metalness={0.45} shadows={quality.shadows} />
    <Batch items={fins} surface="metal" color="#b5c5bb" metalness={0.82} />
    <Fasteners positions={[-0.49, 0.49].flatMap((x) => [-0.46, 0.46].map((z): [number, number, number] => [x, 0.065, z]))} radius={0.011} rotation={[-Math.PI / 2, 0, 0]} />
    <Batch items={diodes} color={running ? "#fff3cb" : "#d5c6a3"} emissive="#ffe6ac" intensity={running ? 4 : 0} roughness={0.27} />
    <Casing size={[0.36, 0.62, 0.26]} position={[0, 0.51, 0]} radius={0.025} color="#43564b" metalness={0.65} />
    <Batch color="#182e23" items={range(9).map((i) => ({ position: [(i - 4) * 0.032, 0.835, 0], scale: [0.012, 0.02, 0.22] }))} />
    {[-0.42, 0.42].flatMap((x) => [-0.39, 0.39].map((z) => <Tube key={`${x}:${z}`} points={[[x, 0.1, z], [x * 0.8, 0.95, z * 0.8], [x * 0.7, 1.6, z * 0.7]]} radius={0.004} color="#bec9bb" metalness={0.85} segments={8} />))}
    <Tube points={[[0.15, 0.65, 0], [0.45, 0.75, -0.1], [0.59, 0.4, -0.32], [0.58, -0.1, -0.5]]} radius={0.012} />
  </group>;
}

export function CirculationFan({ quality, running, output = 100 }: QualityProps & { running: boolean; output?: number }) {
  const rotor = useRef<Group>(null);
  const blade = useMemo(() => {
    const shape = new Shape();
    shape.moveTo(0.06, 0.08);
    shape.bezierCurveTo(0.2, 0.1, 0.48, 0.13, 0.42, 0.34);
    shape.bezierCurveTo(0.34, 0.52, 0.12, 0.44, 0.08, 0.2);
    shape.closePath();
    return new ExtrudeGeometry(shape, { depth: 0.025, bevelEnabled: true, bevelSize: 0.006, bevelThickness: 0.006, bevelSegments: 3, steps: 1, curveSegments: 18 });
  }, []);
  useEffect(() => () => blade.dispose(), [blade]);
  useFrame((_, delta) => { if (running && rotor.current) rotor.current.rotation.z -= Math.min(delta, 0.05) * 14 * Math.max(0, Math.min(1, output / 100)); });
  const spokes = useMemo<Instance[]>(() => range(12).map((i) => ({ position: [0, 0, 0.165], rotation: [0, 0, i * Math.PI / 6], scale: [1.1, 0.012, 0.014] })), []);
  return <group rotation={[0, 0.7, 0]}>
    <Casing size={[0.2, 0.54, 0.16]} position={[0, -0.45, -0.28]} radius={0.045} color="#3d5143" />
    <Casing size={[0.45, 0.12, 0.36]} position={[0, -0.69, -0.3]} radius={0.035} />
    <mesh rotation={[Math.PI / 2, 0, 0]} position={[0, 0, -0.15]} castShadow={quality.shadows}><cylinderGeometry args={[0.2, 0.23, 0.3, 24]} /><meshStandardMaterial color="#2d4436" metalness={0.35} roughness={0.5} /></mesh>
    <Ring radius={0.59} tube={0.044} color="#344d3c" />
    <Ring radius={0.57} tube={0.025} position={[0, 0, -0.12]} color="#5a7060" />
    <group ref={rotor}>
      {[0, 1, 2, 3, 4].map((i) => <group key={i} rotation={[0, 0, i * Math.PI * 2 / 5]}><mesh geometry={blade} rotation={[0.18, -0.1, 0]} castShadow={quality.shadows}><CC0Material surface="plastic" color="#91a68c" roughness={0.5} /></mesh></group>)}
    </group>
    {[0.18, 0.23, 0.28, 0.33, 0.38, 0.43, 0.48, 0.53].map((radius) => <Ring key={radius} radius={radius} tube={0.006} position={[0, 0, 0.17 + (0.53 - radius) * 0.11]} color="#9bad99" segments={quality.name === "low-power" ? 32 : 64} />)}
    <Fasteners positions={[-1, 1].flatMap((x) => [-1, 1].map((y): [number, number, number] => [x * 0.395, y * 0.395, 0.06]))} radius={0.022} />
    <Batch items={spokes} color="#8fa38e" metalness={0.75} />
    <mesh position={[0, 0, 0.21]} rotation={[Math.PI / 2, 0, 0]}><cylinderGeometry args={[0.13, 0.13, 0.07, 24]} /><meshStandardMaterial color="#314c39" metalness={0.3} roughness={0.4} /></mesh>
    <mesh position={[0, 0, 0.253]}><circleGeometry args={[0.057, 24]} /><meshStandardMaterial color="#b6c597" metalness={0.65} roughness={0.3} /></mesh>
  </group>;
}

export function Controller({ online }: { online: boolean }) {
  return <group>
    <Casing size={[0.85, 1.15, 0.32]} color="#c2cdc0" radius={0.055} />
    <Casing size={[0.75, 1.05, 0.045]} position={[0, 0, 0.18]} color="#dae1d1" radius={0.028} />
    <Casing size={[0.56, 0.35, 0.03]} position={[0, 0.21, 0.215]} color="#172c22" radius={0.015} />
    <mesh position={[-0.2, 0.25, 0.235]}><planeGeometry args={[0.035, 0.05]} /><meshStandardMaterial color={online ? "#b3e2a0" : "#586255"} emissive={online ? "#8dca81" : "#000000"} emissiveIntensity={0.6} /></mesh>
    <Batch color="#476452" items={range(6).map((i) => ({ position: [0.02, 0.29 - i * 0.027, 0.235], scale: [i % 2 ? 0.22 : 0.32, 0.008, 0.003] }))} />
    <Batch color="#738773" items={range(8).map((i) => ({ position: [-0.245 + i * 0.07, -0.19, 0.208], scale: [0.024, 0.17, 0.012] }))} />
    <Fasteners positions={[-1, 1].flatMap((x) => [-1, 1].map((y): [number, number, number] => [x * 0.32, y * 0.46, 0.212]))} radius={0.025} />
    {[-0.25, 0, 0.25].map((x) => <group key={x}>
      <mesh position={[x, -0.62, 0]}><cylinderGeometry args={[0.055, 0.043, 0.14, 12]} /><meshStandardMaterial color="#263b2e" roughness={0.5} /></mesh>
      <HoseCollar position={[x, -0.62, 0]} radius={0.047} />
      <Tube points={[[x, -0.68, 0], [x, -0.91, 0], [x + 0.1, -1.04, -0.1], [0.42, -1.12, -0.19]]} radius={0.018} />
    </group>)}
    <Tube points={[[0.42, -1.12, -0.19], [0.58, -0.75, -0.19], [0.68, 0.05, -0.12], [0.83, 0.1, 0]]} radius={0.014} color="#8a9c82" />
    <Casing size={[0.15, 0.3, 0.13]} position={[0.84, -0.05, 0]} color="#c1cdbb" radius={0.03} />
    <Batch color="#364f3e" items={range(5).map((i) => ({ position: [0.84, -0.12 + i * 0.04, 0.068], scale: [0.1, 0.012, 0.008] }))} />
  </group>;
}

export function AirPump({ running }: { running: boolean }) {
  return <group>
    <Casing size={[1, 0.48, 0.65]} color="#6b7e6b" radius={0.15} />
    <Casing size={[0.96, 0.08, 0.63]} position={[0, -0.17, 0]} color="#243e2b" radius={0.025} />
    <Batch color="#253d2c" items={range(10).map((i) => ({ position: [-0.32 + i * 0.07, 0.238, 0], scale: [0.018, 0.012, 0.28] }))} />
    <Batch color="#1d3224" sphere items={[-1, 1].flatMap((x) => [-1, 1].map((z) => ({ position: [x * 0.32, -0.255, z * 0.18], scale: [0.09, 0.07, 0.08] })))} />
    <mesh position={[0.32, 0.21, 0.14]} rotation={[-Math.PI / 2, 0, 0]}><circleGeometry args={[0.035, 16]} /><meshStandardMaterial color={running ? "#a9d68c" : "#485143"} emissive={running ? "#7bad65" : "#000000"} emissiveIntensity={0.7} /></mesh>
    {[-0.16, 0.16].map((x) => <group key={x}>
      <HoseCollar position={[x, -0.02, 0.37]} radius={0.04} rotation={[Math.PI / 2, 0, 0]} />
      <Tube points={[[x, -0.02, 0.3], [x, -0.02, 0.43], [x - 0.2, -0.22, 0.62], [-0.7, -0.23, 0.72 + x]]} radius={0.028} color="#9bae92" />
      <mesh position={[x, -0.02, 0.33]} rotation={[Math.PI / 2, 0, 0]}><cylinderGeometry args={[0.045, 0.045, 0.12, 12]} /><meshStandardMaterial color="#b3b294" metalness={0.7} roughness={0.38} /></mesh>
    </group>)}
  </group>;
}
