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
  await expect(page.getByText("Cambios longitudinales fusionados")).toBeVisible();
  await expect(page.getByText("1 operación aprobada")).toBeVisible();
  await expect(page.getByText("Actor " + actor)).toHaveCount(0);
  await expect(page.locator("pre")).toBeHidden();
  await page.getByText("Detalle técnico").click();
  await expect(page.getByText(actor, { exact: true })).toBeVisible();
  await expect(page.locator("pre")).toContainText('"operation_count": 1');
});
