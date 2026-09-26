import { expect, test } from "@playwright/test";

test.skip(process.env.RUN_LIVE_DEMO === "1", "The live demo run uses a demo-mode frontend build");

test("normal build does not prefill demo credentials", async ({ page }) => {
  const scripts: Promise<string>[] = [];
  page.on("response", (response) => {
    if (response.url().includes("/_next/static/chunks/") && response.url().endsWith(".js")) {
      scripts.push(response.text());
    }
  });
  await page.goto("/login");
  await page.waitForLoadState("networkidle");
  await expect(page.getByText("Demo simulada · solo datos ficticios")).toHaveCount(0);
  await expect(page.getByLabel("Tenant ID")).toBeEmpty();
  await expect(page.getByLabel("Email")).toBeEmpty();
  await expect(page.getByLabel("Password")).toBeEmpty();
  await expect(page.getByLabel("Tenant ID")).not.toHaveAttribute("placeholder", "11111111-1111-1111-1111-111111111111");
  await expect(page.getByLabel("Email")).not.toHaveAttribute("placeholder", "owner@tenant-a.local");
  await expect(page.getByLabel("Password")).not.toHaveAttribute("placeholder", "ChangeMe123!");
  expect(scripts.length).toBeGreaterThan(0);
  const loadedScripts = (await Promise.all(scripts)).join("\n");
  for (const credential of ["11111111-1111-1111-1111-111111111111", "owner@tenant-a.local", "ChangeMe123!"]) {
    expect(loadedScripts.includes(credential), `${credential} was shipped in a normal-mode browser script`).toBe(false);
  }
});
