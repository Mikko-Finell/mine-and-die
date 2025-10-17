import { defineConfig, type PluginOption } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export default defineConfig({
  plugins: [react() as PluginOption],
  resolve: {
    alias: {
      "@js-effects/effects-lib": path.resolve(__dirname, "../../packages/effects-lib/src")
    }
  },
  server: {
    port: 5173
  }
});
