import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";
export default defineConfig(function (_a) {
    var mode = _a.mode;
    var env = loadEnv(mode, process.cwd(), "");
    var browser = mode === "browser" || env.VITE_RUNTIME_MODE === "browser";
    return {
        base: env.VITE_BASE_PATH || (browser ? "/GrowNerve/" : "/"),
        plugins: [react(), VitePWA({
                registerType: "autoUpdate",
                manifest: { name: "GrowNerve", short_name: "GrowNerve", description: "Local-first indoor farm intelligence", theme_color: "#18211b", background_color: "#f7f7f2", display: "standalone", start_url: ".", icons: [] },
                workbox: { globPatterns: ["**/*.{js,css,html,svg,png,woff2,glb}"] },
                devOptions: { enabled: browser },
            })],
        define: { "import.meta.env.VITE_RUNTIME_MODE": JSON.stringify(env.VITE_RUNTIME_MODE || (browser ? "browser" : "server")) },
        server: { host: "127.0.0.1", port: 5173 },
        build: { sourcemap: true },
    };
});
