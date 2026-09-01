import {
  Activity, AlertTriangle, Bell, Boxes, ChevronRight, Cpu, History,
  LayoutDashboard, Leaf, Menu, PackageOpen, Search, Settings, Sparkles, Sprout,
  Workflow, X,
} from "lucide-react";
import { useState, type ComponentType, type ReactNode } from "react";
import type { RuntimeMode } from "../domain/model";
import { Status } from "./Status";

export type RouteKey = "overview" | "farm" | "grows" | "twin" | "alerts" | "history" | "inventory" | "automation" | "devices" | "settings";
type Icon = ComponentType<{ size?: number; strokeWidth?: number }>;
const navigation: Array<{ group: string; items: Array<{ key: RouteKey; label: string; icon: Icon }> }> = [
  { group: "Operations", items: [{ key: "overview", label: "Overview", icon: LayoutDashboard }, { key: "farm", label: "Farm", icon: Boxes }, { key: "grows", label: "Grow Cycles", icon: Sprout }, { key: "twin", label: "3D Twin", icon: Sparkles }, { key: "alerts", label: "Alerts", icon: AlertTriangle }, { key: "history", label: "History", icon: History }] },
  { group: "Control", items: [{ key: "inventory", label: "Inventory", icon: PackageOpen }, { key: "automation", label: "Automation", icon: Workflow }, { key: "devices", label: "Devices", icon: Cpu }] },
  { group: "System", items: [{ key: "settings", label: "Settings", icon: Settings }] },
];

export function AppShell({ route, onRoute, runtimeMode, alertCount, children }: { route: RouteKey; onRoute: (route: RouteKey) => void; runtimeMode: RuntimeMode; alertCount: number; children: ReactNode }) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const navigate = (key: RouteKey) => { onRoute(key); setMobileOpen(false); };
  const sidebar = <aside className={`gn-sidebar ${mobileOpen ? "is-open" : ""}`}>
    <div className="gn-brand"><div className="gn-logo"><Leaf size={19} /></div><div><strong>GrowNerve</strong><span>FARM INTELLIGENCE · V0.1</span></div><button className="gn-mobile-close" onClick={() => setMobileOpen(false)} aria-label="Close navigation"><X /></button></div>
    <div className="gn-runtime"><Status tone={runtimeMode === "browser" ? "simulated" : "ok"}>{runtimeMode === "browser" ? "Browser only" : "Server connected"}</Status></div>
    <div className="gn-operator"><div className="gn-avatar">OP</div><div><strong>Local Operator</strong><span>OPERATOR</span></div></div>
    <nav>{navigation.map((section) => <section key={section.group}><p>{section.group}</p>{section.items.map(({ key, label, icon: Icon }) => <button key={key} className={route === key ? "active" : ""} onClick={() => navigate(key)}><Icon size={17} strokeWidth={1.8} /><span>{label}</span>{key === "alerts" && alertCount > 0 && <b>{alertCount}</b>}</button>)}</section>)}</nav>
    <div className="gn-sidebar-foot"><Activity size={16} /><div><strong>Runtime active</strong><span>IndexedDB · simulator ready</span></div></div>
  </aside>;
  return <div className="gn-app">
    {sidebar}{mobileOpen && <button className="gn-scrim" onClick={() => setMobileOpen(false)} aria-label="Close navigation overlay" />}
    <div className="gn-main">
      <header className="gn-topbar"><button className="gn-menu" onClick={() => setMobileOpen(true)} aria-label="Open navigation"><Menu /></button><div className="gn-crumb"><span>Workspace</span><ChevronRight size={14} /><strong>{navigation.flatMap((entry) => entry.items).find((entry) => entry.key === route)?.label}</strong></div><button className="gn-search" onClick={() => onRoute("farm")}><Search size={17} /><span>Search farm entities…</span><kbd>/</kbd></button><button className="gn-icon-button" onClick={() => onRoute("alerts")} aria-label="Open alerts"><Bell size={18} />{alertCount > 0 && <i />}</button><div className="gn-top-avatar">OP</div></header>
      <main className="gn-page">{children}</main>
    </div>
  </div>;
}
