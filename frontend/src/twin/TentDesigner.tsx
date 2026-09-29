import { useGLTF, TransformControls, OrbitControls, Environment } from "@react-three/drei";
import { useThree } from "@react-three/fiber";
import { Suspense, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Box3, BufferGeometry, Float32BufferAttribute, Group, InstancedMesh, Mesh, Object3D, Vector3 } from "three";
import { AirPump, Controller, DwcReservoir, GrowTent, LettucePlant } from "./models/GrowModels";
import { HydroponicTower } from "./models/HydroponicTower";
import type { TwinPerformanceProfile } from "./performance";
import { addItem, catalog, initialTentLayout, itemName, parseTentLayout, placeItem, resizeTent, updateItem, type LayoutItem, type TentLayout, type Vec3 } from "./tentLayout";
import "./tent-designer.css";

export function useTentDesigner(storageKey: string) {
  const [history, setHistory] = useState(() => {
    let layout = initialTentLayout();
    try { layout = parseTentLayout(localStorage.getItem(storageKey)); } catch { /* Invalid drafts do not block the editor. */ }
    return { past: [] as TentLayout[], present: layout, future: [] as TentLayout[] };
  });
  const [selected, select] = useState<string>();
  const [placing, setPlacing] = useState(false);
  const [message, setMessage] = useState("Layout saves in this browser.");
  const save = (next: typeof history) => {
    setHistory(next);
    try { localStorage.setItem(storageKey, JSON.stringify(next.present)); setMessage("Layout saved in this browser."); }
    catch { setMessage("Storage is unavailable. Keep this page open to retain your edits."); }
  };
  const commit = (change: (layout: TentLayout) => TentLayout) => {
    try { const next = change(history.present); save({ past: [...history.past.slice(-49), history.present], present: next, future: [] }); }
    catch (error) { setMessage(error instanceof Error ? error.message : "Could not apply layout change."); }
  };
  return {
    layout: history.present, selected, select, placing, setPlacing, message, commit,
    canUndo: history.past.length > 0, canRedo: history.future.length > 0,
    undo: () => { const previous = history.past.at(-1); if (previous) save({ past: history.past.slice(0, -1), present: previous, future: [history.present, ...history.future] }); },
    redo: () => { const next = history.future[0]; if (next) save({ past: [...history.past, history.present], present: next, future: history.future.slice(1) }); },
  };
}
type Editor = ReturnType<typeof useTentDesigner>;

function CmInput({ label, value, onChange, min = 1 }: { label: string; value: number; onChange: (n: number) => void; min?: number }) {
  return <label>{label}<input key={`${label}-${value}`} aria-label={label} type="number" step="1" min={min} max="1000" defaultValue={Math.round(value * 100)} onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }} onBlur={(event) => {
    const n = event.currentTarget.valueAsNumber;
    if (Number.isFinite(n) && n >= min && n <= 1000 && Math.abs(n / 100 - value) > 1e-6) onChange(n / 100);
    event.currentTarget.value = String(Math.round(value * 100));
  }} /><span>cm</span></label>;
}

export function TentDesignerControls({ editor }: { editor: Editor }) {
  const { layout, selected, select, commit } = editor;
  const [kind, setKind] = useState(catalog[0].kind);
  const item = layout.items.find((entry) => entry.id === selected);
  const dimensions = ['Width', 'Height', 'Depth'];
  const changeVector = (vector: Vec3, axis: number, value: number): Vec3 => vector.map((v, i) => i === axis ? value : v) as Vec3;
  return <div className="gn-tent-editor">
    <div className="gn-model-switch">
      <label>Add equipment<select aria-label="Add equipment type" value={kind} onChange={(event) => setKind(event.target.value)}>{catalog.map((entry) => <option key={entry.kind} value={entry.kind}>{entry.name}</option>)}</select></label>
      <button onClick={() => { const id = crypto.randomUUID(); commit((current) => addItem(current, kind, id)); select(id); }}>Add to tent</button>
      <label>Grid<select aria-label="Snap grid" value={layout.grid} onChange={(event) => commit((current) => resizeTent({ ...current, grid: Number(event.target.value) }, current.tent))}>{[.05, .1, .25].map((grid) => <option key={grid} value={grid}>{grid * 100} cm</option>)}</select></label>
      <button disabled={!editor.canUndo} onClick={editor.undo}>Undo</button><button disabled={!editor.canRedo} onClick={editor.redo}>Redo</button>
    </div>
    <details><summary>Tent dimensions · {layout.tent.map((v) => Math.round(v * 100)).join(' × ')} cm</summary><div className="gn-model-switch">{dimensions.map((label, axis) => <CmInput key={label} label={`Tent ${label.toLowerCase()}`} value={layout.tent[axis]} min={50} onChange={(value) => commit((current) => resizeTent(current, changeVector(current.tent, axis, value)))} />)}</div></details>
    <div className="gn-model-switch">
      <label>Selected object<select aria-label="Selected layout object" value={item?.id ?? ''} onChange={(event) => select(event.target.value || undefined)}><option value="">Select an object</option>{layout.items.map((entry, index) => <option key={entry.id} value={entry.id}>{index + 1}. {itemName(entry.kind)}</option>)}</select></label>
      {item && <><button aria-pressed={editor.placing} onClick={() => editor.setPlacing(!editor.placing)}>Place on grid</button><button onClick={() => commit((current) => updateItem(current, { ...item, rotation: (item.rotation + 90) % 360 }))}>Rotate 90°</button><button onClick={() => { commit((current) => ({ ...current, items: current.items.filter((entry) => entry.id !== item.id) })); select(undefined); }}>Remove object</button></>}
    </div>
    {item && <details key={item.id} open><summary>Object size & position · {item.rotation}°</summary><div className="gn-model-switch">{dimensions.map((label, axis) => <CmInput key={label} label={`Object ${label.toLowerCase()}`} value={item.size[axis]} onChange={(value) => commit((current) => updateItem(current, { ...item, size: changeVector(item.size, axis, value) }))} />)}{['X position', 'Elevation', 'Z position'].map((label, axis) => <CmInput key={label} label={label} value={item.position[axis]} min={axis === 1 ? 0 : -1000} onChange={(value) => commit((current) => updateItem(current, { ...item, position: changeVector(item.position, axis, value) }))} />)}</div></details>}
    <p role="status">{editor.message} {editor.placing && item ? 'Click or tap the floor grid to place the selected object.' : 'Select an object and drag its arrows to move. Size and position use centimetres.'}</p>
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

function Asset({ kind, quality }: { kind: string; quality: TwinPerformanceProfile }) {
  if (kind === 'tower') return <HydroponicTower />;
  if (kind === 'reservoir') return <DwcReservoir quality={quality} level={70} />;
  if (kind === 'plant') return <LettucePlant quality={quality} attention={false} occupied seed={0} />;
  if (kind === 'controller') return <Controller online />;
  if (kind === 'air_pump') return <AirPump running={false} />;
  return <ModuleModel kind={kind} />;
}

function PlacedObject({ item, editor, quality }: { item: LayoutItem; editor: Editor; quality: TwinPerformanceProfile }) {
  const root = useRef<Group>(null!);
  const selected = editor.selected === item.id;
  const contents = <group ref={root} position={item.position} onClick={(event) => { event.stopPropagation(); editor.select(item.id); }}>
    <group rotation={[0, item.rotation * Math.PI / 180, 0]} scale={item.size}>
      <Suspense fallback={<mesh position={[0, .5, 0]}><boxGeometry /><meshStandardMaterial wireframe color="#93c67c" /></mesh>}><Normalize><Asset kind={item.kind} quality={quality} /></Normalize></Suspense>
      {selected && <mesh position={[0, .5, 0]}><boxGeometry /><meshBasicMaterial color="#b5e784" wireframe depthTest={false} /></mesh>}
    </group>
  </group>;
  return <>{contents}{selected && !editor.placing && <TransformControls object={root} mode="translate" translationSnap={editor.layout.grid} size={.8} onMouseUp={() => { if (root.current) { const positioned = placeItem({ ...item, position: root.current.position.toArray() as Vec3 }, editor.layout.tent, editor.layout.grid); root.current.position.set(...positioned.position); editor.commit((layout) => updateItem(layout, positioned)); } }} />}</>;
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
  const canvas = useThree((state) => state.gl.domElement);
  useEffect(() => { canvas.dataset.tentLayout = JSON.stringify(layout); return () => { delete canvas.dataset.tentLayout; }; }, [canvas, layout]);
  return <>
    <color attach="background" args={["#142019"]} />
    <ambientLight intensity={.8} /><directionalLight position={[3, 6, 4]} intensity={3} castShadow={quality.shadows} />
    <Suspense fallback={null}><Environment files={`${import.meta.env.BASE_URL}textures/cc0/studio_small_09_1k.hdr`} environmentIntensity={.8} /><group position={[0, layout.tent[1] * .4725 / .9725, 0]} scale={[layout.tent[0] / .98, layout.tent[1] / .9725, layout.tent[2] / .98]}><GrowTent quality={quality} /></group></Suspense>
    <SnapGrid width={layout.tent[0]} depth={layout.tent[2]} step={layout.grid} />
    <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, .015, 0]} onClick={(event) => {
      if (!editor.placing) return;
      event.stopPropagation(); const item = layout.items.find((entry) => entry.id === editor.selected);
      if (item) editor.commit((current) => updateItem(current, { ...item, position: [event.point.x, item.position[1], event.point.z] }));
    }}><planeGeometry args={[layout.tent[0], layout.tent[2]]} /><meshBasicMaterial transparent opacity={0} depthWrite={false} /></mesh>
    {layout.items.map((item) => <PlacedObject key={item.id} item={item} editor={editor} quality={quality} />)}
    <OrbitControls makeDefault minDistance={.5} maxDistance={30} maxPolarAngle={Math.PI / 2.02} target={[0, layout.tent[1] / 2, 0]} />
  </>;
}
