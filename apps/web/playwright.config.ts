import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  timeout: 30000,
  fullyParallel: false,
  workers: 1,
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:3103",
    channel: process.env.PLAYWRIGHT_CHANNEL || undefined,
    screenshot: "only-on-failure",
    trace: "off", // No clinical payload recording; tests contain synthetic fixtures only.
  },
  webServer: {
    command: "pnpm start --port 3103",
    url: "http://127.0.0.1:3103/login",
    reuseExistingServer: false,
    timeout: 60000,
  },
});
