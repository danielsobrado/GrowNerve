import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const browser = mode === "browser" || env.VITE_RUNTIME_MODE === "browser";
  return {
    base: env.VITE_BASE_PATH || (browser ? "/GrowNerve/" : "/"),
    plugins: [react(), VitePWA({
      registerType: "autoUpdate",
      manifest: {
        name: "GrowNerve",
        short_name: "GrowNerve",
        description: "Local-first indoor farm intelligence",
        theme_color: "#0d130f",
        background_color: "#080c09",
        display: "standalone",
        start_url: ".",
        icons: [{ src: "grownerve-mark.svg", sizes: "any", type: "image/svg+xml", purpose: "any maskable" }],
      },
      workbox: { globPatterns: ["**/*.{js,css,html,svg,png,woff2,glb}"] },
      devOptions: { enabled: browser },
    })],
    define: { "import.meta.env.VITE_RUNTIME_MODE": JSON.stringify(env.VITE_RUNTIME_MODE || (browser ? "browser" : "server")) },
    server: { host: "127.0.0.1", port: 5173 },
    build: { sourcemap: true },
  };
});
