import { Html, OrbitControls } from "@react-three/drei";
import { Canvas, useFrame } from "@react-three/fiber";
import { useRef, useState } from "react";
import type { Group } from "three";
import type { EntityType, FarmData, SceneEntity } from "../domain/model";
import { actionsForProfile, entityKey } from "./sceneState";

export interface Selection { type: EntityType; id: string }

function Selectable({ binding, selected, onSelect, children }: { binding: SceneEntity; selected: boolean; onSelect: (selection: Selection) => void; children: React.ReactNode }) {
  const [hovered, setHovered] = useState(false);
  return <group position={binding.position} scale={binding.scale} onClick={(event) => { event.stopPropagation(); onSelect({ type: binding.entity_type, id: binding.entity_id }); }} onPointerOver={(event) => { event.stopPropagation(); setHovered(true); document.body.style.cursor = "pointer"; }} onPointerOut={() => { setHovered(false); document.body.style.cursor = "default"; }}>
    {children}
    {(hovered || selected) && <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.45, 0]}><ringGeometry args={[0.65, 0.78, 40]} /><meshBasicMaterial color={selected ? "#f0b429" : "#9ee493"} transparent opacity={0.9} /></mesh>}
    {hovered && <Html position={[0, 0.9, 0]} center className="gn-twin-tooltip"><strong>{binding.profile}</strong><span>Click to inspect</span></Html>}
  </group>;
}

function Fan({ running }: { running: boolean }) {
  const blades = useRef<Group>(null);
  useFrame((_, delta) => { if (running && blades.current) blades.current.rotation.z -= delta * 8; });
  return <group rotation={[0, Math.PI / 2, 0]}><mesh><cylinderGeometry args={[0.62, 0.62, 0.22, 32]} /><meshStandardMaterial color="#313a35" /></mesh><group ref={blades} position={[0, 0.13, 0]}>{[0, 1, 2, 3].map((blade) => <mesh key={blade} rotation={[Math.PI / 2, 0, blade * Math.PI / 2]} position={[0.23 * Math.cos(blade * Math.PI / 2), 0, 0.23 * Math.sin(blade * Math.PI / 2)]}><boxGeometry args={[0.42, 0.09, 0.12]} /><meshStandardMaterial color="#84a98c" /></mesh>)}</group></group>;
}

function Plant({ attention }: { attention: boolean }) {
  return <group><mesh position={[0, -0.35, 0]}><cylinderGeometry args={[0.28, 0.34, 0.25, 20]} /><meshStandardMaterial color="#303833" /></mesh>{[-0.25, 0, 0.25].map((offset, index) => <mesh key={offset} position={[offset, 0.02 + index * 0.08, 0]} rotation={[0.2, offset * 2, offset]}><sphereGeometry args={[0.32, 20, 12]} /><meshStandardMaterial color={attention ? "#d9a441" : index === 1 ? "#5b9a62" : "#70b870"} roughness={0.8} /></mesh>)}</group>;
}

function Scene({ data, selection, onSelect }: { data: FarmData; selection?: Selection; onSelect: (selection: Selection) => void }) {
  const layout = data.scene_layouts[0];
  if (!layout) return null;
  const device = (binding: SceneEntity) => data.devices.find((entry) => entry.id === binding.entity_id);
  return <>
    <color attach="background" args={["#111914"]} />
    <ambientLight intensity={1.4} />
    <directionalLight position={[5, 7, 4]} intensity={2.3} color="#fff8df" />
    <pointLight position={[0, 3.5, 0]} intensity={data.devices.find((entry) => entry.type === "light")?.state ? 18 : 0} color="#fff1b8" />
    <gridHelper args={[12, 24, "#294235", "#1b2921"]} />
    {layout.entities.map((binding) => {
      const key = entityKey(binding.entity_type, binding.entity_id), selected = selection && entityKey(selection.type, selection.id) === key;
      const equipment = device(binding);
      return <Selectable key={key} binding={binding} selected={Boolean(selected)} onSelect={onSelect}>
        {binding.profile === "zone" && <mesh><boxGeometry args={[1, 1, 1]} /><meshStandardMaterial color="#264737" transparent opacity={0.12} wireframe /></mesh>}
        {binding.profile === "reservoir" && <group><mesh><boxGeometry args={[1, 1, 1]} /><meshStandardMaterial color="#52625b" roughness={0.85} /></mesh><mesh position={[0, data.reservoirs[0].level_percent / 100 - 0.5, 0]} scale={[0.94, 0.04, 0.94]}><boxGeometry args={[1, 1, 1]} /><meshStandardMaterial color="#4bb2c9" transparent opacity={0.76} /></mesh></group>}
        {binding.profile === "light" && <mesh><boxGeometry args={[1, 1, 1]} /><meshStandardMaterial color={equipment?.state ? "#f3d36a" : "#50534f"} emissive={equipment?.state ? "#ffe58a" : "#000000"} emissiveIntensity={equipment?.state ? 1.2 : 0} /></mesh>}
        {binding.profile === "fan" && <Fan running={Boolean(equipment?.state)} />}
        {binding.profile === "plant" && <Plant attention={data.plant_positions.find((entry) => entry.id === binding.entity_id)?.health === "attention"} />}
      </Selectable>;
    })}
    <OrbitControls makeDefault minDistance={4} maxDistance={18} maxPolarAngle={Math.PI / 2.05} target={[0, 1.1, 0]} />
  </>;
}

export function DigitalTwin({ data, selection, onSelect, onAction }: { data: FarmData; selection?: Selection; onSelect: (selection: Selection) => void; onAction: (action: string) => void }) {
  const selectedBinding = selection && data.scene_layouts[0]?.entities.find((entry) => entry.entity_type === selection.type && entry.entity_id === selection.id);
  return <div className="gn-twin-wrap">
    <Canvas camera={{ position: data.scene_layouts[0]?.camera_position ?? [7, 6, 8], fov: 42 }} dpr={[1, 1.75]} onPointerMissed={() => onSelect({ type: "facility", id: data.facilities[0].id })}>
      <Scene data={data} selection={selection} onSelect={onSelect} />
    </Canvas>
    <div className="gn-renderer-badge"><span />{typeof navigator !== "undefined" && "gpu" in navigator ? "WebGPU ready" : "WebGL fallback"}</div>
    {selectedBinding && <div className="gn-radial" aria-label="Entity actions">{actionsForProfile(selectedBinding.profile).slice(0, 5).map((action, index) => <button key={action} style={{ "--index": index } as React.CSSProperties} onClick={() => onAction(action)}>{action}</button>)}</div>}
  </div>;
}
