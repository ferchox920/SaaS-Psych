import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  timeout: 30000,
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: "http://127.0.0.1:3103",
    channel: process.env.PLAYWRIGHT_CHANNEL || undefined,
    screenshot: "only-on-failure",
    trace: "off", // Avoid retaining even fictional clinical payloads in traces.
  },
  webServer: {
    command: "pnpm start --port 3103",
    url: "http://127.0.0.1:3103/login",
    reuseExistingServer: false,
    timeout: 60000,
  },
});
