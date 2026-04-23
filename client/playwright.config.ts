import { defineConfig, devices } from "@playwright/test";

const backendPort = Number(process.env["PLAYWRIGHT_BACKEND_PORT"] ?? "8787");
const devPort = Number(process.env["PLAYWRIGHT_DEV_PORT"] ?? "4273");

// Playwright brings up the Go backend and the Vite dev server before any
// test runs, then tears them down after. Tests speak the real wire
// protocol end-to-end, which catches contract drift between the TS client
// and the Go server that unit tests cannot.
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: [["list", { printSteps: true }], ["html", { open: "never" }]],
  use: {
    baseURL: `http://localhost:${devPort}`,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  webServer: [
    {
      command: `go run ./cmd/server --addr :${backendPort}`,
      cwd: "..",
      port: backendPort,
      reuseExistingServer: !process.env["CI"],
      timeout: 60_000,
    },
    {
      command: "npm run dev",
      port: devPort,
      reuseExistingServer: !process.env["CI"],
      timeout: 60_000,
    },
  ],
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
