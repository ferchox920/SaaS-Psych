import { expect, test } from "@playwright/test";

test("a late 401 cannot start refresh after logout", async ({ page }) => {
  const tenant = "11111111-1111-4111-8111-111111111111";
  let releaseClients!: () => void;
  let releaseLogout!: () => void;
  const clientsGate = new Promise<void>((resolve) => { releaseClients = resolve; });
  const logoutGate = new Promise<void>((resolve) => { releaseLogout = resolve; });
  let refreshCalls = 0;
  let logoutStarted = false;
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  await page.addInitScript((id) => localStorage.setItem("sessionflow.auth.tenant", id), tenant);
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    const send = (body: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (path === "/auth/refresh") {
      refreshCalls++;
      return send({ access_token: `synthetic-${refreshCalls}`, token_type: "Bearer", expires_in: 3600 });
    }
    if (path === "/auth/me") return send({ user_id: "synthetic-user", tenant_id: tenant, role: "member" });
    if (path === "/auth/logout") {
      logoutStarted = true;
      await logoutGate;
      return route.fulfill({ status: 204 });
    }
    if (path === "/clients") {
      await clientsGate;
      return send({ error: { code: "unauthorized", message: "synthetic expiry" } }, 401);
    }
    return send({ items: [] });
  });

  await page.goto("/dashboard");
  await page.getByRole("button", { name: "Cerrar sesion" }).click();
  await expect.poll(() => logoutStarted).toBe(true);
  const clientsResponse = page.waitForResponse((response) => new URL(response.url()).pathname.endsWith("/clients"));
  releaseClients();
  await clientsResponse;
  releaseLogout();
  await expect(page).toHaveURL(/\/login/);
  await expect.poll(() => page.evaluate(() => localStorage.getItem("sessionflow.auth.tenant"))).toBeNull();
  await expect(page.getByRole("button", { name: "Cerrar sesion" })).toHaveCount(0);
  expect(refreshCalls).toBe(1);
  expect(pageErrors).toEqual([]);
});
