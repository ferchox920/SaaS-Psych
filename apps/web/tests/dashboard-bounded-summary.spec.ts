import { expect, test } from "@playwright/test";

test("dashboard bounds summary reads and never presents a partial page as a total", async ({ page }) => {
  const tenant = "11111111-1111-4111-8111-111111111111";
  const clientOffsets: number[] = [];
  const appointmentOffsets: number[] = [];
  await page.addInitScript((id) => localStorage.setItem("sessionflow.auth.tenant", id), tenant);
  await page.route("**/api/v1/**", (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/v1", "");
    const send = (body: unknown) => route.fulfill({ contentType: "application/json", body: JSON.stringify(body) });
    if (path === "/auth/refresh") return send({ access_token: "synthetic", token_type: "Bearer", expires_in: 3600 });
    if (path === "/auth/me") return send({ tenant_id: tenant, user_id: "synthetic-user", role: "member" });
    if (path === "/clients") {
      const offset = Number(url.searchParams.get("offset"));
      clientOffsets.push(offset);
      return send({ items: Array.from({ length: offset === 0 ? 100 : 1 }, (_, index) => ({ id: `client-${offset + index}` })), next_offset: offset === 0 ? 100 : null });
    }
    if (path === "/appointments") {
      const offset = Number(url.searchParams.get("offset"));
      appointmentOffsets.push(offset);
      return send({ items: Array.from({ length: offset === 0 ? 100 : 1 }, (_, index) => ({ id: `appointment-${offset + index}` })), next_offset: offset === 0 ? 100 : null });
    }
    return send({ items: [] });
  });

  await page.goto("/dashboard");
  await expect(page.getByText("Pacientes", { exact: true }).locator("..").getByRole("heading")).toHaveText("100+");
  await expect(page.getByText("Próximas sesiones", { exact: true }).locator("..").getByRole("heading")).toHaveText("100+");
  expect(clientOffsets).toEqual([0]);
  expect(appointmentOffsets).toEqual([0]);
});

test("dashboard reports failed counts as unavailable while exact small counts stay exact", async ({ page }) => {
  const tenant = "11111111-1111-4111-8111-111111111111";
  await page.addInitScript((id) => localStorage.setItem("sessionflow.auth.tenant", id), tenant);
  await page.route("**/api/v1/**", (route) => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    const send = (body: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (path === "/auth/refresh") return send({ access_token: "synthetic", token_type: "Bearer", expires_in: 3600 });
    if (path === "/auth/me") return send({ tenant_id: tenant, user_id: "synthetic-user", role: "member" });
    if (path === "/clients") return send({ error: { code: "service_unavailable" } }, 503);
    if (path === "/appointments") return send({ items: [{ id: "appointment-1" }, { id: "appointment-2" }], next_offset: null });
    return send({ items: [] });
  });

  await page.goto("/dashboard");
  await expect(page.getByText("Pacientes", { exact: true }).locator("..").getByRole("heading")).toHaveText("No disponible", { timeout: 15000 });
  await expect(page.getByText("Próximas sesiones", { exact: true }).locator("..").getByRole("heading")).toHaveText("2");
});
