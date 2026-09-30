import { useGLTF, TransformControls, OrbitControls, Environment, Html } from "@react-three/drei";
import { useThree } from "@react-three/fiber";
import { Suspense, useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { Copy, Crosshair, Download, Maximize2, Move, RotateCw, Trash2, Upload, X, type LucideIcon } from "lucide-react";
import { Box3, BoxGeometry, BufferGeometry, EdgesGeometry, Float32BufferAttribute, Group, InstancedMesh, Mesh, Object3D, Vector3 } from "three";
import { AirPump, Controller, DwcReservoir, GrowTent, LettucePlant } from "./models/GrowModels";
import { HydroponicTower } from "./models/HydroponicTower";
import type { TwinPerformanceProfile } from "./performance";
import { catalog, displayName, footprint, gridOptions, itemName, placeItem, resizeTent, spaceLimits, updateItem, type Environment as SpaceEnvironment, type LayoutItem, type Vec3 } from "./tentLayout";
import type { TentEditor } from "./tentEditor";
import { LengthInput } from "./TentObjectPanel";
import { formatLength, fromMeters, useLengthUnit } from "../lib/units";
import "./tent-designer.css";

type Editor = TentEditor;

export function TentDesignerControls({ editor }: { editor: Editor }) {
  const { layout, item, select, commit } = editor;
  const unit = useLengthUnit();
  const [kind, setKind] = useState(catalog[0].kind);
  const fileInput = useRef<HTMLInputElement>(null);
  const outdoor = layout.environment === "outdoor";
  const dimensions = outdoor ? ['Plot width', 'Height limit', 'Plot depth'] : ['Tent width', 'Tent height', 'Tent depth'];
  const limits = spaceLimits[layout.environment];
  const changeVector = (vector: Vec3, axis: number, value: number): Vec3 => vector.map((v, i) => i === axis ? value : v) as Vec3;
  return <div className="gn-tent-editor">
    <div className="gn-model-switch">
      <label>Add equipment<select aria-label="Add equipment type" value={kind} onChange={(event) => setKind(event.target.value)}>{catalog.map((entry) => <option key={entry.kind} value={entry.kind}>{entry.name}</option>)}</select></label>
      <button onClick={() => editor.add(kind)}>{outdoor ? "Add to plot" : "Add to tent"}</button>
      <label>Grid<select aria-label="Snap grid" value={layout.grid} onChange={(event) => commit((current) => resizeTent({ ...current, grid: Number(event.target.value) }, current.tent))}>{gridOptions.map((grid) => <option key={grid} value={grid}>{formatLength(grid, unit)}</option>)}</select></label>
      <button disabled={!editor.canUndo} onClick={editor.undo}>Undo</button><button disabled={!editor.canRedo} onClick={editor.redo}>Redo</button>
      <label>Selected object<select aria-label="Selected layout object" value={item?.id ?? ''} onChange={(event) => select(event.target.value || undefined)}><option value="">Select an object</option>{layout.items.map((entry, index) => <option key={entry.id} value={entry.id}>{index + 1}. {displayName(entry)}</option>)}</select></label>
      <button onClick={editor.exportFile} title="Download this layout with every object and its specifications"><Download size={14} /> Export layout</button>
      <button onClick={() => fileInput.current?.click()} title="Replace this layout from a .grownerve-layout.json file"><Upload size={14} /> Import layout</button>
      <input ref={fileInput} className="gn-hidden" type="file" accept=".json,application/json" aria-label="Layout file to import" onChange={(event) => { const file = event.target.files?.[0]; if (file) void editor.importFile(file); event.target.value = ""; }} />
    </div>
    <details><summary>{outdoor ? "Outdoor plot" : "Grow tent"} · {layout.name} · {layout.tent.map((v) => fromMeters(v, unit)).join(' × ')} {unit}</summary><div className="gn-model-switch">
      <label>Layout name<input key={`name-${layout.name}`} aria-label="Layout name" type="text" maxLength={80} defaultValue={layout.name} onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }} onBlur={(event) => { if (event.currentTarget.value.trim() !== layout.name) editor.rename(event.currentTarget.value.trim()); }} /></label>
      <label>Growing space<select aria-label="Growing space" value={layout.environment} onChange={(event) => { const environment = event.target.value as SpaceEnvironment; const bounds = spaceLimits[environment]; commit((current) => resizeTent(current, current.tent.map((v, axis) => Math.min(axis === 1 ? bounds.maxHeight : bounds.max, v)) as Vec3, environment)); }}><option value="tent">Grow tent</option><option value="outdoor">Outdoor plot (no tent)</option></select></label>
      {dimensions.map((label, axis) => <LengthInput key={label} label={label} value={layout.tent[axis]} min={limits.min} max={axis === 1 ? limits.maxHeight : limits.max} onChange={(value) => commit((current) => resizeTent(current, changeVector(current.tent, axis, value)))} />)}
    </div></details>
    <p role="status">{editor.message} {editor.placing && item ? 'Click or tap the floor grid to place the selected object.' : item ? 'Use the ring next to the object or the panel on the right to move, resize, rotate, duplicate or remove it.' : `Click an object to edit it. Sizes use ${unit === 'cm' ? 'centimetres' : 'feet'} (change in Settings).`}</p>
  </div>;
}

function ModuleModel({ kind }: { kind: string }) {
  const asset = kind.startsWith('sensor_') ? 'layout-sensors' : kind.startsWith('pot_') || kind.startsWith('led_') ? 'grow-options' : 'climate-systems';
  const { scene } = useGLTF(`${import.meta.env.BASE_URL}models/blender/${asset}.glb`);
  const model = useMemo(() => {
    const sources = new Map<string, Object3D>();
    scene.traverse((obj) => { if (obj.userData.equipmentModule) sources.set(obj.userData.equipmentModule as string, obj); });
    const root = new Group();
    const source = sources.get(kind);
    if (!source) throw new Error(`Missing layout model ${kind}`);
    root.add(source.clone(true));
    if (kind.startsWith('fan_')) { const rotor = sources.get(kind.replace('fan_', 'rotor_'))?.clone(true); if (rotor) { rotor.position.set(0, .32, .015); root.add(rotor); } }
    if (kind.startsWith('pot_')) { const soil = sources.get('soil')?.clone(true); if (soil) { soil.position.y = .245; root.add(soil); } }
    root.traverse((obj) => { if (obj instanceof Mesh) { obj.castShadow = true; obj.receiveShadow = true; } });
    return root;
  }, [scene, kind]);
  return <primitive object={model} />;
}

function Normalize({ children }: { children: ReactNode }) {
  const content = useRef<Group>(null);
  useLayoutEffect(() => {
    const group = content.current;
    if (!group) return;
    group.position.set(0, 0, 0); group.scale.set(1, 1, 1); group.updateWorldMatrix(true, true);
    // Measure in this group's local frame so parent placement cannot change the result.
    const bounds = new Box3();
    group.traverse((object) => {
      if (!(object instanceof Mesh)) return;
      object.geometry.computeBoundingBox();
      if (object.geometry.boundingBox) {
        const transform = group.matrixWorld.clone().invert().multiply(object.matrixWorld);
        if (object instanceof InstancedMesh) {
          const matrix = transform.clone();
          for (let i = 0; i < object.count; i++) { object.getMatrixAt(i, matrix); bounds.union(object.geometry.boundingBox.clone().applyMatrix4(transform.clone().multiply(matrix))); }
        } else bounds.union(object.geometry.boundingBox.clone().applyMatrix4(transform));
      }
    });
    if (bounds.isEmpty()) return;
    const size = bounds.getSize(new Vector3()); const center = bounds.getCenter(new Vector3());
    group.scale.set(1 / Math.max(size.x, .001), 1 / Math.max(size.y, .001), 1 / Math.max(size.z, .001));
    group.position.set(-center.x / size.x, -bounds.min.y / size.y, -center.z / size.z);
  }, []);
  return <group ref={content}>{children}</group>;
}

function Asset({ kind, props, quality }: { kind: string; props: LayoutItem['props']; quality: TwinPerformanceProfile }) {
  if (kind === 'tower') return <HydroponicTower levels={Number(props.levels)} />;
  if (kind === 'reservoir') return <DwcReservoir quality={quality} level={Number(props.fill)} />;
  if (kind === 'plant') return <LettucePlant quality={quality} attention={false} occupied seed={0} />;
  if (kind === 'controller') return <Controller online />;
  if (kind === 'air_pump') return <AirPump running={false} />;
  return <ModuleModel kind={kind} />;
}

const GHOST = "#9be07f";
const ACCENT = "#f2d15b";
const unitBoxEdges = new EdgesGeometry(new BoxGeometry());
const noRaycast = () => null;

/** Translucent bounds at the committed transform: marks the object, and stays behind as its origin while handles are dragged. */
function SelectionGhost({ item, strength = 1 }: { item: LayoutItem; strength?: number }) {
  const [w, h, d] = footprint(item);
  return <group position={item.position}>
    <mesh position={[0, h / 2, 0]} scale={[w + .02, h + .02, d + .02]} raycast={noRaycast}><boxGeometry /><meshBasicMaterial color={GHOST} transparent opacity={.05 * strength} depthWrite={false} /></mesh>
    <lineSegments geometry={unitBoxEdges} position={[0, h / 2, 0]} scale={[w + .02, h + .02, d + .02]} raycast={noRaycast}><lineBasicMaterial color={GHOST} transparent opacity={.5 * strength} depthWrite={false} /></lineSegments>
    {strength === 1 && <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, .009, 0]} raycast={noRaycast}><planeGeometry args={[w + .06, d + .06]} /><meshBasicMaterial color={ACCENT} transparent opacity={.12} depthWrite={false} /></mesh>}
  </group>;
}

interface RadialAction { label: string; hint: string; icon: LucideIcon; run: () => void; active?: boolean; danger?: boolean }

/** Quick actions orbiting the selected object; mirrors the side panel for pointer and touch users. */
function RadialMenu({ item, editor }: { item: LayoutItem; editor: Editor }) {
  const actions: RadialAction[] = [
    { label: "Move with handles", hint: "Move · G", icon: Move, active: !editor.placing && editor.mode === "translate", run: () => { editor.setPlacing(false); editor.setMode("translate"); } },
    { label: "Resize with handles", hint: "Resize · S", icon: Maximize2, active: !editor.placing && editor.mode === "scale", run: () => { editor.setPlacing(false); editor.setMode("scale"); } },
    { label: "Rotate object", hint: "Rotate 90° · R", icon: RotateCw, run: () => editor.rotate(90) },
    { label: "Duplicate", hint: "Duplicate · Ctrl+D", icon: Copy, run: editor.duplicate },
    { label: "Delete", hint: "Delete · Del", icon: Trash2, danger: true, run: editor.remove },
    { label: "Click the floor to place", hint: "Place on floor", icon: Crosshair, active: editor.placing, run: () => editor.setPlacing(!editor.placing) },
  ];
  return <Html position={[0, footprint(item)[1], 0]} zIndexRange={[30, 10]} wrapperClass="gn-tent-radial-anchor">
    <div className="gn-tent-radial" role="toolbar" aria-label={`${itemName(item.kind)} quick actions`}>
      {actions.map(({ label, hint, icon: Icon, run, active, danger }, index) => <button key={label} className={danger ? "is-danger" : undefined} style={{ "--i": index } as CSSProperties} aria-label={label} aria-pressed={active} title={hint} onClick={run}><Icon size={15} strokeWidth={1.9} /></button>)}
      <button className="gn-tent-radial-center" aria-label="Deselect object" title="Deselect · Esc" onClick={() => editor.select(undefined)}><X size={14} strokeWidth={2} /></button>
    </div>
  </Html>;
}

/** Time of the last TransformControls press/release; the gizmo is not an R3F target, so its clicks fall through to the floor. */
let lastGizmoPointer = 0;
const markGizmoPointer = () => { lastGizmoPointer = performance.now(); };

function PlacedObject({ item, editor, quality }: { item: LayoutItem; editor: Editor; quality: TwinPerformanceProfile }) {
  const root = useRef<Group>(null!);
  const [hovered, setHovered] = useState(false);
  const selected = editor.selected === item.id;
  useEffect(() => { if (!hovered) return; document.body.style.cursor = "pointer"; return () => { document.body.style.cursor = ""; }; }, [hovered]);
  const release = () => {
    const group = root.current;
    if (!group) return;
    if (editor.mode === "scale") {
      // Handles scale world axes; map them back onto the item's unrotated axes.
      const [x, y, z] = group.scale.toArray();
      const factors = item.rotation % 180 === 0 ? [x, y, z] : [z, y, x];
      group.scale.set(1, 1, 1);
      const size = item.size.map((value, axis) => Math.max(.01, Math.round(value * Math.abs(factors[axis]) * 1000) / 1000)) as Vec3;
      if (size.some((value, axis) => value !== item.size[axis])) editor.commit((layout) => updateItem(layout, { ...item, size }));
      return;
    }
    const positioned = placeItem({ ...item, position: group.position.toArray() as Vec3 }, editor.layout.tent, editor.layout.grid);
    group.position.set(...positioned.position);
    if (positioned.position.some((value, axis) => value !== item.position[axis])) editor.commit((layout) => updateItem(layout, positioned));
  };
  const contents = <group ref={root} position={item.position} onClick={(event) => { event.stopPropagation(); editor.select(item.id); }} onPointerOver={(event) => { event.stopPropagation(); setHovered(true); }} onPointerOut={() => setHovered(false)}>
    <group rotation={[0, item.rotation * Math.PI / 180, 0]} scale={item.size}>
      <Suspense fallback={<mesh position={[0, .5, 0]}><boxGeometry /><meshStandardMaterial wireframe color="#93c67c" /></mesh>}><Normalize key={item.kind === 'tower' ? `levels-${item.props.levels}` : 'model'}><Asset kind={item.kind} props={item.props} quality={quality} /></Normalize></Suspense>
    </group>
    {selected && <RadialMenu item={item} editor={editor} />}
  </group>;
  return <>
    {contents}
    {selected ? <SelectionGhost item={item} /> : hovered && <SelectionGhost item={item} strength={.45} />}
    {selected && !editor.placing && <TransformControls object={root} mode={editor.mode} translationSnap={editor.layout.grid} size={.8} onMouseDown={markGizmoPointer} onMouseUp={() => { markGizmoPointer(); release(); }} />}
  </>;
}

const SPECTRUM_COLORS: Record<string, string> = { full: "#ffe6f0", veg: "#cfe0ff", bloom: "#ffc7b8", white: "#fff1dc" };
/** Scene lights are costly, so only the first few fixtures cast real light; the rest still count in the summary. */
const MAX_LED_LIGHTS = 4;

/** Light under an LED fixture, scaled by its rated watts and dimmer. */
function FixtureLight({ item }: { item: LayoutItem }) {
  const draw = Number(item.props.watts) * Number(item.props.output) / 100;
  if (draw <= 0) return null;
  return <pointLight position={[item.position[0], Math.max(.05, item.position[1] - .03), item.position[2]]} color={SPECTRUM_COLORS[String(item.props.spectrum)] ?? "#ffffff"} intensity={Math.min(60, draw * .06)} distance={0} decay={2} />;
}

/** Ground and boundary for an outdoor plot, replacing the tent enclosure. */
function OutdoorPlot({ width, height, depth }: { width: number; height: number; depth: number }) {
  const outline = useMemo(() => {
    const w = width / 2, d = depth / 2, y = .012;
    const geometry = new BufferGeometry();
    geometry.setAttribute('position', new Float32BufferAttribute([-w, y, -d, w, y, -d, w, y, -d, w, y, d, w, y, d, -w, y, d, -w, y, d, -w, y, -d], 3));
    return geometry;
  }, [width, depth]);
  useEffect(() => () => outline.dispose(), [outline]);
  const ground = Math.max(width, depth) * 3 + 6;
  return <>
    <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -.002, 0]} receiveShadow><planeGeometry args={[ground, ground]} /><meshStandardMaterial color="#27331f" roughness={1} /></mesh>
    <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, .001, 0]} receiveShadow><planeGeometry args={[width, depth]} /><meshStandardMaterial color="#3b2f22" roughness={1} /></mesh>
    <lineSegments geometry={outline}><lineBasicMaterial color="#f2d15b" transparent opacity={.7} /></lineSegments>
    {/* Faint posts mark the height limit at the corners. */}
    {[[-1, -1], [1, -1], [1, 1], [-1, 1]].map(([x, z]) => <mesh key={`${x}${z}`} position={[x * width / 2, height / 2, z * depth / 2]}><boxGeometry args={[.02, height, .02]} /><meshBasicMaterial color="#f2d15b" transparent opacity={.18} depthWrite={false} /></mesh>)}
  </>;
}

function SnapGrid({ width, depth, step }: { width: number; depth: number; step: number }) {
  const geometry = useMemo(() => {
    const vertices: number[] = [];
    for (let n = Math.ceil(-width / 2 / step); n <= Math.floor(width / 2 / step); n++) vertices.push(n * step, .006, -depth / 2, n * step, .006, depth / 2);
    for (let n = Math.ceil(-depth / 2 / step); n <= Math.floor(depth / 2 / step); n++) vertices.push(-width / 2, .006, n * step, width / 2, .006, n * step);
    const result = new BufferGeometry(); result.setAttribute('position', new Float32BufferAttribute(vertices, 3)); return result;
  }, [width, depth, step]);
  useEffect(() => () => geometry.dispose(), [geometry]);
  return <lineSegments geometry={geometry}><lineBasicMaterial color="#84a975" /></lineSegments>;
}

export function TentDesignerScene({ editor, quality }: { editor: Editor; quality: TwinPerformanceProfile }) {
  const { layout } = editor;
  const outdoor = layout.environment === "outdoor";
  const canvas = useThree((state) => state.gl.domElement);
  const camera = useThree((state) => state.camera);
  const fitted = useRef<{ environment: string; span: number } | undefined>(undefined);
  const span = Math.max(layout.tent[0], layout.tent[2], layout.tent[1] * .9);
  useEffect(() => {
    // Refit only when switching space type or when the space changes a lot, so small edits never jump the view.
    const last = fitted.current;
    if (last && last.environment === layout.environment && span <= last.span * 1.25 && span >= last.span * .5) return;
    const scale = Math.max(1, span / 2.4);
    if (last || scale > 1) camera.position.set(4 * scale, 3.5 * scale, 5 * scale);
    fitted.current = { environment: layout.environment, span };
  }, [camera, layout.environment, span]);
  useEffect(() => { canvas.dataset.tentLayout = JSON.stringify(layout); return () => { delete canvas.dataset.tentLayout; }; }, [canvas, layout]);
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (!editor.item || (event.target as HTMLElement | null)?.closest("input, select, textarea, [contenteditable='true']")) return;
      const key = event.key.toLowerCase(), modifier = event.ctrlKey || event.metaKey;
      if (key === "escape") editor.select(undefined);
      else if (key === "delete" || key === "backspace") editor.remove();
      else if (key === "d" && modifier) editor.duplicate();
      else if (modifier || event.altKey) return;
      else if (key === "r") editor.rotate(90);
      else if (key === "g") { editor.setPlacing(false); editor.setMode("translate"); }
      else if (key === "s") { editor.setPlacing(false); editor.setMode("scale"); }
      else return;
      event.preventDefault();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [editor]);
  return <>
    {outdoor ? <>
      <color attach="background" args={["#16222a"]} />
      <hemisphereLight args={["#cfe3ff", "#2b3a22", .9]} /><directionalLight position={[6, 10, 3]} intensity={3.4} color="#fff4de" castShadow={quality.shadows} />
      <Suspense fallback={null}><Environment files={`${import.meta.env.BASE_URL}textures/cc0/studio_small_09_1k.hdr`} environmentIntensity={.6} /></Suspense>
      <OutdoorPlot width={layout.tent[0]} height={layout.tent[1]} depth={layout.tent[2]} />
    </> : <>
      <color attach="background" args={["#142019"]} />
      <ambientLight intensity={.8} /><directionalLight position={[3, 6, 4]} intensity={3} castShadow={quality.shadows} />
      <Suspense fallback={null}><Environment files={`${import.meta.env.BASE_URL}textures/cc0/studio_small_09_1k.hdr`} environmentIntensity={.8} /><group position={[0, layout.tent[1] * .4725 / .9725, 0]} scale={[layout.tent[0] / .98, layout.tent[1] / .9725, layout.tent[2] / .98]}><GrowTent quality={quality} /></group></Suspense>
    </>}
    {layout.items.filter((item) => item.kind.startsWith('led_')).slice(0, MAX_LED_LIGHTS).map((item) => <FixtureLight key={item.id} item={item} />)}
    <SnapGrid width={layout.tent[0]} depth={layout.tent[2]} step={layout.grid} />
    <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, .015, 0]} onClick={(event) => {
      // Only a genuine click (not the end of a handle or orbit drag) clears the selection.
      if (!editor.placing) { if (event.delta <= 4 && performance.now() - lastGizmoPointer > 250) editor.select(undefined); return; }
      event.stopPropagation(); const item = layout.items.find((entry) => entry.id === editor.selected);
      if (item) editor.commit((current) => updateItem(current, { ...item, position: [event.point.x, item.position[1], event.point.z] }));
    }}><planeGeometry args={[layout.tent[0], layout.tent[2]]} /><meshBasicMaterial transparent opacity={0} depthWrite={false} /></mesh>
    {layout.items.map((item) => <PlacedObject key={item.id} item={item} editor={editor} quality={quality} />)}
    <OrbitControls makeDefault minDistance={.5} maxDistance={Math.max(30, Math.max(layout.tent[0], layout.tent[2]) * 2.5)} maxPolarAngle={Math.PI / 2.02} target={[0, layout.tent[1] / 2, 0]} />
  </>;
}
