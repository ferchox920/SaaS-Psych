import { expect, test } from "@playwright/test";

test("generic clinical workspace has no developer or personal copy", async ({ page }) => {
  const tenant = "11111111-1111-4111-8111-111111111111";
  await page.addInitScript((id) => localStorage.setItem("sessionflow.auth.tenant", id), tenant);
  await page.route("**/api/v1/**", (route) => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    const body = path === "/auth/refresh"
      ? { access_token: "synthetic", token_type: "Bearer", expires_in: 3600 }
      : path === "/auth/me"
        ? { user_id: "synthetic-user", tenant_id: tenant, role: "member" }
        : { items: [] };
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  });
  for (const path of ["/dashboard", "/clinical-workspace", "/clinical-formulation", "/clinical-review"]) {
    await page.goto(path);
    await expect(page.getByRole("button", { name: "Cerrar sesion" })).toBeVisible();
    await expect(page.locator("body")).not.toContainText(/Fernando|MVP cockpit|carpeta de feature|Professional workspace|Operation workspace/i);
    await expect(page.locator("header details div").first()).toBeHidden();
    await page.getByText("Detalle de cuenta").click();
    await expect(page.locator("header details div").first()).toContainText(tenant);
    await page.getByText("Detalle de cuenta").click();
    if (path.startsWith("/clinical-")) {
      const journey = page.getByRole("navigation", { name: "Recorrido clínico" });
      await expect(journey.getByRole("link", { name: "Pacientes y revisión clínica" })).toBeVisible();
      await expect(journey.getByRole("link", { name: "Formulación aprobada" })).toBeVisible();
    }
  }
});
