import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  testMatch: "calendar-patient-review.spec.ts",
  reporter: "list",
});
