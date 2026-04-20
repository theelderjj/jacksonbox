import { defineConfig, devices } from "@playwright/test";

// Playwright brings up the Go backend and the Vite dev server before any
// test runs, then tears them down after. Tests speak the real wire
// protocol end-to-end — a reverse of the unit suite (which mocks the
// socket). Trade-off: slower, but catches contract drift between the TS
// client + the Go server that unit tests can't.
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false, // one shared backend, one shared room
  retries: 0,
  workers: 1,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: "http://localhost:5173",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  webServer: [
    {
      // Go backend. `go run` recompiles on every test run — acceptable for
      // a small server and avoids a separate `go build` step in CI.
      command: "go run ./cmd/server",
      cwd: "..",
      port: 8080,
      reuseExistingServer: !process.env["CI"],
      timeout: 60_000,
    },
    {
      command: "npm run dev",
      port: 5173,
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
