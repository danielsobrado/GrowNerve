import { useState } from "react";
import { addItem, catalog, duplicateItem, initialTentLayout, parseLayoutFile, parseTentLayout, removeItem, serializeLayoutFile, updateItem, type TentLayout } from "./tentLayout";

/** How the selected object's 3D handles behave. */
export type TransformMode = "translate" | "scale";

export function useTentDesigner(storageKey: string) {
  const [history, setHistory] = useState(() => {
    let layout = initialTentLayout();
    try { layout = parseTentLayout(localStorage.getItem(storageKey)); } catch { /* Invalid drafts do not block the editor. */ }
    return { past: [] as TentLayout[], present: layout, future: [] as TentLayout[] };
  });
  const [selected, setSelected] = useState<string>();
  const [placing, setPlacing] = useState(false);
  const [mode, setMode] = useState<TransformMode>("translate");
  const [message, setMessage] = useState("Layout saves in this browser.");
  const save = (next: typeof history) => {
    setHistory(next);
    try { localStorage.setItem(storageKey, JSON.stringify(next.present)); setMessage("Layout saved in this browser."); }
    catch { setMessage("Storage is unavailable. Keep this page open to retain your edits."); }
  };
  const commit = (change: (layout: TentLayout) => TentLayout) => {
    try { const next = change(history.present); save({ past: [...history.past.slice(-49), history.present], present: next, future: [] }); return true; }
    catch (error) { setMessage(error instanceof Error ? error.message : "Could not apply layout change."); return false; }
  };
  const select = (id: string | undefined) => { setSelected(id); setPlacing(false); };
  const item = history.present.items.find((entry) => entry.id === selected);
  return {
    layout: history.present, selected: item?.id, item, select, placing, setPlacing, mode, setMode, message, commit,
    canUndo: history.past.length > 0, canRedo: history.future.length > 0,
    undo: () => { const previous = history.past.at(-1); if (previous) save({ past: history.past.slice(0, -1), present: previous, future: [history.present, ...history.future] }); },
    redo: () => { const next = history.future[0]; if (next) save({ past: [...history.past, history.present], present: next, future: history.future.slice(1) }); },
    add: (kind: string) => { const id = crypto.randomUUID(); if (commit((current) => addItem(current, kind, id))) select(id); },
    duplicate: () => { if (!item) return; const id = crypto.randomUUID(); if (commit((current) => duplicateItem(current, item.id, id))) select(id); },
    remove: () => { if (item && commit((current) => removeItem(current, item.id))) select(undefined); },
    rotate: (degrees = 90) => { if (item) commit((current) => updateItem(current, { ...item, rotation: (item.rotation + degrees + 360) % 360 })); },
    rename: (name: string) => { commit((current) => ({ ...current, name: name.slice(0, 80) })); },
    /** Downloads the whole layout (space, every object and its parameters) as a portable file. */
    exportFile: () => {
      const blob = new Blob([serializeLayoutFile(history.present)], { type: "application/json" });
      const url = URL.createObjectURL(blob), anchor = document.createElement("a");
      const slug = history.present.name.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "layout";
      anchor.href = url; anchor.download = `${slug}.grownerve-layout.json`; anchor.click();
      URL.revokeObjectURL(url);
      setMessage(`Exported “${history.present.name}”.`);
    },
    /** Replaces the layout from an exported file after validating all of it; undo restores the previous layout. */
    importFile: async (file: File) => {
      try {
        const imported = parseLayoutFile(await file.text());
        if (commit(() => imported)) { select(undefined); setMessage(`Imported “${imported.name}” with ${imported.items.length} object${imported.items.length === 1 ? "" : "s"}. Undo restores the previous layout.`); }
      } catch (error) { setMessage(error instanceof Error ? `Import failed: ${error.message}` : "Import failed."); }
    },
    resetSize: () => { const entry = catalog.find((candidate) => candidate.kind === item?.kind); if (item && entry) commit((current) => updateItem(current, { ...item, size: [...entry.size] })); },
  };
}
export type TentEditor = ReturnType<typeof useTentDesigner>;
