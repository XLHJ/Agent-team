import { cpSync, existsSync } from "node:fs";
import path from "node:path";
import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";

// Vendored sub-apps are standalone projects embedded via iframe.
// Dev: each has its own vite dev server (/canvas-app -> 3002, /apexforge-app -> 5175)
//      and the ApexForge NestJS API runs on 3000 (proxy /api/v1).
// Build: their dist outputs are copied into dist/<name>/ next to the panel build.
function copySubApp(name: string): Plugin {
    return {
        name: `copy-${name}`,
        closeBundle() {
            // mcp-docs-app builds its client into <root>/dist/client; copy that.
            const rel = name === "mcp-docs-app" ? `${name}/dist/client` : `${name}/dist`;
            const src = path.resolve(__dirname, rel);
            const dst = path.resolve(__dirname, `dist/${name}`);
            if (existsSync(src)) {
                cpSync(src, dst, { recursive: true });
            }
        },
    };
}

export default defineConfig(({ command }) => ({
  // Relative base so the built panel works inside Electron (file://).
  base: command === "build" ? "./" : "/",
  plugins: [react(), copySubApp("canvas-app"), copySubApp("apexforge-app"), copySubApp("mcp-docs-app")],
  // Dev server proxies to the local Hub; production runs inside Electron.
  server: {
    port: 5173,
    // The vendored sub-apps run their own dev servers; watching their trees
    // (incl. node_modules) exhausts inotify watchers.
    watch: {
      ignored: ["**/canvas-app/**", "**/apexforge-app/**", "**/mcp-docs-app/**"],
    },
    proxy: {
      "/ws": {
        target: "ws://127.0.0.1:8080",
        ws: true,
      },
      "/panel": {
        target: "ws://127.0.0.1:8080",
        ws: true,
      },
      "/kitesurf": {
        target: "http://127.0.0.1:8080",
      },
      "/canvas-app": {
        target: "http://localhost:3002",
        changeOrigin: true,
      },
      "/apexforge-app": {
        target: "http://localhost:5175",
        changeOrigin: true,
      },
      "/mcp-docs-app": {
        target: "http://localhost:5176",
        changeOrigin: true,
      },
      // Order matters: /api/v1 (ApexForge, port 3000) must match before /api (mcp-docs, port 3001).
      "/api/v1": {
        target: "http://localhost:3000",
        changeOrigin: true,
      },
      "/api": {
        // mcp-docs Nest... Fastify API (npm run dev:server in panel/mcp-docs-app).
        target: "http://localhost:3001",
        changeOrigin: true,
      },
    },
  },
}));
