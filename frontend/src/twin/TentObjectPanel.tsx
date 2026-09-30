import { Box, Copy, Crosshair, Maximize2, Move, RotateCcw, RotateCw, Trash2 } from "lucide-react";
import { fromMeters, lengthStep, toMeters, useLengthUnit } from "../lib/units";
import type { TentEditor } from "./tentEditor";
import { catalog, displayName, footprint, itemName, layoutSummary, spaceLimits, specsFor, updateItem, type LayoutItem, type PropSpec, type PropValue, type Vec3 } from "./tentLayout";
import "./tent-object-panel.css";

/** Edits a length stored in metres, shown in the user's preferred unit. Limits are in metres. */
export function LengthInput({ label, value, onChange, min = .01, max = 10 }: { label: string; value: number; onChange: (meters: number) => void; min?: number; max?: number }) {
  const unit = useLengthUnit();
  const shown = fromMeters(value, unit);
  return <label>{label}<input key={`${label}-${value}-${unit}`} aria-label={label} type="number" step={lengthStep[unit]} min={fromMeters(min, unit)} max={fromMeters(max, unit)} defaultValue={shown} onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }} onBlur={(event) => {
    const n = event.currentTarget.valueAsNumber;
    // Validate against the displayed (rounded) limits, then clamp so rounding never pushes metres out of range.
    if (Number.isFinite(n) && n !== shown && n >= fromMeters(min, unit) && n <= fromMeters(max, unit)) onChange(Math.min(max, Math.max(min, toMeters(n, unit))));
    event.currentTarget.value = String(shown);
  }} /><span>{unit}</span></label>;
}

const changeVector = (vector: Vec3, axis: number, value: number): Vec3 => vector.map((v, i) => i === axis ? value : v) as Vec3;

/** One equipment parameter (watts, diode count, spectrum…), committed when the field is left or changed. */
function PropField({ spec, value, onChange }: { spec: PropSpec; value: PropValue; onChange: (value: PropValue) => void }) {
  if (spec.type === "select") return <label>{spec.label}<select aria-label={spec.label} value={String(value)} onChange={(event) => onChange(event.target.value)}>{spec.options.map(([option, text]) => <option key={option} value={option}>{text}</option>)}</select><span /></label>;
  if (spec.type === "text") return <label className="is-text">{spec.label}<input key={`${spec.key}-${value}`} aria-label={spec.label} type="text" maxLength={spec.maxLength} placeholder={spec.key === "label" ? "Optional" : undefined} defaultValue={String(value)} onKeyDown={(event) => { if (event.key === "Enter") event.currentTarget.blur(); }} onBlur={(event) => { if (event.currentTarget.value !== value) onChange(event.currentTarget.value); }} /><span /></label>;
  return <label>{spec.label}<input key={`${spec.key}-${value}`} aria-label={spec.label} type="number" min={spec.min} max={spec.max} step={spec.step} defaultValue={Number(value)} onKeyDown={(event) => { if (event.key === "Enter") event.currentTarget.blur(); }} onBlur={(event) => {
    const n = event.currentTarget.valueAsNumber;
    if (Number.isFinite(n) && n !== value && n >= spec.min && n <= spec.max) onChange(n);
    else event.currentTarget.value = String(value);
  }} /><span>{spec.unit ?? ""}</span></label>;
}

/** Tower height follows its level count, so a level change rescales the object's height to match. */
function withProp(item: LayoutItem, key: string, value: PropValue): LayoutItem {
  const props = { ...item.props, [key]: value };
  if (item.kind === "tower" && key === "levels") {
    const height = (levels: number) => .5 + levels * .3;
    return { ...item, props, size: changeVector(item.size, 1, Math.round(item.size[1] * height(Number(value)) / height(Number(item.props.levels)) * 1000) / 1000) };
  }
  return { ...item, props };
}

const formatNumber = (value: number, digits = 0) => value.toLocaleString(undefined, { maximumFractionDigits: digits });

/** With nothing selected: what is in the space, and the power budget someone would need to replicate it. */
function LayoutOverview({ editor }: { editor: TentEditor }) {
  const unit = useLengthUnit();
  const { layout } = editor;
  const summary = layoutSummary(layout);
  const where = layout.environment === "tent" ? "Grow tent" : "Outdoor plot";
  return <>
    <div className="gn-inspector-head"><span>{where}</span><h2>{layout.name || "Untitled layout"}</h2><code>{layout.tent.map((v) => fromMeters(v, unit)).join(" × ")} {unit} · {formatNumber(summary.area, 2)} m²</code></div>
    <section>
      <h3>Equipment · {layout.items.length}</h3>
      {summary.counts.length ? <dl className="gn-object-summary">{summary.counts.map(({ name, count }) => <div key={name}><dt>{name}</dt><dd>× {count}</dd></div>)}</dl> : <p className="gn-object-keys">Nothing placed yet. Add equipment above, or import a layout file.</p>}
    </section>
    <section>
      <h3>Power budget</h3>
      <dl className="gn-object-summary">
        <div><dt>LED fixtures</dt><dd>{summary.leds}</dd></div>
        <div><dt>Lighting at current dimming</dt><dd>{formatNumber(summary.lightWatts)} W</dd></div>
        <div><dt>Light density</dt><dd>{formatNumber(summary.lightDensity)} W/m²</dd></div>
        <div><dt>Lighting energy</dt><dd>{formatNumber(summary.lightKwhPerDay, 2)} kWh/day</dd></div>
        <div><dt>Total rated draw</dt><dd>{formatNumber(summary.totalWatts)} W</dd></div>
      </dl>
    </section>
    <div className="gn-inspector-empty is-compact"><Box /><p>Select an object in the view or from “Selected object” to edit its position, size and specifications.</p></div>
  </>;
}

/** Inspector content for the tent layout view: everything about the selected layout object. */
export function TentObjectPanel({ editor }: { editor: TentEditor }) {
  const unit = useLengthUnit();
  const { item, commit } = editor;
  if (!item) return <LayoutOverview editor={editor} />;
  const size = footprint(item);
  const isDefaultSize = catalog.find((entry) => entry.kind === item.kind)?.size.every((v, axis) => Math.abs(v - item.size[axis]) < 1e-6);
  return <div className="gn-object-panel">
    <div className="gn-inspector-head"><span>{itemName(item.kind)}</span><h2>{displayName(item)}</h2><code>{size.map((v) => fromMeters(v, unit)).join(" × ")} {unit} · {item.rotation}°</code></div>
    <section>
      <h3>Specifications</h3>
      <div className="gn-object-fields">{specsFor(item.kind).map((spec) => <PropField key={spec.key} spec={spec} value={item.props[spec.key] ?? spec.default} onChange={(value) => commit((current) => updateItem(current, withProp(item, spec.key, value)))} />)}</div>
    </section>
    <section>
      <h3>Handles</h3>
      <div className="gn-object-modes" role="group" aria-label="Transform handles">
        <button aria-pressed={!editor.placing && editor.mode === "translate"} onClick={() => { editor.setPlacing(false); editor.setMode("translate"); }}><Move size={15} />Move</button>
        <button aria-pressed={!editor.placing && editor.mode === "scale"} onClick={() => { editor.setPlacing(false); editor.setMode("scale"); }}><Maximize2 size={15} />Resize</button>
        <button aria-pressed={editor.placing} onClick={() => editor.setPlacing(!editor.placing)}><Crosshair size={15} />Place on grid</button>
      </div>
    </section>
    <section>
      <h3>Position</h3>
      <div className="gn-object-fields">{['X position', 'Elevation', 'Z position'].map((label, axis) => <LengthInput key={label} label={label} value={item.position[axis]} min={axis === 1 ? 0 : -spaceLimits.outdoor.max / 2} max={axis === 1 ? spaceLimits.outdoor.maxHeight : spaceLimits.outdoor.max / 2} onChange={(value) => commit((current) => updateItem(current, { ...item, position: changeVector(item.position, axis, value) }))} />)}</div>
    </section>
    <section>
      <h3>Size</h3>
      <div className="gn-object-fields">{['Width', 'Height', 'Depth'].map((label, axis) => <LengthInput key={label} label={`Object ${label.toLowerCase()}`} value={item.size[axis]} max={spaceLimits.outdoor.max} onChange={(value) => commit((current) => updateItem(current, { ...item, size: changeVector(item.size, axis, value) }))} />)}</div>
      <button className="gn-object-link" disabled={isDefaultSize} onClick={editor.resetSize}>Reset to catalog size</button>
    </section>
    <section>
      <h3>Rotation · {item.rotation}°</h3>
      <div className="gn-object-modes">
        <button onClick={() => editor.rotate(-90)} aria-label="Rotate -90°"><RotateCcw size={15} />−90°</button>
        <button onClick={() => editor.rotate(90)}><RotateCw size={15} />Rotate 90°</button>
      </div>
    </section>
    <section>
      <h3>Object</h3>
      <div className="gn-object-actions">
        <button className="gn-button" onClick={editor.duplicate}><Copy size={15} />Duplicate object</button>
        <button className="gn-button danger" onClick={editor.remove}><Trash2 size={15} />Remove object</button>
      </div>
      <p className="gn-object-keys">Shortcuts: G move · S resize · R rotate · Ctrl+D duplicate · Del remove · Esc deselect</p>
    </section>
  </div>;
}
