import {
  AlertCircle, ArrowDownRight, ArrowUpRight, Check, CircleGauge, CloudOff, Download,
  Droplets, FileJson, Gauge, Leaf, Lightbulb, Pause, Play, Plus, Power, RefreshCw,
  RotateCcw, Search, ShieldCheck, Thermometer, Upload, Waves, Wind,
} from "lucide-react";
import { lazy, Suspense, useMemo, useRef, useState, type ChangeEvent } from "react";
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { AutomationRule, Device, EntityType, FarmData, GrowNerveArchive, RuntimeMode } from "../domain/model";
import { Card, formatDateTime, formatRelative, PageHeader, Status } from "../components/Status";
import type { Selection } from "../twin/DigitalTwin";

const DigitalTwin = lazy(() => import("../twin/DigitalTwin").then((module) => ({ default: module.DigitalTwin })));

export interface ScreenActions {
  refreshSimulator: () => void;
  command: (channelId: string, value: number | boolean, reason: string) => void;
  select: (selection?: Selection) => void;
  acknowledgeAlert: (alertId: string) => void;
  resolveAlert: (alertId: string) => void;
  addObservation: (notes: string, severity: "info" | "warning" | "critical") => void;
  addInventoryAdjustment: (itemId: string, quantity: number, reason: "purchase" | "use" | "correction" | "waste") => void;
  toggleRule: (ruleId: string) => void;
  setDeviceOnline: (deviceId: string, online: boolean) => void;
  exportArchive: (includeMedia: boolean) => void;
  importArchive: (archive: unknown) => Promise<void>;
  reset: () => void;
  loadPilot: () => void;
}

const latest = (data: FarmData, channelId: string) => [...data.measurements].reverse().find((measurement) => measurement.channel_id === channelId);
const latestByKey = (data: FarmData, key: string) => {
  const channel = data.channels.find((entry) => entry.key === key);
  return channel ? latest(data, channel.id) : undefined;
};
const entityName = (data: FarmData, type: EntityType, id: string) => {
  const sources: Partial<Record<EntityType, Array<{ id: string; name?: string; code?: string }>>> = { facility: data.facilities, zone: data.zones, reservoir: data.reservoirs, grow_cycle: data.grow_cycles, plant_position: data.plant_positions, device: data.devices, channel: data.channels, inventory_item: data.inventory_items };
  const entity = sources[type]?.find((entry) => entry.id === id);
  return entity?.name ?? entity?.code ?? "Unknown entity";
};
const inventoryBalance = (data: FarmData, itemId: string) => data.inventory_adjustments.filter((entry) => entry.item_id === itemId).reduce((sum, entry) => sum + entry.quantity, 0);

function Metric({ icon, label, value, unit, state, detail }: { icon: React.ReactNode; label: string; value: string | number; unit?: string; state: "ok" | "warning" | "critical" | "neutral"; detail: string }) {
  return <div className={`gn-metric gn-metric-${state}`}><div className="gn-metric-icon">{icon}</div><div><span>{label}</span><strong>{value}<small>{unit}</small></strong><p>{detail}</p></div></div>;
}

export function OverviewScreen({ data, actions, runtimeMode }: { data: FarmData; actions: ScreenActions; runtimeMode: RuntimeMode }) {
  const grow = data.grow_cycles.find((entry) => entry.status === "active"), reservoir = data.reservoirs[0];
  const temperature = latestByKey(data, "air.temperature"), humidity = latestByKey(data, "air.humidity"), waterTemperature = latestByKey(data, "water.temperature"), waterLevel = latestByKey(data, "water.level");
  const chart = useMemo(() => data.measurements.filter((entry) => entry.channel_id === data.channels.find((channel) => channel.key === "air.temperature")?.id).slice(-24).map((entry) => ({ time: new Date(entry.observed_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }), value: entry.value })), [data]);
  const browser = runtimeMode === "browser";
  return <>
    <PageHeader eyebrow="Live farm read model" title="Farm overview" description="The active grow, current conditions, exceptions, and equipment state in one operational view." actions={browser ? <button className="gn-button" onClick={actions.refreshSimulator}><RefreshCw size={16} /> Refresh simulator</button> : undefined} />
    {browser ? <div className="gn-callout"><div><ShieldCheck /><span><strong>Browser simulator is active.</strong> Commands and acknowledgements on this screen are local simulations, never claims about physical equipment.</span></div><Status tone="simulated">Simulated control</Status></div> : <div className="gn-callout"><div><ShieldCheck /><span><strong>Server runtime is active.</strong> Telemetry and command status come from registered devices through the server control plane.</span></div><Status tone="ok">Server control</Status></div>}
    <div className="gn-metric-grid">
      <Metric icon={<Thermometer />} label="Air temperature" value={temperature?.value ?? "—"} unit=" °C" state="ok" detail="Target 18–24 °C" />
      <Metric icon={<Droplets />} label="Relative humidity" value={humidity?.value ?? "—"} unit=" %" state="ok" detail="Target 55–75 %RH" />
      <Metric icon={<Waves />} label="Water temperature" value={waterTemperature?.value ?? "—"} unit=" °C" state={Number(waterTemperature?.value) > 22 ? "warning" : "ok"} detail="Target ≤22 °C" />
      <Metric icon={<Gauge />} label="Reservoir level" value={waterLevel?.value ?? reservoir?.level_percent ?? "—"} unit=" %" state="ok" detail={`${reservoir?.working_volume_l ?? 0} L estimated`} />
    </div>
    <div className="gn-dashboard-grid">
      <Card title="Active grow" subtitle={grow?.name ?? "No active cycle"} action={<Status tone="ok">{grow?.status ?? "empty"}</Status>}>
        {grow && <div className="gn-grow-summary"><div className="gn-grow-hero"><div className="gn-plant-mark"><Leaf /></div><div><span>Current stage</span><strong>{grow.stage_key}</strong><p>Day {Math.max(1, Math.floor((Date.now() - new Date(grow.actual_start!).getTime()) / 86400000) + 1)} · {grow.plant_count} plant positions</p></div></div><div className="gn-progress"><span style={{ width: "58%" }} /></div><div className="gn-grow-foot"><span>Seedling</span><b>Vegetative</b><span>Harvest</span></div></div>}
      </Card>
      <Card title="Active alerts" subtitle={`${data.alerts.filter((entry) => entry.status !== "resolved").length} condition requires attention`} action={<AlertCircle size={18} />}>
        <div className="gn-list">{data.alerts.filter((entry) => entry.status !== "resolved").map((alert) => <button className="gn-list-row" key={alert.id} onClick={() => { actions.select({ type: alert.entity_type, id: alert.entity_id }); }}><span className="gn-severity warning" /><div><strong>{alert.title}</strong><p>{alert.detail}</p></div><time>{formatRelative(alert.opened_at)}</time></button>)}{data.alerts.every((entry) => entry.status === "resolved") && <Empty message="No active alerts" />}</div>
      </Card>
      <Card title="Air temperature · 24 hours" subtitle="Telemetry remains separate from farm history" className="gn-chart-card">
        <div className="gn-chart"><ResponsiveContainer width="100%" height="100%"><LineChart data={chart}><CartesianGrid vertical={false} stroke="#dde3dc" strokeDasharray="3 3" /><XAxis dataKey="time" tick={{ fontSize: 10 }} interval={5} /><YAxis domain={[16, 27]} tick={{ fontSize: 10 }} width={32} /><Tooltip /><Line dataKey="value" type="monotone" stroke="#b66a12" strokeWidth={2} dot={false} /></LineChart></ResponsiveContainer></div>
      </Card>
      <Card title="Equipment" subtitle={browser ? "Acknowledged simulator state only" : "Acknowledged device state"}>
        <div className="gn-equipment-grid">{data.devices.filter((entry) => entry.type !== "controller").map((device) => <div key={device.id} className="gn-equipment"><div>{device.type === "light" ? <Lightbulb /> : device.type === "fan" ? <Wind /> : <Waves />}</div><span>{device.name}</span><strong>{device.output_percent ?? (device.state ? 100 : 0)}%</strong><Status tone={device.online ? "ok" : "critical"}>{device.online ? device.state ? "Running" : "Stopped" : "Offline"}</Status></div>)}</div>
      </Card>
      <Card title="Recent farm history" subtitle="Meaningful actions, not raw telemetry">
        <Timeline data={data} limit={4} />
      </Card>
    </div>
  </>;
}

function Empty({ message }: { message: string }) { return <div className="gn-empty"><Check /><p>{message}</p></div>; }

function Timeline({ data, limit }: { data: FarmData; limit?: number }) {
  return <div className="gn-timeline">{[...data.events].sort((a, b) => b.occurred_at.localeCompare(a.occurred_at)).slice(0, limit).map((event) => <article key={event.id}><span /><div><strong>{event.summary}</strong><p>{event.type.replaceAll(".", " · ")} · {event.actor}</p></div><time>{formatDateTime(event.occurred_at)}</time></article>)}</div>;
}

export function FarmScreen({ data, actions }: { data: FarmData; actions: ScreenActions }) {
  const [query, setQuery] = useState("");
  const searchable = [
    ...data.facilities.map((entry) => ({ type: "facility" as const, id: entry.id, name: entry.name, meta: "Facility" })),
    ...data.zones.map((entry) => ({ type: "zone" as const, id: entry.id, name: entry.name, meta: entry.type })),
    ...data.reservoirs.map((entry) => ({ type: "reservoir" as const, id: entry.id, name: entry.name, meta: "Reservoir" })),
    ...data.devices.map((entry) => ({ type: "device" as const, id: entry.id, name: entry.name, meta: entry.type })),
    ...data.plant_positions.map((entry) => ({ type: "plant_position" as const, id: entry.id, name: entry.code, meta: "Plant position" })),
  ].filter((entry) => `${entry.name} ${entry.meta}`.toLowerCase().includes(query.toLowerCase()));
  return <><PageHeader eyebrow="Facility topology" title="Farm" description="Durable facilities, hierarchical zones, reservoirs, devices, and plant positions share the same identities as the digital twin." />
    <div className="gn-search-large"><Search /><input aria-label="Search farm" placeholder="Search facilities, zones, devices, channels…" value={query} onChange={(event) => setQuery(event.target.value)} /></div>
    <div className="gn-two-columns"><Card title="Farm hierarchy" subtitle={`${data.facilities.length} facility · ${data.zones.length} zones`}><div className="gn-tree">{data.facilities.map((facility) => <div key={facility.id}><button onClick={() => actions.select({ type: "facility", id: facility.id })}><span className="gn-tree-icon"><Leaf /></span><strong>{facility.name}</strong><Status tone="ok">Operational</Status></button>{data.zones.filter((zone) => !zone.parent_zone_id).map((zone) => <div className="gn-tree-child" key={zone.id}><button onClick={() => actions.select({ type: "zone", id: zone.id })}><span className="gn-tree-icon muted"><CircleGauge /></span><strong>{zone.name}</strong></button>{data.zones.filter((child) => child.parent_zone_id === zone.id).map((child) => <div className="gn-tree-child" key={child.id}><button onClick={() => actions.select({ type: "zone", id: child.id })}><span className="gn-tree-icon amber"><Lightbulb /></span><strong>{child.name}</strong><span>{data.plant_positions.filter((position) => position.zone_id === child.id).length} positions</span></button></div>)}</div>)}</div>)}</div></Card>
      <Card title="Entities" subtitle={`${searchable.length} matching records`}><div className="gn-entity-list">{searchable.map((entry) => <button key={`${entry.type}:${entry.id}`} onClick={() => actions.select({ type: entry.type, id: entry.id })}><span>{entry.name.slice(0, 2).toUpperCase()}</span><div><strong>{entry.name}</strong><p>{entry.meta.replaceAll("_", " ")}</p></div><ArrowUpRight /></button>)}</div></Card></div>
  </>;
}

export function GrowCyclesScreen({ data, actions }: { data: FarmData; actions: ScreenActions }) {
  const [notes, setNotes] = useState("");
  const grow = data.grow_cycles[0], stages = data.recipe_stages.filter((entry) => entry.recipe_version_id === grow?.recipe_version_id).sort((a, b) => a.sort_order - b.sort_order);
  return <><PageHeader eyebrow="Crop execution" title="Grow cycles" description="Stage-aware recipe targets, positions, observations, inputs, alerts, and harvest history remain attached to one production record." />
    <Card title={grow?.name} subtitle={`${data.crops.find((entry) => entry.id === grow?.crop_id)?.name} · ${data.varieties.find((entry) => entry.id === grow?.variety_id)?.name}`} action={<Status tone="ok">{grow?.status}</Status>}>
      <div className="gn-stage-track">{stages.map((stage, index) => <div key={stage.id} className={stage.key === grow?.stage_key ? "current" : index < stages.findIndex((entry) => entry.key === grow?.stage_key) ? "done" : ""}><span>{index + 1}</span><div><strong>{stage.name}</strong><p>{stage.guidance_days ? `${stage.guidance_days} day guidance` : "Manual transition"}</p></div></div>)}</div>
      <div className="gn-cycle-stats"><div><span>Started</span><strong>{grow?.actual_start?.slice(0, 10)}</strong></div><div><span>Plant positions</span><strong>{grow?.plant_count}</strong></div><div><span>Recipe version</span><strong>v{data.recipe_versions.find((entry) => entry.id === grow?.recipe_version_id)?.version}</strong></div><div><span>Open observations</span><strong>{data.observations.length}</strong></div></div>
    </Card>
    <div className="gn-two-columns grow"><Card title="Current stage targets" subtitle="Demo values require agronomic review"><table className="gn-table"><thead><tr><th>Target</th><th>Range</th><th>Latest</th><th>Status</th></tr></thead><tbody>{data.setpoints.filter((entry) => entry.stage_id === stages.find((stage) => stage.key === grow?.stage_key)?.id).map((setpoint) => { const value = latestByKey(data, setpoint.channel_key); const outside = value && (setpoint.minimum !== undefined && value.value < setpoint.minimum || setpoint.maximum !== undefined && value.value > setpoint.maximum); return <tr key={setpoint.id}><td>{setpoint.channel_key.replaceAll(".", " ")}</td><td>{setpoint.minimum ?? "—"}–{setpoint.maximum ?? "—"} {setpoint.unit}</td><td>{value?.value ?? "—"} {setpoint.unit}</td><td><Status tone={outside ? "warning" : "ok"}>{outside ? "Outside" : "In range"}</Status></td></tr>; })}</tbody></table></Card>
      <Card title="Record observation" subtitle="Attach a meaningful crop note to the timeline"><form className="gn-form" onSubmit={(event) => { event.preventDefault(); if (notes.trim()) { actions.addObservation(notes.trim(), "info"); setNotes(""); } }}><label>Observation notes<textarea value={notes} onChange={(event) => setNotes(event.target.value)} placeholder="What changed or needs attention?" /></label><div className="gn-form-actions"><button className="gn-button primary" type="submit"><Plus size={16} /> Add observation</button></div></form>{data.observations.slice(-2).map((observation) => <div className="gn-note" key={observation.id}><Status tone={observation.severity === "critical" ? "critical" : observation.severity === "warning" ? "warning" : "info"}>{observation.category}</Status><p>{observation.notes}</p><time>{formatDateTime(observation.observed_at)}</time></div>)}</Card></div>
  </>;
}

export function TwinScreen({ data, selection, actions, runtimeMode }: { data: FarmData; selection?: Selection; actions: ScreenActions; runtimeMode: RuntimeMode }) {
  const selectedName = selection ? entityName(data, selection.type, selection.id) : "Select an object";
  const activeAlerts = selection ? data.alerts.filter((entry) => entry.entity_type === selection.type && entry.entity_id === selection.id && entry.status !== "resolved") : [];
  return <><PageHeader eyebrow="Operational digital twin" title="3D Twin" description="Select real domain entities in the pilot tent. HTML inspection and actions remain accessible outside the renderer." actions={<><Status tone={runtimeMode === "browser" ? "simulated" : "ok"}>{runtimeMode === "browser" ? "Simulated live state" : "Server live state"}</Status><Status tone={typeof navigator !== "undefined" && "gpu" in navigator ? "ok" : "neutral"}>{typeof navigator !== "undefined" && "gpu" in navigator ? "WebGPU available" : "Fallback renderer"}</Status></>} />
    <div className="gn-twin-layout"><Card className="gn-twin-card"><Suspense fallback={<div className="gn-loading"><p>Opening digital twin…</p></div>}><DigitalTwin data={data} selection={selection} onSelect={actions.select} onAction={(action) => { if (action === "Set output" && selection?.type === "device") { const channel = data.channels.find((entry) => entry.entity_id === selection.id && entry.kind === "command"); if (channel) actions.command(channel.id, 55, "3D radial action"); } }} /></Suspense></Card>
      <aside className="gn-inspector"><div className="gn-inspector-head"><span>{selection?.type.replaceAll("_", " ") ?? "Entity inspector"}</span><h2>{selectedName}</h2>{selection && <code>{selection.id.slice(0, 18)}…</code>}</div>{!selection ? <div className="gn-inspector-empty"><CircleGauge /><p>Select the reservoir, light, fan, or a plant position to inspect the same durable entity used by 2D views.</p></div> : <><section><h3>Current status</h3><Status tone={activeAlerts.length ? "warning" : "ok"}>{activeAlerts.length ? `${activeAlerts.length} active alert` : "Normal"}</Status></section>{selection.type === "plant_position" && <section><h3>Available action</h3><div className="gn-action-stack"><button className="gn-button primary" onClick={() => actions.addObservation(`Visual inspection of ${selectedName}`, "info")}>Record observation</button></div></section>}{activeAlerts.map((alert) => <section className="gn-inspector-alert" key={alert.id}><Status tone="warning">{alert.severity}</Status><strong>{alert.title}</strong><p>{alert.detail}</p></section>)}</>}</aside>
    </div></>;
}

export function AlertsScreen({ data, actions }: { data: FarmData; actions: ScreenActions }) {
  const [filter, setFilter] = useState<"active" | "all">("active");
  const alerts = data.alerts.filter((entry) => filter === "all" || entry.status !== "resolved");
  return <><PageHeader eyebrow="Exceptions requiring attention" title="Alerts" description="Target-aware conditions are deduplicated and move through an explicit open, acknowledged, and resolved lifecycle." actions={<div className="gn-segment"><button className={filter === "active" ? "active" : ""} onClick={() => setFilter("active")}>Active</button><button className={filter === "all" ? "active" : ""} onClick={() => setFilter("all")}>All</button></div>} />
    <Card><div className="gn-alert-list">{alerts.map((alert) => <article key={alert.id}><div className={`gn-alert-icon ${alert.severity}`}><AlertCircle /></div><div className="gn-alert-content"><div><Status tone={alert.severity === "critical" ? "critical" : "warning"}>{alert.severity}</Status><Status tone={alert.status === "resolved" ? "ok" : "neutral"}>{alert.status}</Status></div><h2>{alert.title}</h2><p>{alert.detail}</p><span>{entityName(data, alert.entity_type, alert.entity_id)} · Opened {formatDateTime(alert.opened_at)}</span></div><div className="gn-alert-actions"><button className="gn-button" onClick={() => actions.select({ type: alert.entity_type, id: alert.entity_id })}>Locate entity</button>{alert.status === "open" && <button className="gn-button primary" onClick={() => actions.acknowledgeAlert(alert.id)}>Acknowledge</button>}{alert.status !== "resolved" && <button className="gn-button" onClick={() => actions.resolveAlert(alert.id)}>Resolve</button>}</div></article>)}{alerts.length === 0 && <Empty message="No active alerts" />}</div></Card>
  </>;
}

export function HistoryScreen({ data }: { data: FarmData }) {
  return <><PageHeader eyebrow="Immutable operational record" title="History" description="Farm events explain meaningful actions. High-volume measurements stay in bounded telemetry views and can be overlaid without becoming events." />
    <div className="gn-history-layout"><Card title="Farm event timeline" subtitle={`${data.events.length} durable events`}><Timeline data={data} /></Card><Card title="Event quantities" subtitle="Dimension-aware values attached to actions"><table className="gn-table"><thead><tr><th>Material</th><th>Quantity</th><th>Event</th></tr></thead><tbody>{data.event_quantities.map((quantity) => <tr key={quantity.id}><td>{quantity.material ?? "—"}</td><td>{quantity.value} {quantity.unit}</td><td>{data.events.find((entry) => entry.id === quantity.event_id)?.summary}</td></tr>)}</tbody></table></Card></div>
  </>;
}

export function InventoryScreen({ data, actions }: { data: FarmData; actions: ScreenActions }) {
  const [itemId, setItemId] = useState(data.inventory_items[0]?.id ?? ""), [quantity, setQuantity] = useState(0);
  return <><PageHeader eyebrow="Append-only material ledger" title="Inventory" description="Balances are derived from accepted adjustments so nutrient, seed, and consumable history never changes silently." />
    <div className="gn-metric-grid inventory">{data.inventory_items.map((item) => { const balance = inventoryBalance(data, item.id), low = balance <= item.reorder_level; return <Metric key={item.id} icon={low ? <ArrowDownRight /> : <ArrowUpRight />} label={item.name} value={balance} unit={` ${item.unit}`} state={low ? "warning" : "ok"} detail={low ? `Reorder at ${item.reorder_level}` : "Stock level healthy"} />; })}</div>
    <div className="gn-two-columns"><Card title="Record adjustment" subtitle="Positive purchases, negative use or waste"><form className="gn-form" onSubmit={(event) => { event.preventDefault(); if (itemId && quantity !== 0) { actions.addInventoryAdjustment(itemId, quantity, quantity > 0 ? "purchase" : "use"); setQuantity(0); } }}><label>Inventory item<select value={itemId} onChange={(event) => setItemId(event.target.value)}>{data.inventory_items.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label><label>Quantity<input type="number" step="any" value={quantity} onChange={(event) => setQuantity(Number(event.target.value))} /></label><button className="gn-button primary" type="submit"><Plus size={16} /> Record adjustment</button></form></Card><Card title="Adjustment history"><table className="gn-table"><thead><tr><th>Item</th><th>Reason</th><th>Quantity</th><th>Date</th></tr></thead><tbody>{[...data.inventory_adjustments].reverse().map((adjustment) => <tr key={adjustment.id}><td>{data.inventory_items.find((entry) => entry.id === adjustment.item_id)?.name}</td><td>{adjustment.reason}</td><td className={adjustment.quantity < 0 ? "negative" : "positive"}>{adjustment.quantity > 0 ? "+" : ""}{adjustment.quantity} {adjustment.unit}</td><td>{formatDateTime(adjustment.occurred_at)}</td></tr>)}</tbody></table></Card></div>
  </>;
}

export function AutomationScreen({ data, actions, runtimeMode }: { data: FarmData; actions: ScreenActions; runtimeMode: RuntimeMode }) {
  const browser = runtimeMode === "browser";
  return <><PageHeader eyebrow="Bounded and auditable rules" title="Automation" description={browser ? "Rules are available for observe and simulation workflows in the browser runtime." : "Persistent schedules and bounded commands execute through the server and edge control path."} />
    {browser ? <div className="gn-callout warning"><div><CloudOff /><span><strong>Unattended hardware execution is unavailable.</strong> Browsers may suspend timers; persistent physical schedules belong on the Go server and ESP32 edge.</span></div></div> : <div className="gn-callout"><div><ShieldCheck /><span><strong>Edge-resilient control is active.</strong> Essential schedules are delivered to controllers and continue without the server once adopted.</span></div></div>}
    <div className="gn-rule-grid">{data.automation_rules.map((rule: AutomationRule) => <Card key={rule.id} title={rule.name} subtitle={`Last evaluated ${formatRelative(rule.last_evaluated_at)}`} action={<button className={`gn-toggle ${rule.enabled ? "on" : ""}`} onClick={() => actions.toggleRule(rule.id)} aria-label={`${rule.enabled ? "Disable" : "Enable"} ${rule.name}`}><span /></button>}><div className="gn-rule-body"><Status tone={rule.mode === "simulate" ? "simulated" : "info"}>{rule.mode}</Status><dl><div><dt>Trigger</dt><dd>{rule.trigger}</dd></div><div><dt>Action</dt><dd>{rule.action}</dd></div><div><dt>Cooldown</dt><dd>{rule.cooldown_minutes ? `${rule.cooldown_minutes} minutes` : "None"}</dd></div></dl></div></Card>)}</div>
  </>;
}

function DeviceCard({ device, data, actions, runtimeMode }: { device: Device; data: FarmData; actions: ScreenActions; runtimeMode: RuntimeMode }) {
  const [output, setOutput] = useState(device.output_percent ?? 0);
  const channel = data.channels.find((entry) => entry.device_id === device.id && entry.kind === "command");
  const browser = runtimeMode === "browser";
  return <Card title={device.name} subtitle={`${device.type.replaceAll("_", " ")} · ${device.firmware_version}`} action={<Status tone={device.online ? device.simulated ? "simulated" : "ok" : "critical"}>{device.online ? device.simulated ? "Simulated" : "Online" : "Offline"}</Status>}>
    <div className="gn-device-health"><div><span>Heartbeat</span><strong>{formatRelative(device.last_heartbeat)}</strong></div><div><span>Config</span><strong>{device.active_config_version}</strong></div><div><span>Output</span><strong>{device.output_percent ?? 0}%</strong></div></div>
    {channel && <div className="gn-device-control">{channel.value_type === "boolean" ? <><button className="gn-button" onClick={() => actions.command(channel.id, !device.state, browser ? "manual browser control" : "manual operator control")} disabled={!device.online}>{device.state ? <Pause size={15} /> : <Play size={15} />}{device.state ? "Turn off" : "Turn on"}</button></> : <><input aria-label={`${device.name} output`} type="range" min={channel.safe_minimum} max={channel.safe_maximum} value={output} onChange={(event) => setOutput(Number(event.target.value))} /><strong>{output}%</strong><button className="gn-button primary" onClick={() => actions.command(channel.id, output, browser ? "manual browser control" : "manual operator control")} disabled={!device.online}>Apply</button></>}</div>}
    {browser && <button className="gn-text-button" onClick={() => actions.setDeviceOnline(device.id, !device.online)}>{device.online ? "Simulate offline fault" : "Reconnect simulator"}</button>}
  </Card>;
}

export function DevicesScreen({ data, actions, runtimeMode }: { data: FarmData; actions: ScreenActions; runtimeMode: RuntimeMode }) {
  const browser = runtimeMode === "browser";
  return <><PageHeader eyebrow="Device and logical channel registry" title="Devices" description={browser ? "Stable application channels survive physical replacement. Command success appears only after the simulator acknowledgement path completes." : "Stable application channels survive physical replacement. Command status reflects acknowledgements from registered controllers."} actions={browser ? <button className="gn-button" onClick={actions.refreshSimulator}><RefreshCw size={16} /> Generate telemetry</button> : undefined} />
    <div className="gn-device-grid">{data.devices.map((device) => <DeviceCard key={device.id} device={device} data={data} actions={actions} runtimeMode={runtimeMode} />)}</div>
    <Card title="Command history" subtitle="Durable idempotent intents and outcomes"><table className="gn-table"><thead><tr><th>Requested</th><th>Target</th><th>Value</th><th>Result</th><th>Reason</th></tr></thead><tbody>{[...data.commands].reverse().map((command) => <tr key={command.id}><td>{formatDateTime(command.requested_at)}</td><td>{data.channels.find((entry) => entry.id === command.target_channel_id)?.name}</td><td>{String(command.value)}</td><td><Status tone={command.status === "applied" ? "ok" : command.status === "rejected" ? "critical" : "neutral"}>{command.status}</Status></td><td>{command.reason_code ?? command.reason}</td></tr>)}</tbody></table></Card>
  </>;
}

export function SettingsScreen({ data, runtimeMode, actions }: { data: FarmData; runtimeMode: RuntimeMode; actions: ScreenActions }) {
  const input = useRef<HTMLInputElement>(null), [includeMedia, setIncludeMedia] = useState(true), [error, setError] = useState<string>();
  const importFile = async (event: ChangeEvent<HTMLInputElement>) => { const file = event.target.files?.[0]; if (!file) return; try { setError(undefined); await actions.importArchive(JSON.parse(await file.text()) as GrowNerveArchive); } catch (cause) { setError(cause instanceof Error ? cause.message : "Import failed"); } finally { event.target.value = ""; } };
  const estimatedBytes = new Blob([JSON.stringify(data)]).size;
  const browser = runtimeMode === "browser";
  return <><PageHeader eyebrow="Runtime and data portability" title="Settings" description={browser ? "Browser data stays in IndexedDB and can be moved or restored through the complete versioned GrowNerve archive." : "Server data is persisted by the Go/PostgreSQL runtime; deployment authentication and backups are managed by the server environment."} />
    <div className="gn-settings-grid"><Card title="Runtime" subtitle="Current application adapter"><dl className="gn-settings-list"><div><dt>Mode</dt><dd><Status tone={browser ? "simulated" : "ok"}>{browser ? "Browser only" : "Server"}</Status></dd></div><div><dt>Storage</dt><dd>{browser ? "IndexedDB" : "PostgreSQL + media store"}</dd></div><div><dt>Application</dt><dd>0.1.0</dd></div><div><dt>Archive schema</dt><dd>Version 1</dd></div><div><dt>Records</dt><dd>{Object.values(data).reduce((sum, records) => sum + records.length, 0).toLocaleString()}</dd></div></dl></Card>
      {browser && <Card title="Portable backup" subtitle="Complete domain data with stable UUID identities"><div className="gn-settings-action"><div className="gn-settings-icon"><FileJson /></div><div><strong>Export all farm data</strong><p>Estimated JSON size: {(estimatedBytes / 1024).toFixed(1)} KB</p><label className="gn-checkbox"><input type="checkbox" checked={includeMedia} onChange={(event) => setIncludeMedia(event.target.checked)} /> Include media in base64</label></div><button className="gn-button primary" onClick={() => actions.exportArchive(includeMedia)}><Download size={16} /> Export</button></div><div className="gn-settings-action"><div className="gn-settings-icon"><Upload /></div><div><strong>Replace from archive</strong><p>The complete archive is validated before the current farm is changed.</p>{error && <span className="gn-error">{error}</span>}</div><button className="gn-button" onClick={() => input.current?.click()}><Upload size={16} /> Import</button><input ref={input} className="gn-hidden" type="file" accept=".json,.grownerve.json,application/json" onChange={importFile} /></div></Card>}
      {browser && <Card title="Local farm actions" subtitle="Destructive actions require deliberate confirmation"><div className="gn-danger-actions"><div><RotateCcw /><span><strong>Reload pilot example</strong><p>Replace local data with the deterministic 3 × 3 ft DWC reference farm.</p></span><button className="gn-button" onClick={() => { if (confirm("Replace local data with the pilot example?")) actions.loadPilot(); }}>Load pilot</button></div><div><Power /><span><strong>Reset local farm</strong><p>Clear GrowNerve records from this browser. Export first if needed.</p></span><button className="gn-button danger" onClick={() => { if (confirm("Permanently clear this browser's local GrowNerve farm?")) actions.reset(); }}>Reset</button></div></div></Card>}
      <Card title="Safety boundary" subtitle={browser ? "What browser-only operation means" : "What server operation means"}><div className="gn-safety-copy"><ShieldCheck /><div><strong>{browser ? "Designed for planning, record keeping, demonstrations, and simulation." : "Persistent control is routed through authenticated server and edge safety checks."}</strong><p>{browser ? "It does not provide unattended real-equipment guarantees. Use the full Go/PostgreSQL/MQTT/ESP32 deployment for persistent physical control." : "Physical commissioning is still required before real outputs are enabled; repository defaults do not prove hardware safety."}</p></div></div></Card></div>
  </>;
}
