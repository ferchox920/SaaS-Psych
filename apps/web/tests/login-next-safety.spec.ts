import { expect, test, type Page } from "@playwright/test";
import { safeLoginNext } from "../src/features/auth/lib/login-next";

const tenant = "11111111-1111-1111-1111-111111111111";

test("login next accepts only same-app absolute paths", () => {
  expect(safeLoginNext("/clients/abc/clinical?tab=reports#latest")).toBe("/clients/abc/clinical?tab=reports#latest");
  for (const unsafe of [
    "javascript:sessionStorage.setItem('unsafe-next','executed')",
    "https://example.invalid/landing",
    "//example.invalid/landing",
    "/\\example.invalid/landing",
    "\\example.invalid/landing",
    " /dashboard",
    "/dashboard\n",
    "/%2fexample.invalid/landing",
    "/%5cexample.invalid/landing",
    "/login?next=%2Fappointments",
    "/login/",
    "/lo%67in",
  ]) {
    expect(safeLoginNext(unsafe), unsafe).toBe("/dashboard");
  }
  expect(safeLoginNext(["/dashboard", "https://example.invalid"])).toBe("/dashboard");
});

async function mockLogin(page: Page) {
  await page.route("**/api/v1/**", (route) => {
    const path = new URL(route.request().url()).pathname;
    const send = (body: unknown) => route.fulfill({ contentType: "application/json", body: JSON.stringify(body) });
    if (path.endsWith("/auth/login") || path.endsWith("/auth/refresh")) {
      return send({ access_token: "synthetic-token", token_type: "Bearer", expires_in: 3600 });
    }
    if (path.endsWith("/auth/me")) {
      return send({ tenant_id: tenant, user_id: "synthetic-user", role: "member" });
    }
    return send({ items: [], next_offset: null });
  });
}

test("untrusted login next cannot execute script-like navigation", async ({ page }) => {
  await mockLogin(page);
  await page.goto(`/login?next=${encodeURIComponent("javascript:sessionStorage.setItem('unsafe-next','executed')")}`);
  await signIn(page);
  await expect(page).toHaveURL(/\/dashboard$/);
  expect(await page.evaluate(() => sessionStorage.getItem("unsafe-next"))).toBeNull();
});

test("a safe internal next route survives login", async ({ page }) => {
  await mockLogin(page);
  await page.goto("/login?next=%2Fappointments");
  await signIn(page);
  await expect(page).toHaveURL(/\/appointments$/);
});

async function signIn(page: Page) {
  await page.getByLabel("Tenant ID").fill(tenant);
  await page.getByLabel("Email").fill("person@example.invalid");
  await page.getByLabel("Password").fill("synthetic-password");
  await page.getByRole("button", { name: "Entrar" }).click();
}
