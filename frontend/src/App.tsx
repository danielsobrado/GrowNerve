import { Download, FileUp, Leaf, Plus } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { AppShell, type RouteKey } from "./components/AppShell";
import type { FarmData, RuntimeMode } from "./domain/model";
import { emptyFarmData } from "./domain/model";
import { acknowledgeAlert, resolveAlert } from "./domain/invariants";
import { BrowserFarmRepository, type FarmRepository } from "./runtime/browserRepository";
import { createArchive, serializeArchive } from "./runtime/archive";
import { pilotData } from "./runtime/pilotData";
import { applySimulatedCommand, setDeviceOnline, tickSimulator } from "./runtime/simulator";
import type { Selection } from "./twin/DigitalTwin";
import {
  AlertsScreen, AutomationScreen, DevicesScreen, FarmScreen, GrowCyclesScreen, HistoryScreen,
  InventoryScreen, OverviewScreen, SettingsScreen, TwinScreen, type ScreenActions,
} from "./screens/screens";

const routeFromHash = (): RouteKey => {
  const candidate = location.hash.replace(/^#\/?/, "") as RouteKey;
  return ["overview", "farm", "grows", "twin", "alerts", "history", "inventory", "automation", "devices", "settings"].includes(candidate) ? candidate : "overview";
};

type ApplicationRepository = FarmRepository & Partial<Pick<BrowserFarmRepository, "importReplace">> & { issueCommand?: (intent: { targetChannelId: string; value: number | boolean; reason: string }) => Promise<unknown> };

export function App({ repository, runtimeMode = "browser" }: { repository: ApplicationRepository; runtimeMode?: RuntimeMode }) {
  const [data, setData] = useState<FarmData>();
  const [loading, setLoading] = useState(true);
  const [route, setRouteState] = useState<RouteKey>(routeFromHash);
  const [selection, setSelection] = useState<Selection>();

  useEffect(() => {
    let active = true;
    void repository.load().then((value) => { if (active) setData(value); }).finally(() => { if (active) setLoading(false); });
    const unsubscribe = repository.subscribe(() => { void repository.load().then((value) => { if (active) setData(value); }).catch(() => undefined); });
    return () => { active = false; unsubscribe(); };
  }, [repository]);
  useEffect(() => { const listener = () => setRouteState(routeFromHash()); window.addEventListener("hashchange", listener); return () => window.removeEventListener("hashchange", listener); }, []);
  const setRoute = useCallback((next: RouteKey) => { setRouteState(next); history.replaceState(null, "", `${location.pathname}${location.search}#/${next}`); }, []);
  const persist = useCallback(async (next: FarmData) => { await repository.replace(next); setData(next); }, [repository]);

  const loadPilot = useCallback(() => {
    if (runtimeMode !== "browser") return;
    void persist(tickSimulator(pilotData(), new Date().toISOString(), 1));
    setRoute("overview");
  }, [persist, runtimeMode, setRoute]);
  const reset = useCallback(() => { void repository.clear().then(() => setData(undefined)); setSelection(undefined); }, [repository]);

  const actions = useMemo<ScreenActions>(() => ({
    refreshSimulator: () => { if (runtimeMode === "browser" && data) void persist(tickSimulator(data)); },
    command: (channelId, value, reason) => {
      if (!data) return;
      if (runtimeMode === "server" && repository.issueCommand) {
        void repository.issueCommand({ targetChannelId: channelId, value, reason }).then(() => repository.load()).then((next) => { if (next) setData(next); });
      } else if (runtimeMode === "browser") {
        void persist(applySimulatedCommand(data, { targetChannelId: channelId, value, reason }));
      }
    },
    select: (next) => { setSelection(next); if (next) setRoute("twin"); },
    acknowledgeAlert: (alertId) => { if (!data) return; const next = structuredClone(data); const index = next.alerts.findIndex((entry) => entry.id === alertId); if (index >= 0) next.alerts[index] = acknowledgeAlert(next.alerts[index], "Local Operator", new Date().toISOString()); void persist(next); },
    resolveAlert: (alertId) => { if (!data) return; const next = structuredClone(data); const index = next.alerts.findIndex((entry) => entry.id === alertId); if (index >= 0) next.alerts[index] = resolveAlert(next.alerts[index], new Date().toISOString()); void persist(next); },
    addObservation: (notes, severity) => { if (!data?.grow_cycles[0]) return; const next = structuredClone(data), grow = next.grow_cycles[0], now = new Date().toISOString(); next.observations.push({ id: crypto.randomUUID(), grow_cycle_id: grow.id, target_type: selection?.type ?? "grow_cycle", target_id: selection?.id ?? grow.id, category: "general", severity, notes, observed_at: now, media_ids: [] }); next.events.push({ id: crypto.randomUUID(), type: "observation.recorded", occurred_at: now, actor: "Local Operator", entity_type: "grow_cycle", entity_id: grow.id, summary: notes }); void persist(next); },
    addInventoryAdjustment: (itemId, quantity, reason) => { if (!data) return; const next = structuredClone(data), item = next.inventory_items.find((entry) => entry.id === itemId); if (!item) return; next.inventory_adjustments.push({ id: crypto.randomUUID(), item_id: itemId, occurred_at: new Date().toISOString(), quantity, unit: item.unit, reason }); void persist(next); },
    toggleRule: (ruleId) => { if (!data) return; const next = structuredClone(data), rule = next.automation_rules.find((entry) => entry.id === ruleId); if (rule) rule.enabled = !rule.enabled; void persist(next); },
    setDeviceOnline: (deviceId, online) => { if (runtimeMode === "browser" && data) void persist(setDeviceOnline(data, deviceId, online)); },
    exportArchive: (includeMedia) => { if (!data) return; const archive = createArchive(data, { media: includeMedia ? [] : [] }), blob = new Blob([serializeArchive(archive)], { type: "application/json" }), url = URL.createObjectURL(blob), anchor = document.createElement("a"); anchor.href = url; anchor.download = `grownerve-${new Date().toISOString().slice(0, 10)}.grownerve.json`; anchor.click(); URL.revokeObjectURL(url); },
    importArchive: async (archive) => { if (repository.importReplace) await repository.importReplace(archive); else throw new Error("Import is unavailable in this runtime"); setData(await repository.load()); setRoute("overview"); },
    reset,
    loadPilot,
  }), [data, persist, repository, reset, selection, setRoute, loadPilot, runtimeMode]);

  useEffect(() => {
    if (!data || runtimeMode !== "browser") return;
    const interval = window.setInterval(() => {
      setData((current) => {
        if (!current) return current;
        const next = tickSimulator(current);
        void repository.replace(next);
        return next;
      });
    }, 30_000);
    return () => window.clearInterval(interval);
  }, [data, repository, runtimeMode]);

  if (loading) return <div className="gn-loading"><div className="gn-loading-mark"><Leaf /></div><p>Opening farm…</p></div>;
  if (!data) return <FirstRun repository={repository} onPilot={loadPilot} onCreated={setData} runtimeMode={runtimeMode} />;

  const screen = (() => {
    switch (route) {
      case "farm": return <FarmScreen data={data} actions={actions} />;
      case "grows": return <GrowCyclesScreen data={data} actions={actions} />;
      case "twin": return <TwinScreen data={data} selection={selection} actions={actions} />;
      case "alerts": return <AlertsScreen data={data} actions={actions} />;
      case "history": return <HistoryScreen data={data} />;
      case "inventory": return <InventoryScreen data={data} actions={actions} />;
      case "automation": return <AutomationScreen data={data} actions={actions} />;
      case "devices": return <DevicesScreen data={data} actions={actions} />;
      case "settings": return <SettingsScreen data={data} runtimeMode={runtimeMode} actions={actions} />;
      default: return <OverviewScreen data={data} actions={actions} />;
    }
  })();
  return <AppShell route={route} onRoute={setRoute} runtimeMode={runtimeMode} alertCount={data.alerts.filter((entry) => entry.status !== "resolved").length}>{screen}</AppShell>;
}

function FirstRun({ repository, onPilot, onCreated, runtimeMode }: { repository: ApplicationRepository; onPilot: () => void; onCreated: (data: FarmData) => void; runtimeMode: RuntimeMode }) {
  const [error, setError] = useState<string>();
  const createFarm = async () => { const data = emptyFarmData(); data.facilities.push({ id: crypto.randomUUID(), name: "My Indoor Farm", timezone: Intl.DateTimeFormat().resolvedOptions().timeZone }); await repository.replace(data); onCreated(data); };
  const importArchive = async (file?: File) => { if (!file || !repository.importReplace) return; try { setError(undefined); await repository.importReplace(JSON.parse(await file.text())); const data = await repository.load(); if (data) onCreated(data); } catch (cause) { setError(cause instanceof Error ? cause.message : "Import failed"); } };
  return <main className="gn-welcome"><div className="gn-welcome-brand"><div><Leaf /></div><span>GrowNerve</span></div><section><p className="gn-eyebrow">Local-first farm intelligence</p><h1>Welcome to GrowNerve</h1><p>{runtimeMode === "browser" ? "Start a private farm in this browser, explore the deterministic pilot tent, or restore a complete portable archive." : "Create the first server-backed farm configuration."}</p><div className="gn-welcome-actions"><button aria-label="Create farm" className="gn-welcome-card" onClick={createFarm}><span><Plus /></span><div><strong>Create farm</strong><p>{runtimeMode === "browser" ? "Start empty with IndexedDB persistence." : "Create the server-backed farm state."}</p></div></button>{runtimeMode === "browser" && <button aria-label="Load pilot example" className="gn-welcome-card featured" onClick={onPilot}><span><Leaf /></span><div><strong>Load pilot example</strong><p>Explore the 3 × 3 ft DWC reference grow.</p></div></button>}{runtimeMode === "browser" && <label className="gn-welcome-card"><span><FileUp /></span><div><strong>Import .grownerve.json</strong><p>Validate and restore a portable backup.</p></div><input type="file" accept=".json,.grownerve.json" onChange={(event) => void importArchive(event.target.files?.[0])} /></label>}</div>{error && <p className="gn-error">{error}</p>}{runtimeMode === "browser" && <p className="gn-welcome-note"><Download size={14} /> Your data stays local and remains exportable.</p>}</section></main>;
}

export const defaultBrowserRepository = new BrowserFarmRepository();
