import { expect, test } from "@playwright/test";

test("audit explains a human merge and keeps raw provenance on demand", async ({ page }) => {
  const tenant = "11111111-1111-4111-8111-111111111111";
  const actor = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
  await page.addInitScript((id) => localStorage.setItem("sessionflow.auth.tenant", id), tenant);
  await page.route("**/api/v1/**", (route) => {
    const path = new URL(route.request().url()).pathname;
    const send = (body: unknown) => route.fulfill({ contentType: "application/json", body: JSON.stringify(body) });
    if (path.endsWith("/auth/refresh")) return send({ access_token: "synthetic", token_type: "Bearer", expires_in: 3600 });
    if (path.endsWith("/auth/me")) return send({ tenant_id: tenant, user_id: actor, role: "owner" });
    if (path.endsWith("/audit")) return send({ items: [{ id: "event-1", tenant_id: tenant, actor_user_id: actor, action: "clinical_diff.merged", entity: "clinical_diff", entity_id: "diff-1", metadata: { operation_count: 1, from_state_version: 2, to_state_version: 3 }, created_at: "2026-09-26T12:00:00Z" }], pagination: { limit: 20, offset: 0, count: 1, total_count: 1 } });
    throw new Error(`Unexpected request ${path}`);
  });
  await page.goto("/audit");
  await expect(page.getByRole("heading", { name: "Cambios longitudinales fusionados" })).toBeVisible();
  await expect(page.getByText("1 operación aprobada")).toBeVisible();
  await expect(page.getByText("Actor " + actor)).toHaveCount(0);
  await expect(page.locator("pre")).toBeHidden();
  await page.getByText("Detalle técnico").click();
  await expect(page.getByText(actor, { exact: true })).toBeVisible();
  await expect(page.locator("pre")).toContainText('"operation_count": 1');
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
});

test("audit filters and known events use Spanish labels while preserving API values", async ({ page }) => {
  const requested: URL[] = [];
  await page.addInitScript(() => localStorage.setItem("sessionflow.auth.tenant", "11111111-1111-4111-8111-111111111111"));
  await page.route("**/api/v1/**", (route) => {
    const url = new URL(route.request().url());
    const send = (body: unknown) => route.fulfill({ contentType: "application/json", body: JSON.stringify(body) });
    if (url.pathname.endsWith("/auth/refresh")) return send({ access_token: "synthetic", token_type: "Bearer", expires_in: 3600 });
    if (url.pathname.endsWith("/auth/me")) return send({ tenant_id: "11111111-1111-4111-8111-111111111111", user_id: "owner", role: "owner" });
    if (url.pathname.endsWith("/audit")) {
      requested.push(url);
      return send({ items: [
        { id: "1", tenant_id: "tenant", action: "client.create", entity: "client", metadata: {}, created_at: "2026-09-26T12:00:00Z" },
        { id: "2", tenant_id: "tenant", action: "session_note.sign", entity: "session_note", metadata: {}, created_at: "2026-09-26T12:00:00Z" },
        { id: "3", tenant_id: "tenant", action: "unknown.operation", entity: "unknown", metadata: {}, created_at: "2026-09-26T12:00:00Z" },
      ], pagination: { limit: 20, offset: 0, count: 3, total_count: 3 } });
    }
    throw new Error(`Unexpected request ${url}`);
  });
  await page.goto("/audit");
  await expect(page.getByText("Paciente creado")).toBeVisible();
  await expect(page.getByText("Nota de sesión firmada")).toBeVisible();
  await expect(page.getByText("Otra acción registrada")).toBeVisible();
  await expect(page.getByText("Paciente", { exact: true }).last()).toBeVisible();
  await page.getByLabel("Tipo de cambio").selectOption("clinical_diff.merged");
  await page.getByLabel("Entidad").selectOption("session_note");
  await page.getByLabel("Orden").selectOption("asc");
  await page.getByRole("button", { name: "Aplicar filtros" }).click();
  await expect.poll(() => requested.at(-1)?.searchParams.get("order")).toBe("asc");
  expect(requested.at(-1)?.searchParams.get("action_prefix")).toBe("clinical_diff.merged");
  expect(requested.at(-1)?.searchParams.get("entity")).toBe("session_note");
  await page.getByLabel("Orden").selectOption("desc");
  await page.getByRole("button", { name: "Aplicar filtros" }).click();
  await expect.poll(() => requested.at(-1)?.searchParams.get("order")).toBe("desc");
  await page.getByLabel("Tipo de cambio").selectOption("__custom__");
  await page.getByLabel("Código de acción").fill("unknown.operation");
  await page.getByLabel("Entidad").selectOption("__custom__");
  await page.getByLabel("Código de entidad").fill("unknown");
  await page.getByRole("button", { name: "Aplicar filtros" }).click();
  await expect.poll(() => requested.at(-1)?.searchParams.get("action_prefix")).toBe("unknown.operation");
  expect(requested.at(-1)?.searchParams.get("entity")).toBe("unknown");
  await page.getByRole("button", { name: "Limpiar" }).click();
  await expect(page.getByLabel("Código de acción")).toHaveCount(0);
  await expect(page.getByLabel("Código de entidad")).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
});
