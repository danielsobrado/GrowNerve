import { useRef } from "react";
import { DoubleSide, type Mesh } from "three";
import type { TwinPerformanceProfile } from "../performance";
import { GROW_NERVE_PTL_RECIPES } from "./ptlRecipes";
import { useProceduralSurface } from "./useProceduralSurface";

type SurfaceQuality = Pick<TwinPerformanceProfile, "geometryScale" | "ptlResolution">;

const detail = (base: number, scale: number, minimum: number): number => Math.max(minimum, Math.round(base * scale));

export function ProceduralFloor({ quality }: { quality: SurfaceQuality }) {
  const mesh = useRef<Mesh>(null);
  useProceduralSurface(mesh, GROW_NERVE_PTL_RECIPES["concrete-sealed"], { resolution: quality.ptlResolution });
  const segments = detail(72, quality.geometryScale, 24);
  return <mesh ref={mesh} rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.035, 0]} receiveShadow>
    <planeGeometry args={[12, 12, segments, segments]} />
    <meshStandardMaterial color="#29312c" roughness={0.86} />
  </mesh>;
}

export function ReservoirBody({ quality }: { quality: SurfaceQuality }) {
  const mesh = useRef<Mesh>(null);
  useProceduralSurface(mesh, GROW_NERVE_PTL_RECIPES["hdpe-reservoir"], { resolution: quality.ptlResolution });
  const horizontal = detail(10, quality.geometryScale, 4);
  const vertical = detail(5, quality.geometryScale, 2);
  return <mesh ref={mesh} castShadow receiveShadow>
    <boxGeometry args={[1, 1, 1, horizontal, vertical, horizontal]} />
    <meshStandardMaterial color="#52625b" roughness={0.85} />
  </mesh>;
}

export function AbsHousing({ quality, geometry = "pot" }: { quality: SurfaceQuality; geometry?: "fan" | "pot" }) {
  const mesh = useRef<Mesh>(null);
  useProceduralSurface(mesh, GROW_NERVE_PTL_RECIPES["abs-dark"], { resolution: quality.ptlResolution });
  const radialSegments = detail(geometry === "fan" ? 32 : 20, quality.geometryScale, 12);
  const heightSegments = detail(geometry === "fan" ? 4 : 3, quality.geometryScale, 1);
  return <mesh ref={mesh} castShadow receiveShadow>
    {geometry === "fan"
      ? <cylinderGeometry args={[0.62, 0.62, 0.22, radialSegments, heightSegments]} />
      : <cylinderGeometry args={[0.28, 0.34, 0.25, radialSegments, heightSegments]} />}
    <meshStandardMaterial color="#27302b" roughness={0.62} />
  </mesh>;
}

export function LedFixture({ quality, running }: { quality: SurfaceQuality; running: boolean }) {
  const housing = useRef<Mesh>(null);
  useProceduralSurface(housing, GROW_NERVE_PTL_RECIPES["aluminum-brushed"], { resolution: quality.ptlResolution });
  const horizontal = detail(12, quality.geometryScale, 4);
  const vertical = detail(2, quality.geometryScale, 1);
  const depth = detail(8, quality.geometryScale, 3);
  return <group>
    <mesh ref={housing} castShadow receiveShadow>
      <boxGeometry args={[1, 1, 1, horizontal, vertical, depth]} />
      <meshStandardMaterial color="#89928e" metalness={0.76} roughness={0.38} />
    </mesh>
    <mesh position={[0, -0.53, 0]} scale={[0.9, 0.05, 0.82]}>
      <boxGeometry args={[1, 1, 1]} />
      <meshStandardMaterial
        color={running ? "#fff0b2" : "#444944"}
        emissive={running ? "#ffe27d" : "#000000"}
        emissiveIntensity={running ? 2.1 : 0}
        roughness={0.28}
      />
    </mesh>
  </group>;
}

function TentPanel({ quality, position, rotation }: { quality: SurfaceQuality; position: [number, number, number]; rotation?: [number, number, number] }) {
  const mesh = useRef<Mesh>(null);
  useProceduralSurface(mesh, GROW_NERVE_PTL_RECIPES["tent-fabric"], { resolution: quality.ptlResolution });
  const widthSegments = detail(24, quality.geometryScale, 8);
  const heightSegments = detail(18, quality.geometryScale, 6);
  return <mesh ref={mesh} position={position} rotation={rotation} receiveShadow>
    <planeGeometry args={[1, 1, widthSegments, heightSegments]} />
    <meshStandardMaterial color="#111713" roughness={0.9} side={DoubleSide} />
  </mesh>;
}

export function TentShell({ quality }: { quality: SurfaceQuality }) {
  return <group>
    <TentPanel quality={quality} position={[0, 0, -0.5]} />
    <TentPanel quality={quality} position={[-0.5, 0, 0]} rotation={[0, Math.PI / 2, 0]} />
  </group>;
}
