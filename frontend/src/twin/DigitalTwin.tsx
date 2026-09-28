import { Html, OrbitControls } from "@react-three/drei";
import { Canvas } from "@react-three/fiber";
import {
  AlertTriangle, Camera, CircleEllipsis, ClipboardPlus, Droplets, Eye, FlaskConical,
  Gauge, History, Plus, Power, Scissors, Settings, SlidersHorizontal, Wrench,
} from "lucide-react";
import { useCallback, useMemo, useState, type ComponentType } from "react";
import { ACESFilmicToneMapping, WebGLRenderer, type WebGLRendererParameters } from "three";
import type { EntityType, FarmData, SceneEntity } from "../domain/model";
import { CC0Material, TextureResolution } from "./materials/CC0Material";
import { AirPump, CirculationFan, Controller, DwcReservoir, GrowLight, GrowTent, LettucePlant } from "./models/GrowModels";
import { type TwinPerformanceProfile, useTwinPerformanceProfile } from "./performance";
import { actionsForProfile, entityKey, sceneBindings } from "./sceneState";
import { latestMeasurementsByChannel, readingByKey } from "./telemetry";
import { TwinHud } from "./TwinHud";
import "./models/model-controls.css";

export interface Selection { type: EntityType; id: string }

type ActionIcon = ComponentType<{ size?: number; strokeWidth?: number }>;
type LatestMeasurements = ReturnType<typeof latestMeasurementsByChannel>;

function actionIcon(action: string): ActionIcon {
  switch (action) {
    case "Inspect": return Eye;
    case "History": return History;
    case "Calibrate": return Gauge;
    case "Alerts": return AlertTriangle;
    case "Configure": return Settings;
    case "Set output": return SlidersHorizontal;
    case "Override": return Power;
    case "Maintenance": return Wrench;
    case "Observe": return ClipboardPlus;
    case "Photo": return Camera;
    case "Harvest": return Scissors;
    case "Chemistry": return FlaskConical;
    case "Add input": return Plus;
    case "Refill": return Droplets;
    default: return CircleEllipsis;
  }
}

const readingSummary = (data: FarmData, latest: LatestMeasurements, key: string, binding: SceneEntity) => {
  const reading = readingByKey(data, latest, key, { entityType: binding.entity_type, entityId: binding.entity_id });
  return reading ? `${reading.displayValue}${reading.stale ? " · stale" : ""}` : undefined;
};

function tooltipFor(data: FarmData, latest: LatestMeasurements, binding: SceneEntity): { title: string; detail: string } {
  if (binding.entity_type === "device") {
    const device = data.devices.find((entry) => entry.id === binding.entity_id);
    if (device) {
      const state = device.online ? device.state ? "Running" : "Stopped" : "Offline";
      const output = device.output_percent !== undefined ? ` · ${device.output_percent}%` : "";
      return { title: device.name, detail: `${state}${output}` };
    }
  }

  if (binding.entity_type === "reservoir") {
    const reservoir = data.reservoirs.find((entry) => entry.id === binding.entity_id);
    if (reservoir) {
      const temperature = readingSummary(data, latest, "water.temperature", binding);
      const level = readingSummary(data, latest, "water.level", binding) ?? `${Math.round(reservoir.level_percent)}% level`;
      return { title: reservoir.name, detail: [temperature, level].filter(Boolean).join(" · ") };
    }
  }

  if (binding.entity_type === "plant_position") {
    const plant = data.plant_positions.find((entry) => entry.id === binding.entity_id);
    if (plant) return { title: `Plant ${plant.code}`, detail: `${plant.health === "normal" ? "Healthy" : plant.health} · ${plant.occupied ? "occupied" : "empty"}` };
  }

  if (binding.entity_type === "zone") {
    const zone = data.zones.find((entry) => entry.id === binding.entity_id);
    if (zone) {
      const temperature = readingSummary(data, latest, "air.temperature", binding);
      const humidity = readingSummary(data, latest, "air.humidity", binding);
      const telemetry = [temperature, humidity].filter(Boolean).join(" · ");
      const positions = data.plant_positions.filter((entry) => entry.zone_id === zone.id).length;
      return { title: zone.name, detail: telemetry || `${zone.type} · ${positions} plant positions` };
    }
  }

  return { title: binding.profile.replaceAll("_", " "), detail: "Click to inspect" };
}

function Selectable({ binding, selected, onSelect, title, detail, children }: { binding: SceneEntity; selected: boolean; onSelect: (selection: Selection) => void; title: string; detail: string; children: React.ReactNode }) {
  const [hovered, setHovered] = useState(false);
  return <group position={binding.position} scale={binding.scale} onClick={(event) => { event.stopPropagation(); onSelect({ type: binding.entity_type, id: binding.entity_id }); }} onPointerOver={(event) => { event.stopPropagation(); setHovered(true); document.body.style.cursor = "pointer"; }} onPointerOut={() => { setHovered(false); document.body.style.cursor = "default"; }}>
    {children}
    {(hovered || selected) && <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.45, 0]}><ringGeometry args={[0.65, 0.78, 40]} /><meshBasicMaterial color={selected ? "#8ddd7b" : "#9ee493"} transparent opacity={0.9} /></mesh>}
    {hovered && <Html position={[0, 0.9, 0]} center className="gn-twin-tooltip"><strong>{title}</strong><span>{detail}</span></Html>}
  </group>;
}

function Scene({ data, latest, quality, selection, onSelect, cutaway }: { data: FarmData; latest: LatestMeasurements; quality: TwinPerformanceProfile; selection?: Selection; onSelect: (selection: Selection) => void; cutaway: boolean }) {
  const layout = data.scene_layouts[0];
  if (!layout) return null;
  const device = (binding: SceneEntity) => data.devices.find((entry) => entry.id === binding.entity_id);
  return <>
    <color attach="background" args={["#101b16"]} />
    <ambientLight intensity={0.45} />
    <hemisphereLight args={["#e7efd8", "#283c30", 1.1]} />
    <directionalLight position={[-4, 4, -2]} intensity={1.2} color="#c5e0df" />
    <directionalLight
      position={[5, 7, 4]}
      intensity={2.5}
      color="#fff8df"
      castShadow={quality.shadows}
      shadow-mapSize-width={quality.shadowMapSize}
      shadow-mapSize-height={quality.shadowMapSize}
      shadow-normalBias={0.035}
      shadow-camera-left={-5}
      shadow-camera-right={5}
      shadow-camera-top={5}
      shadow-camera-bottom={-5}
      shadow-camera-far={20}
    />
    <pointLight position={[0, 2.65, 0]} intensity={data.devices.find((entry) => entry.type === "light")?.state ? 7 : 0} color="#fff1d2" />
    <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.035, 0]} receiveShadow><planeGeometry args={[12, 12]} /><CC0Material surface="concrete" repeat={[6, 6]} color="#5b655f" roughness={0.92} grain={0.5} /></mesh>
    {sceneBindings(data).map((binding, index) => {
      const key = entityKey(binding.entity_type, binding.entity_id), selected = selection && entityKey(selection.type, selection.id) === key;
      const equipment = device(binding), tooltip = tooltipFor(data, latest, binding);
      const reservoir = binding.entity_type === "reservoir" ? data.reservoirs.find((entry) => entry.id === binding.entity_id) : undefined;
      const plant = binding.entity_type === "plant_position" ? data.plant_positions.find((entry) => entry.id === binding.entity_id) : undefined;
      return <Selectable key={key} binding={binding} selected={Boolean(selected)} onSelect={onSelect} title={tooltip.title} detail={tooltip.detail}>
        {binding.profile === "zone" && <GrowTent quality={quality} />}
        {binding.profile === "reservoir" && <DwcReservoir quality={quality} level={reservoir?.level_percent ?? 0} cutaway={cutaway} rootPositions={layout.entities
          .filter((entry) => entry.profile === "plant" && data.plant_positions.some((position) => position.id === entry.entity_id && position.occupied && position.zone_id === reservoir?.zone_id))
          .map((entry): [number, number] => [(entry.position[0] - binding.position[0]) / binding.scale[0], (entry.position[2] - binding.position[2]) / binding.scale[2]])
          .filter(([x, z]) => Math.abs(x) < 0.46 && Math.abs(z) < 0.46)} />}
        {binding.profile === "light" && <GrowLight quality={quality} running={Boolean(equipment?.state)} />}
        {binding.profile === "fan" && <CirculationFan quality={quality} running={Boolean(equipment?.state)} output={equipment?.output_percent} />}
        {binding.profile === "plant" && <LettucePlant quality={quality} attention={plant?.health === "attention"} occupied={Boolean(plant?.occupied)} seed={index} />}
        {binding.profile === "controller" && <Controller online={Boolean(equipment?.online)} />}
        {binding.profile === "air_pump" && <AirPump running={Boolean(equipment?.state)} />}
      </Selectable>;
    })}
    <OrbitControls makeDefault minDistance={2.5} maxDistance={18} maxPolarAngle={Math.PI / 2.05} target={[0, 1.1, 0]} />
  </>;
}

export function DigitalTwin({ data, selection, onSelect, onAction }: { data: FarmData; selection?: Selection; onSelect: (selection: Selection) => void; onAction: (action: string) => void }) {
  const [renderer, setRenderer] = useState<"starting" | "webgpu" | "webgl">("starting");
  const [cutaway, setCutaway] = useState(false);
  const quality = useTwinPerformanceProfile();
  const latest = useMemo(() => latestMeasurementsByChannel(data), [data]);
  const selectedBinding = selection && sceneBindings(data).find((entry) => entry.entity_type === selection.type && entry.entity_id === selection.id);
  const selectedTooltip = selectedBinding ? tooltipFor(data, latest, selectedBinding) : undefined;
  const createRenderer = useCallback(async (options: WebGLRendererParameters) => {
    if (typeof navigator !== "undefined" && "gpu" in navigator) {
      try {
        const { WebGPURenderer } = await import("three/webgpu");
        const webgpu = new WebGPURenderer({ canvas: options.canvas as HTMLCanvasElement, antialias: quality.antialias });
        await webgpu.init();
        webgpu.toneMapping = ACESFilmicToneMapping;
        setRenderer("webgpu");
        return webgpu;
      } catch {
        // A reported adapter can still fail initialization; use the explicit safe fallback.
      }
    }
    setRenderer("webgl");
    const webgl = new WebGLRenderer({ ...options, antialias: quality.antialias });
    webgl.toneMapping = ACESFilmicToneMapping;
    return webgl;
  }, [quality.antialias]);
  const rendererLabel = renderer === "webgpu" ? "WebGPU · CC0 materials" : renderer === "webgl" ? "WebGL · CC0 materials" : "Starting renderer";
  const help = quality.touchOptimized ? "Drag to orbit · Pinch to zoom · Tap an object to inspect" : "Drag to orbit · Scroll to zoom · Click an object to inspect";
  return <div className="gn-twin-wrap">
    <Canvas shadows={quality.shadows} gl={createRenderer} camera={{ position: data.scene_layouts[0]?.camera_position ?? [7, 6, 8], fov: 42 }} dpr={quality.dpr}>
      <TextureResolution.Provider value={quality.name === "desktop" ? 1024 : 512}>
        <Scene data={data} latest={latest} quality={quality} selection={selection} onSelect={onSelect} cutaway={cutaway} />
      </TextureResolution.Provider>
    </Canvas>
    <div className="gn-renderer-badge"><span />{rendererLabel} · {quality.name}</div>
    <button className="gn-cutaway-toggle" aria-pressed={cutaway} onClick={() => setCutaway((value) => !value)}><Eye size={14} />{cutaway ? "Close reservoir" : "Look inside reservoir"}</button>
    <TwinHud data={data} />
    <div className="gn-scene-help">{help}</div>
    {selectedBinding && selectedTooltip && <div className="gn-context-panel">
      <div className="gn-context-target"><span>Selected</span><strong>{selectedTooltip.title}</strong><small>{selectedTooltip.detail}</small></div>
      <div className="gn-radial" role="toolbar" aria-label={`${selectedTooltip.title} actions`}>{actionsForProfile(selectedBinding.profile).slice(0, 5).map((action) => { const Icon = actionIcon(action); return <button key={action} title={action} aria-label={action} onClick={() => onAction(action)}><Icon size={15} strokeWidth={1.8} /><span>{action}</span></button>; })}</div>
    </div>}
  </div>;
}
