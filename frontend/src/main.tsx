import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App, defaultBrowserRepository } from "./App";
import { ServerFarmRepository } from "./runtime/serverRepository";
import "./index.css";

const runtimeMode = (import.meta.env.VITE_RUNTIME_MODE === "server" ? "server" : "browser") as "server" | "browser";

/**
 * Reads the API credential for server mode. It is held in sessionStorage rather
 * than localStorage so it does not outlive the browser session, and the read is
 * guarded because a browser with site data blocked throws on access.
 */
function apiToken(): string | undefined {
  try {
    return sessionStorage.getItem("grownerve.token") ?? undefined;
  } catch {
    return undefined;
  }
}

const repository = runtimeMode === "server"
  ? new ServerFarmRepository(import.meta.env.VITE_API_URL || "http://127.0.0.1:8080", { token: apiToken() })
  : defaultBrowserRepository;

createRoot(document.getElementById("root")!).render(
  <StrictMode><App repository={repository} runtimeMode={runtimeMode} /></StrictMode>,
);
