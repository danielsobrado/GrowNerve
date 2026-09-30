import { AlertTriangle, CloudOff, Plug, Radio, RefreshCw, ShieldAlert, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { integrationProviders, type Device, type FarmData, type IntegrationProvider, type RuntimeMode } from "../domain/model";
import { Card, PageHeader, Status } from "../components/Status";
import {
  buildAdoptRequest, defaultDraft, displayUnit, draftProblems, permitJoinRemaining, providerDescriptions, providerLabels, selectsCommand,
  type AdoptDraft, type DiscoveredDevice, type IntegrationClient, type IntegrationStatus,
} from "../runtime/integrations";
import "./integrations.css";

const PAIRING_SECONDS = 120;
const STATUS_REFRESH_MS = 10_000;
const deviceTypes: Device["type"][] = ["sensor", "light", "fan", "air_pump", "controller"];

const stateTone = (state: IntegrationStatus["state"]) =>
  state === "connected" ? "ok" : state === "degraded" || state === "connecting" ? "warning" : state === "error" ? "critical" : "neutral";

const message = (cause: unknown) => cause instanceof Error ? cause.message : "The request failed";

const clock = (seconds: number) => `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;

export function IntegrationsScreen({ data, runtimeMode, client, onOpenDevices }: { data: FarmData; runtimeMode: RuntimeMode; client?: IntegrationClient; onOpenDevices: () => void }) {
  if (runtimeMode === "browser" || !client) return <BrowserNotice />;
  return <ServerIntegrations data={data} client={client} onOpenDevices={onOpenDevices} />;
}

function BrowserNotice() {
  return <><PageHeader eyebrow="Third-party devices" title="Integrations" description="Connect Zigbee, Matter and Home Assistant devices to the farm." />
    <div className="gn-callout warning"><div><CloudOff /><span><strong>Integrations need the server runtime.</strong> The browser never talks to hardware; the GrowNerve server connects to Zigbee2MQTT, the Matter Server and Home Assistant and applies the same safety checks as for its own controllers.</span></div></div>
    <div className="gn-integration-grid">{integrationProviders.map((provider) => <Card key={provider} title={providerLabels[provider]} subtitle={providerDescriptions[provider]}><Status tone="neutral">Server only</Status></Card>)}</div>
  </>;
}

function ServerIntegrations({ data, client, onOpenDevices }: { data: FarmData; client: IntegrationClient; onOpenDevices: () => void }) {
  const [statuses, setStatuses] = useState<IntegrationStatus[]>();
  const [statusError, setStatusError] = useState<string>();
  const [provider, setProvider] = useState<IntegrationProvider>();
  const [now, setNow] = useState(() => Date.now());

  const refresh = useCallback(async () => {
    try {
      const next = await client.listIntegrations();
      setStatuses(next);
      setStatusError(undefined);
      setProvider((current) => current ?? next.find((entry) => entry.enabled)?.provider);
    } catch (cause) {
      setStatusError(message(cause));
    }
  }, [client]);

  useEffect(() => {
    void refresh();
    const interval = window.setInterval(() => void refresh(), STATUS_REFRESH_MS);
    return () => window.clearInterval(interval);
  }, [refresh]);

  const pairing = statuses?.some((status) => permitJoinRemaining(status, now) > 0);
  useEffect(() => {
    if (!pairing) return;
    const tick = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(tick);
  }, [pairing]);

  const selected = statuses?.find((status) => status.provider === provider);
  return <><PageHeader eyebrow="Third-party devices" title="Integrations" description="Adopt Zigbee, Matter and Home Assistant devices as farm devices. Their readings become ordinary channels and their outputs use the same command safety checks as GrowNerve controllers." actions={<button className="gn-button" onClick={() => void refresh()}><RefreshCw size={16} /> Refresh</button>} />
    {statusError && <div className="gn-callout warning" role="alert"><div><AlertTriangle /><span>{statusError}</span></div></div>}
    <div className="gn-integration-grid">
      {(statuses ?? []).map((status) => <ProviderCard key={status.provider} status={status} now={now} active={status.provider === provider} onSelect={() => setProvider(status.provider)} client={client} onChanged={refresh} />)}
      {!statuses && !statusError && <p className="gn-muted">Loading integrations…</p>}
    </div>
    {selected?.enabled && <DiscoveredDevices key={selected.provider} provider={selected.provider} data={data} client={client} onAdopted={refresh} onOpenDevices={onOpenDevices} />}
  </>;
}

function ProviderCard({ status, now, active, onSelect, client, onChanged }: { status: IntegrationStatus; now: number; active: boolean; onSelect: () => void; client: IntegrationClient; onChanged: () => Promise<void> }) {
  const [busy, setBusy] = useState(false), [error, setError] = useState<string>();
  const remaining = permitJoinRemaining(status, now);
  const permitJoin = async (seconds: number) => {
    setBusy(true);
    setError(undefined);
    try { await client.permitJoin(status.provider, seconds); await onChanged(); } catch (cause) { setError(message(cause)); } finally { setBusy(false); }
  };
  const label = providerLabels[status.provider];
  return <Card className={`gn-integration-card ${active ? "is-active" : ""}`} title={label} subtitle={providerDescriptions[status.provider]} action={<Status tone={stateTone(status.state)}>{status.state}</Status>}>
    {status.enabled ? <>
      {status.detail && <p className="gn-integration-detail">{status.detail}</p>}
      <dl className="gn-integration-counts"><div><dt>Discovered</dt><dd>{status.device_count}</dd></div><div><dt>Adopted</dt><dd>{status.adopted_count}</dd></div></dl>
      <div className="gn-integration-actions">
        <button className={`gn-button ${active ? "primary" : ""}`} onClick={onSelect} aria-pressed={active}>Show {label} devices</button>
        {status.permit_join_supported && (remaining > 0
          ? <button className="gn-button" disabled={busy} onClick={() => void permitJoin(0)}><X size={15} /> Stop pairing · {clock(remaining)}</button>
          : <button className="gn-button" disabled={busy || status.state !== "connected"} onClick={() => void permitJoin(PAIRING_SECONDS)}><Radio size={15} /> Pair new devices</button>)}
      </div>
      {remaining > 0 && <p className="gn-integration-detail">The {label} network accepts new devices for {clock(remaining)}. Put the device into pairing mode now; it appears below once it joins.</p>}
      {error && <p className="gn-error" role="alert">{error}</p>}
    </> : <p className="gn-integration-detail">Disabled. Enable <code>integrations.{status.provider}</code> in the server configuration.</p>}
  </Card>;
}

function DiscoveredDevices({ provider, data, client, onAdopted, onOpenDevices }: { provider: IntegrationProvider; data: FarmData; client: IntegrationClient; onAdopted: () => Promise<void>; onOpenDevices: () => void }) {
  const [devices, setDevices] = useState<DiscoveredDevice[]>();
  const [error, setError] = useState<string>();
  const [adopting, setAdopting] = useState<DiscoveredDevice>();
  const [notice, setNotice] = useState<string>();

  const load = useCallback(async () => {
    try { setDevices(await client.listDiscovered(provider)); setError(undefined); } catch (cause) { setError(message(cause)); }
  }, [client, provider]);
  useEffect(() => { void load(); }, [load]);

  const sorted = useMemo(() => [...(devices ?? [])].sort((left, right) => Number(Boolean(left.adopted_device_id)) - Number(Boolean(right.adopted_device_id)) || left.name.localeCompare(right.name)), [devices]);
  const adopted = async (name: string) => {
    setAdopting(undefined);
    setNotice(`${name} was adopted. Its channels appear on the Devices screen and start recording immediately.`);
    await Promise.all([load(), onAdopted()]);
  };

  return <>
    <Card title={`${providerLabels[provider]} devices`} subtitle="Devices the provider knows about. Adopt one to bind it to a zone." action={<button className="gn-text-button" onClick={() => void load()}><RefreshCw size={14} /> Rescan</button>}>
      {error && <p className="gn-error" role="alert">{error}</p>}
      {notice && <p className="gn-integration-notice" role="status">{notice}</p>}
      {devices && devices.length === 0 && <div className="gn-empty"><Plug /><p>No devices yet. Pair one, then rescan.</p></div>}
      {sorted.length > 0 && <table className="gn-table gn-integration-table"><thead><tr><th>Device</th><th>Model</th><th>Readings and outputs</th><th>Status</th><th /></tr></thead><tbody>
        {sorted.map((device) => <tr key={device.external_id}>
          <td><strong>{device.name}</strong><small>{device.external_id}</small></td>
          <td>{[device.manufacturer, device.model].filter(Boolean).join(" · ") || "—"}{device.power_source && <small>{device.power_source === "battery" ? "Battery" : "Mains"}</small>}</td>
          <td><div className="gn-chip-row">{device.capabilities.filter((capability) => !capability.diagnostic).map((capability) => <span key={capability.key} className={`gn-chip ${capability.kind === "command" ? "is-command" : ""}`}>{capability.label}{displayUnit(capability.unit) ? ` · ${displayUnit(capability.unit)}` : ""}</span>)}</div></td>
          <td><Status tone={device.available ? "ok" : "critical"}>{device.available ? "Available" : "Unavailable"}</Status></td>
          <td className="gn-integration-row-action">{device.adopted_device_id
            ? <button className="gn-text-button" onClick={onOpenDevices}>Adopted · view</button>
            : <button className="gn-button primary" onClick={() => { setNotice(undefined); setAdopting(device); }} aria-label={`Adopt ${device.name}`}>Adopt</button>}</td>
        </tr>)}
      </tbody></table>}
    </Card>
    {adopting && <AdoptForm key={adopting.external_id} provider={provider} device={adopting} data={data} client={client} onCancel={() => setAdopting(undefined)} onAdopted={adopted} />}
  </>;
}

function AdoptForm({ provider, device, data, client, onCancel, onAdopted }: { provider: IntegrationProvider; device: DiscoveredDevice; data: FarmData; client: IntegrationClient; onCancel: () => void; onAdopted: (name: string) => Promise<void> }) {
  const [draft, setDraft] = useState<AdoptDraft>(() => defaultDraft(device, data.zones[0]?.id ?? ""));
  const [submitting, setSubmitting] = useState(false), [error, setError] = useState<string>();
  const form = useRef<HTMLDivElement>(null);
  // The form opens below the device table; bring it into view so the click visibly did something.
  useEffect(() => { form.current?.scrollIntoView?.({ behavior: "smooth", block: "start" }); }, []);
  const problems = draftProblems(device, draft, data);
  const controllable = selectsCommand(device, draft);
  const update = (index: number, change: Partial<AdoptDraft["selections"][number]>) =>
    setDraft((current) => ({ ...current, selections: current.selections.map((selection, position) => position === index ? { ...selection, ...change } : selection) }));
  const optionalNumber = (value: string) => value === "" ? undefined : Number(value);

  const submit = async () => {
    setSubmitting(true);
    setError(undefined);
    try {
      await client.adoptDevice(provider, device.external_id, buildAdoptRequest(device, draft));
      await onAdopted(draft.name.trim() || device.name);
    } catch (cause) {
      setError(message(cause));
    } finally {
      setSubmitting(false);
    }
  };

  if (data.zones.length === 0) {
    return <Card title={`Adopt ${device.name}`} action={<button className="gn-text-button" onClick={onCancel}>Close</button>}><p className="gn-integration-detail">Create a zone on the Farm screen first; adopted devices belong to a zone.</p></Card>;
  }
  return <div ref={form}><Card className="gn-adopt-card" title={`Adopt ${device.name}`} subtitle={`${providerLabels[provider]} · ${device.external_id}`} action={<button className="gn-text-button" onClick={onCancel} aria-label="Cancel adoption"><X size={14} /> Cancel</button>}>
    <form className="gn-form gn-adopt-form" onSubmit={(event) => { event.preventDefault(); if (problems.length === 0) void submit(); }}>
      <div className="gn-adopt-fields">
        <label>Zone<select value={draft.zoneId} onChange={(event) => setDraft({ ...draft, zoneId: event.target.value })}>{data.zones.map((zone) => <option key={zone.id} value={zone.id}>{zone.name}</option>)}</select></label>
        <label>Device name<input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
        <label>Device type<select value={draft.type} onChange={(event) => setDraft({ ...draft, type: event.target.value as Device["type"] })}>{deviceTypes.map((type) => <option key={type} value={type}>{type.replaceAll("_", " ")}</option>)}</select></label>
      </div>
      <table className="gn-table gn-adopt-table"><thead><tr><th>Use</th><th>Capability</th><th>Channel key</th><th>Channel name</th><th>Safe range</th></tr></thead><tbody>
        {draft.selections.map((selection, index) => {
          const capability = device.capabilities[index];
          const command = capability.kind === "command";
          return <tr key={capability.key} className={selection.selected ? "" : "is-off"}>
            <td><input type="checkbox" aria-label={`Use ${capability.label}`} checked={selection.selected} onChange={(event) => update(index, { selected: event.target.checked })} /></td>
            <td><strong>{capability.label}</strong><small>{command ? "Output" : capability.kind === "state" ? "State" : "Reading"}{displayUnit(capability.unit) ? ` · ${displayUnit(capability.unit)}` : ""}{capability.diagnostic ? " · diagnostic" : ""}</small></td>
            <td><input aria-label={`${capability.label} channel key`} value={selection.key} disabled={!selection.selected} onChange={(event) => update(index, { key: event.target.value })} /></td>
            <td><input aria-label={`${capability.label} channel name`} value={selection.name} disabled={!selection.selected} onChange={(event) => update(index, { name: event.target.value })} /></td>
            <td>{command && capability.value_type === "number"
              ? <div className="gn-adopt-range"><input type="number" aria-label={`${capability.label} safe minimum`} value={selection.safeMinimum ?? ""} min={capability.minimum} max={capability.maximum} disabled={!selection.selected} onChange={(event) => update(index, { safeMinimum: optionalNumber(event.target.value) })} /><span>–</span><input type="number" aria-label={`${capability.label} safe maximum`} value={selection.safeMaximum ?? ""} min={capability.minimum} max={capability.maximum} disabled={!selection.selected} onChange={(event) => update(index, { safeMaximum: optionalNumber(event.target.value) })} /></div>
              : <span className="gn-muted">{command ? "On / off" : "—"}</span>}</td>
          </tr>;
        })}
      </tbody></table>
      {controllable && <label className="gn-adopt-failsafe"><input type="checkbox" checked={draft.acknowledgeNoEdgeFailsafe} onChange={(event) => setDraft({ ...draft, acknowledgeNoEdgeFailsafe: event.target.checked })} /><ShieldAlert size={18} /><span><strong>This output has no controller-side failsafe.</strong> If the server is unavailable the device keeps its last state and runs no schedule. Don't use it for lights, pumps or anything that must fail safe; keep those on a GrowNerve controller.</span></label>}
      {problems.length > 0 && <ul className="gn-adopt-problems">{problems.map((problem) => <li key={problem}>{problem}</li>)}</ul>}
      {error && <p className="gn-error" role="alert">{error}</p>}
      <div className="gn-integration-actions"><button className="gn-button primary" type="submit" disabled={submitting || problems.length > 0}>{submitting ? "Adopting…" : "Adopt device"}</button><button className="gn-button" type="button" onClick={onCancel}>Cancel</button></div>
    </form>
  </Card></div>;
}
