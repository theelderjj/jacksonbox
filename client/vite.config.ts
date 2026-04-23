/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Vite dev server proxies `/ws` to the Go backend so the client can use a
// relative WebSocket URL and avoid CORS gymnastics. Production serves the
// built client behind the same origin.
export default defineConfig(() => {
  const devPort = Number(process.env["VITE_PORT"] ?? "4273");
  const backendPort = Number(process.env["BACKEND_PORT"] ?? "8787");

  return {
    plugins: [react()],
    server: {
      port: devPort,
      strictPort: true,
      // Bind 0.0.0.0 so phones on the same Wi-Fi can reach the dev server.
      // The backend still stays localhost-only behind the proxy.
      host: true,
      allowedHosts: [".ngrok-free.app", ".ngrok-free.dev", ".ngrok.app", ".ngrok.io"],
      proxy: {
        "/api": {
          target: `http://localhost:${backendPort}`,
          changeOrigin: true,
        },
        "/ws": {
          target: `ws://localhost:${backendPort}`,
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
  };
});
