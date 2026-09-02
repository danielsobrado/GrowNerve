import { useRef } from "react";
import { DoubleSide, type Mesh } from "three";
import { GROW_NERVE_PTL_RECIPES } from "./ptlRecipes";
import { useProceduralSurface } from "./useProceduralSurface";

export function ProceduralFloor() {
  const mesh = useRef<Mesh>(null);
  useProceduralSurface(mesh, GROW_NERVE_PTL_RECIPES["concrete-sealed"]);
  return <mesh ref={mesh} rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.035, 0]} receiveShadow>
    <planeGeometry args={[12, 12, 72, 72]} />
    <meshStandardMaterial color="#29312c" roughness={0.86} />
  </mesh>;
}

export function ReservoirBody() {
  const mesh = useRef<Mesh>(null);
  useProceduralSurface(mesh, GROW_NERVE_PTL_RECIPES["hdpe-reservoir"]);
  return <mesh ref={mesh} castShadow receiveShadow>
    <boxGeometry args={[1, 1, 1, 10, 5, 10]} />
    <meshStandardMaterial color="#52625b" roughness={0.85} />
  </mesh>;
}

export function AbsHousing({ geometry = "pot" }: { geometry?: "fan" | "pot" }) {
  const mesh = useRef<Mesh>(null);
  useProceduralSurface(mesh, GROW_NERVE_PTL_RECIPES["abs-dark"]);
  return <mesh ref={mesh} castShadow receiveShadow>
    {geometry === "fan"
      ? <cylinderGeometry args={[0.62, 0.62, 0.22, 32, 4]} />
      : <cylinderGeometry args={[0.28, 0.34, 0.25, 20, 3]} />}
    <meshStandardMaterial color="#27302b" roughness={0.62} />
  </mesh>;
}

export function LedFixture({ running }: { running: boolean }) {
  const housing = useRef<Mesh>(null);
  useProceduralSurface(housing, GROW_NERVE_PTL_RECIPES["aluminum-brushed"]);
  return <group>
    <mesh ref={housing} castShadow receiveShadow>
      <boxGeometry args={[1, 1, 1, 12, 2, 8]} />
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

function TentPanel({ position, rotation }: { position: [number, number, number]; rotation?: [number, number, number] }) {
  const mesh = useRef<Mesh>(null);
  useProceduralSurface(mesh, GROW_NERVE_PTL_RECIPES["tent-fabric"]);
  return <mesh ref={mesh} position={position} rotation={rotation} receiveShadow>
    <planeGeometry args={[1, 1, 24, 18]} />
    <meshStandardMaterial color="#111713" roughness={0.9} side={DoubleSide} />
  </mesh>;
}

export function TentShell() {
  return <group>
    <TentPanel position={[0, 0, -0.5]} />
    <TentPanel position={[-0.5, 0, 0]} rotation={[0, Math.PI / 2, 0]} />
  </group>;
}
