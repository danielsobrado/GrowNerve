import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App, defaultBrowserRepository } from "./App";
import { ServerFarmRepository } from "./runtime/serverRepository";
import "./index.css";

const runtimeMode = (import.meta.env.VITE_RUNTIME_MODE === "server" ? "server" : "browser") as "server" | "browser";
const repository = runtimeMode === "server" ? new ServerFarmRepository(import.meta.env.VITE_API_URL || "http://127.0.0.1:8080") : defaultBrowserRepository;

createRoot(document.getElementById("root")!).render(<StrictMode><App repository={repository} runtimeMode={runtimeMode} /></StrictMode>);
