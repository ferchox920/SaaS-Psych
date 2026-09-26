import { expect, test } from "@playwright/test";

test("client list follows visible pages without losing later patients", async ({ page }) => {
  const tenant = "11111111-1111-4111-8111-111111111111";
  const offsets: number[] = [];
  await page.addInitScript((id) => localStorage.setItem("sessionflow.auth.tenant", id), tenant);
  await page.route("**/api/v1/**", (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/v1", "");
    const send = (body: unknown) => route.fulfill({ contentType: "application/json", body: JSON.stringify(body) });
    if (path === "/auth/refresh") return send({ access_token: "synthetic", token_type: "Bearer", expires_in: 3600 });
    if (path === "/auth/me") return send({ tenant_id: tenant, user_id: "synthetic-user", role: "member" });
    if (path === "/clients") {
      const offset = Number(url.searchParams.get("offset"));
      offsets.push(offset);
      return send(offset === 0
        ? { items: [{ id: "22222222-2222-4222-8222-222222222222", fullname: "Paciente Ficticia Uno", contact: "", notes_public: "" }], next_offset: 100 }
        : { items: [{ id: "33333333-3333-4333-8333-333333333333", fullname: "Paciente Ficticia Dos", contact: "", notes_public: "" }], next_offset: null });
    }
    if (path === "/clients/archived") return send({ items: [], next_offset: null });
    return send({ items: [] });
  });
  await page.goto("/clients");
  await expect(page.getByRole("button", { name: /Paciente Ficticia Uno/ })).toBeVisible();
  await expect(page.getByRole("button", { name: /Paciente Ficticia Dos/ })).toBeVisible();
  expect(offsets).toEqual([0, 100]);
});
