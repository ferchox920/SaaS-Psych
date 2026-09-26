import { expect, test } from "@playwright/test";

test.skip(process.env.RUN_LIVE_DEMO !== "1", "Requires an opt-in demo-mode frontend build");

test("demo build clearly labels and prefills fictional local credentials", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByText("Demo simulada · solo datos ficticios")).toBeVisible();
  await expect(page.getByLabel("Tenant ID")).toHaveValue("11111111-1111-1111-1111-111111111111");
  await expect(page.getByLabel("Email")).toHaveValue("owner@tenant-a.local");
  await expect(page.getByLabel("Password")).toHaveValue("ChangeMe123!");
});
