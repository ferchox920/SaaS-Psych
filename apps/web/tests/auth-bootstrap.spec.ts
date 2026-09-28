import { expect, test } from "@playwright/test";

const tenant = "11111111-1111-4111-8111-111111111111";

test("a fresh browser shows login without attempting token refresh", async ({ page }) => {
  let refreshCalls = 0;
  await page.route("**/api/v1/auth/refresh", (route) => {
    refreshCalls++;
    return route.fulfill({ status: 401, contentType: "application/json", body: "{}" });
  });

  await page.goto("/login");
  await expect(page.getByRole("button", { name: "Entrar" })).toBeVisible();
  expect(refreshCalls).toBe(0);
});

test("an invalid stored tenant returns to login and clears local storage", async ({ page }) => {
  await page.addInitScript((id) => localStorage.setItem("sessionflow.auth.tenant", id), tenant);
  await page.route("**/api/v1/auth/refresh", (route) =>
    route.fulfill({ status: 401, contentType: "application/json", body: "{}" }),
  );

  await page.goto("/login");
  await expect(page.getByRole("button", { name: "Entrar" })).toBeVisible();
  await expect.poll(() => page.evaluate(() => localStorage.getItem("sessionflow.auth.tenant"))).toBeNull();
});
