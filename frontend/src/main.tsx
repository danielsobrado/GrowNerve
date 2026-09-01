import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App, defaultBrowserRepository } from "./App";
import "./index.css";

const runtimeMode = (import.meta.env.VITE_RUNTIME_MODE === "server" ? "server" : "browser") as "server" | "browser";

createRoot(document.getElementById("root")!).render(<StrictMode><App repository={defaultBrowserRepository} runtimeMode={runtimeMode} /></StrictMode>);
