import { Environment, Html, OrbitControls } from "@react-three/drei";
import { Canvas, useThree } from "@react-three/fiber";
import {
  AlertTriangle, Camera, CircleEllipsis, ClipboardPlus, Droplets, Eye, FlaskConical,
  Gauge, History, Plus, Power, Scissors, Settings, SlidersHorizontal, Wrench,
} from "lucide-react";
import { Suspense, useCallback, useEffect, useMemo, useState, type ComponentType } from "react";
import { ACESFilmicToneMapping, WebGLRenderer, type WebGLRendererParameters } from "three";
import type { EntityType, FarmData, SceneEntity } from "../domain/model";
import { CC0Material, TextureResolution } from "./materials/CC0Material";
import { AirPump, CirculationFan, Controller, DwcReservoir, GrowLight, GrowTent, LettucePlant } from "./models/GrowModels";
import { RefinedGeometry, type RefinedModel } from "./models/RefinedGeometry";
import { HydroponicTower } from "./models/HydroponicTower";
import { GrowOptions } from "./models/GrowOptions";
import { ClimateSystems, type ClimateConfig } from "./models/ClimateSystems";
import { TentDesignerControls, TentDesignerScene } from "./TentDesigner";
import type { TentEditor } from "./tentEditor";
import { type TwinPerformanceProfile, useTwinPerformanceProfile } from "./performance";
import { actionsForProfile, entityKey, sceneBindings } from "./sceneState";
import { latestMeasurementsByChannel, readingByKey } from "./telemetry";
import { TwinHud } from "./TwinHud";
import { formatLength, useLengthUnit } from "../lib/units";
import "./twin-hud.css";

export interface Selection { type: EntityType; id: string }

type ActionIcon = ComponentType<{ size?: number; strokeWidth?: number }>;
type LatestMeasurements = ReturnType<typeof latestMeasurementsByChannel>;
const refinedModels: Record<string, RefinedModel> = { zone: "tent", reservoir: "reservoir", light: "light", fan: "fan", plant: "lettuce", controller: "controller", air_pump: "pump" };

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
    {(hovered || selected) && <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.45, 0]}><ringGeometry args={[0.65, 0.78, 40]} /><meshBasicMaterial color={selected ? "#f2d15b" : "#9be07f"} transparent opacity={0.9} /></mesh>}
    {hovered && <Html position={[0, 0.9, 0]} center className="gn-twin-tooltip"><strong>{title}</strong><span>{detail}</span></Html>}
  </group>;
}

function Scene({ data, latest, quality, selection, onSelect, cutaway }: { data: FarmData; latest: LatestMeasurements; quality: TwinPerformanceProfile; selection?: Selection; onSelect: (selection: Selection) => void; cutaway: boolean }) {
  const layout = data.scene_layouts[0];
  if (!layout) return null;
  const device = (binding: SceneEntity) => data.devices.find((entry) => entry.id === binding.entity_id);
  return <>
    <color attach="background" args={["#101b16"]} />
    <Suspense fallback={null}><Environment files={`${import.meta.env.BASE_URL}textures/cc0/studio_small_09_1k.hdr`} environmentIntensity={1.1} /></Suspense>
    <ambientLight intensity={0.4} />
    <hemisphereLight args={["#f1f3e9", "#38463e", 0.9]} />
    <directionalLight position={[-4, 4, -2]} intensity={1.1} color="#dce9ef" />
    <directionalLight
      position={[5, 7, 4]}
      intensity={2.75}
      color="#fff8ee"
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
    <pointLight position={[0, 2.65, 0]} intensity={data.devices.find((entry) => entry.type === "light")?.state ? 5 : 0} color="#fff8ef" />
    <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.035, 0]} receiveShadow><planeGeometry args={[40, 40]} /><CC0Material surface="concrete" repeat={[20, 20]} color="#303a34" roughness={1} grain={0.14} /></mesh>
    {sceneBindings(data).map((binding, index) => {
      const key = entityKey(binding.entity_type, binding.entity_id), selected = selection && entityKey(selection.type, selection.id) === key;
      const equipment = device(binding), tooltip = tooltipFor(data, latest, binding);
      const reservoir = binding.entity_type === "reservoir" ? data.reservoirs.find((entry) => entry.id === binding.entity_id) : undefined;
      const plant = binding.entity_type === "plant_position" ? data.plant_positions.find((entry) => entry.id === binding.entity_id) : undefined;
      return <Selectable key={key} binding={binding} selected={Boolean(selected)} onSelect={onSelect} title={tooltip.title} detail={tooltip.detail}>
        <RefinedGeometry model={refinedModels[binding.profile] ?? "tent"} enabled={Boolean(refinedModels[binding.profile]) && quality.name !== "low-power"}>
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
        {binding.profile === "hydroponic_tower" && <Suspense fallback={null}><HydroponicTower levels={data.zones.filter((zone) => zone.parent_zone_id === binding.entity_id && zone.type === "level").length || 6} /></Suspense>}
        </RefinedGeometry>
      </Selectable>;
    })}
    <OrbitControls makeDefault minDistance={2.5} maxDistance={18} maxPolarAngle={Math.PI / 2.05} target={[0, 1.1, 0]} />
  </>;
}

function ViewCamera({ view, farmPosition }: { view: string; farmPosition?: [number, number, number] }) {
  const camera = useThree((state) => state.camera);
  useEffect(() => {
    const position: [number, number, number] = view === "designer" ? [4, 3.5, 5] : view === "systems" ? [1.1, 0.9, 1.4] : view === "equipment" ? [1.2, 1.1, 1.6] : view === "tower" ? [3, 2.6, 4] : farmPosition ?? [7, 6, 8];
    camera.position.set(...position);
  }, [camera, view, farmPosition]);
  return null;
}

export type TwinView = "farm" | "tower" | "equipment" | "systems" | "designer";

export function DigitalTwin({ data, selection, onSelect, onAction, view, onViewChange: setView, tentEditor }: { data: FarmData; selection?: Selection; onSelect: (selection: Selection) => void; onAction: (action: string) => void; view: TwinView; onViewChange: (view: TwinView) => void; tentEditor: TentEditor }) {
  const [climate, setClimate] = useState<ClimateConfig>({ category: "fan", variant: "clip", running: true, output: 60 });
  const [growOptions, setGrowOptions] = useState({ led: "panel", pot: "nursery", diameter: 30, height: 30, fill: 80, output: 70 });
  const [towerLevels, setTowerLevels] = useState(6);
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
  const lengthUnit = useLengthUnit();
  const help = quality.touchOptimized ? "Drag to orbit · Pinch to zoom · Tap an object to inspect" : "Drag to orbit · Scroll to zoom · Click an object to inspect";
  return <div className="gn-model-viewer">
    <div className="gn-model-switch" role="group" aria-label="3D view">
      <button aria-pressed={view === "farm"} onClick={() => setView("farm")}>Farm</button>
      <button aria-pressed={view === "tower"} onClick={() => setView("tower")}>Hydroponic tower</button>
      <button aria-pressed={view === "equipment"} onClick={() => setView("equipment")}>Lights & pots</button>
      <button aria-pressed={view === "systems"} onClick={() => setView("systems")}>Climate & irrigation</button>
      <button aria-pressed={view === "designer"} onClick={() => setView("designer")}>Tent layout</button>
      {view === "tower" && <label>Levels <select aria-label="Tower levels" value={towerLevels} onChange={(event) => setTowerLevels(Number(event.target.value))}>{[3, 4, 6, 8].map((count) => <option key={count} value={count}>{count}</option>)}</select><span>{towerLevels * 3} planting sites · Model preview</span></label>}
    </div>
    {view === "equipment" && <div className="gn-model-switch" role="group" aria-label="Equipment options">
      <label>LED <select aria-label="LED style" value={growOptions.led} onChange={(event) => setGrowOptions({ ...growOptions, led: event.target.value })}><option value="panel">Panel</option><option value="bar">Linear bar</option><option value="multi_bar">Multi-bar</option></select></label>
      <label>Pot <select aria-label="Pot style" value={growOptions.pot} onChange={(event) => setGrowOptions({ ...growOptions, pot: event.target.value })}><option value="nursery">Nursery</option><option value="fabric">Fabric bag</option><option value="ceramic">Ceramic</option></select></label>
      {([['diameter', 'Diameter', [20, 30, 40, 50]], ['height', 'Height', [20, 30, 40, 50]], ['fill', 'Soil fill', [0, 25, 50, 80, 100]], ['output', 'LED brightness', [0, 25, 50, 70, 100]]] as const).map(([key, label, values]) => <label key={key}>{label}<select aria-label={label} value={growOptions[key]} onChange={(event) => setGrowOptions({ ...growOptions, [key]: Number(event.target.value) })}>{values.map((value) => <option key={value} value={value}>{key === 'diameter' || key === 'height' ? formatLength(value / 100, lengthUnit) : `${value}%`}</option>)}</select></label>)}
      <span>Model preview · visual brightness</span>
    </div>}
    {view === "systems" && <div className="gn-model-switch" role="group" aria-label="Climate equipment options">
      <label>Equipment <select aria-label="Equipment category" value={climate.category} onChange={(event) => { const category = event.target.value as ClimateConfig['category']; setClimate({ ...climate, category, variant: category === 'fan' ? 'clip' : category === 'humidifier' ? 'ultrasonic' : 'drip' }); }}><option value="fan">Fans</option><option value="humidifier">Humidifiers</option><option value="irrigation">Automated irrigation</option></select></label>
      <label>Type <select aria-label="Equipment variant" value={climate.variant} onChange={(event) => setClimate({ ...climate, variant: event.target.value })}>{(climate.category === 'fan' ? [['clip', 'Clip fan'], ['inline', 'Inline duct fan']] : climate.category === 'humidifier' ? [['ultrasonic', 'Ultrasonic'], ['evaporative', 'Evaporative wick']] : [['drip', 'Drip stakes'], ['ring', 'Watering rings']]).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      {climate.category === 'fan' || (climate.category === 'humidifier' && climate.variant === 'ultrasonic') ? <><button aria-pressed={climate.running} onClick={() => setClimate({ ...climate, running: !climate.running })}>{climate.running ? 'Pause animation' : 'Start animation'}</button><label>Animation <select aria-label="Animation intensity" value={climate.output} onChange={(event) => setClimate({ ...climate, output: Number(event.target.value) })}>{[0, 30, 60, 100].map((value) => <option key={value} value={value}>{value}%</option>)}</select></label></> : null}
      <span>Model preview{climate.category === 'irrigation' ? ' · 2 irrigation zones' : ''}</span>
    </div>}
    {view === "designer" && <TentDesignerControls editor={tentEditor} />}
    <div className="gn-twin-wrap">
    <Canvas shadows={quality.shadows} gl={createRenderer} camera={{ position: data.scene_layouts[0]?.camera_position ?? [7, 6, 8], fov: 42 }} dpr={quality.dpr}>
      <ViewCamera view={view} farmPosition={data.scene_layouts[0]?.camera_position} />
      <TextureResolution.Provider value={quality.name === "desktop" ? 1024 : 512}>
        {view === "farm" ? <Scene data={data} latest={latest} quality={quality} selection={selection} onSelect={onSelect} cutaway={cutaway} /> : view === "designer" ? <TentDesignerScene editor={tentEditor} quality={quality} /> : <>
          <color attach="background" args={["#15201b"]} />
          <ambientLight intensity={0.65} />
          <directionalLight position={[3, 5, 4]} intensity={3} castShadow={quality.shadows} shadow-normalBias={0.015} />
          <directionalLight position={[-3, 3, -2]} intensity={1.4} color="#dde8f0" />
          <Suspense fallback={null}>
            <Environment files={`${import.meta.env.BASE_URL}textures/cc0/studio_small_09_1k.hdr`} environmentIntensity={0.8} />
            {view === "tower" ? <HydroponicTower levels={towerLevels} /> : view === "systems" ? <ClimateSystems {...climate} /> : <GrowOptions {...growOptions} />}
          </Suspense>
          <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.01, 0]} receiveShadow><planeGeometry args={[20, 20]} /><CC0Material surface="concrete" repeat={[10, 10]} color="#344239" roughness={1} grain={0.12} /></mesh>
          <OrbitControls makeDefault minDistance={view === "tower" ? 1.2 : 0.6} maxDistance={12} maxPolarAngle={Math.PI / 2.02} target={[0, view === "systems" ? 0.22 : view === "equipment" ? 0.45 : (0.5 + towerLevels * 0.3) / 2, 0]} />
        </>}
      </TextureResolution.Provider>
    </Canvas>
    <div className="gn-renderer-badge"><span />{rendererLabel} · {quality.name}</div>
    {view === "farm" && <><button className="gn-cutaway-toggle" aria-pressed={cutaway} onClick={() => setCutaway((value) => !value)}><Eye size={14} />{cutaway ? "Close reservoir" : "Look inside reservoir"}</button><TwinHud data={data} /></>}
    <div className="gn-scene-help">{help}</div>
    {view === "farm" && selectedBinding && selectedTooltip && <div className="gn-context-panel">
      <div className="gn-context-target"><span>Selected</span><strong>{selectedTooltip.title}</strong><small>{selectedTooltip.detail}</small></div>
      <div className="gn-radial" role="toolbar" aria-label={`${selectedTooltip.title} actions`}>{actionsForProfile(selectedBinding.profile).slice(0, 5).map((action) => { const Icon = actionIcon(action); return <button key={action} title={action} aria-label={action} onClick={() => onAction(action)}><Icon size={15} strokeWidth={1.8} /><span>{action}</span></button>; })}</div>
    </div>}
    </div>
  </div>;
}
