/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Vite dev server proxies `/ws` to the Go backend so the client can use a
// relative WebSocket URL and avoid CORS gymnastics. Production serves the
// built client behind the same origin.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    strictPort: true,
    // Bind 0.0.0.0 so phones on the same Wi-Fi can reach the dev server
    // (dev machine is 192.168.1.158). Vite still proxies /ws to the Go
    // backend on localhost:8080 — the backend stays localhost-only.
    host: true,
    allowedHosts: [".ngrok-free.app", ".ngrok-free.dev", ".ngrok.app", ".ngrok.io"],
    proxy: {
      "/ws": {
        target: "ws://localhost:8080",
        ws: true,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    sourcemap: true,
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./test/setup.ts"],
    // Unit tests live under src/**/__tests__. Playwright owns e2e/** and
    // has its own runner; explicitly exclude it so Vitest doesn't pick
    // those files up and choke on @playwright/test APIs.
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    exclude: ["node_modules", "dist", "e2e/**"],
    css: false,
  },
});
