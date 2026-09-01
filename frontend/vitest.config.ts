import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary"],
      thresholds: { lines: 80, functions: 80, statements: 80, branches: 80 },
      include: ["src/domain/**/*.ts", "src/runtime/**/*.ts"],
    },
  },
});
