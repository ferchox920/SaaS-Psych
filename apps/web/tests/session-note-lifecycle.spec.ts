import { expect, test } from "@playwright/test";

test("note signing, addendum and version history remain explicit", async ({ page }) => {
  const tenant = "11111111-1111-4111-8111-111111111111";
  const appointment = "22222222-2222-4222-8222-222222222222";
  const client = "33333333-3333-4333-8333-333333333333";
  const note = "44444444-4444-4444-8444-444444444444";
  const author = "55555555-5555-4555-8555-555555555555";
  let current = { id: note, tenant_id: tenant, appointment_id: appointment, author_user_id: author, body: "Nota inicial ficticia", is_private: true, status: "draft", current_version: 1, created_at: "2026-09-01", updated_at: "2026-09-01" };
  const versions = [{ id: "v1", note_id: note, version: 1, body: current.body, is_private: true, change_kind: "create", change_reason: "", actor_user_id: author, created_at: "2026-09-01" }];
  await page.addInitScript((t) => localStorage.setItem("sessionflow.auth.tenant", t), tenant);
  await page.route("**/api/v1/**", (route) => {
    const req = route.request();
    const path = new URL(req.url()).pathname.replace("/api/v1", "");
    const send = (body: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (path === "/auth/refresh") return send({ access_token: "synthetic", token_type: "Bearer", expires_in: 3600 });
    if (path === "/auth/me") return send({ tenant_id: tenant, user_id: author, role: "member" });
    if (path === "/clients") return send({ items: [{ id: client, fullname: "Paciente ficticia" }] });
    if (path === "/appointments") return send({ items: [{ id: appointment, client_id: client, starts_at: "2026-09-22T12:00:00Z", ends_at: "2026-09-22T13:00:00Z", status: "scheduled", location: "" }] });
    if (path === `/appointments/${appointment}/notes`) return send({ items: [current] });
    if (path === `/notes/${note}/versions`) return send({ items: versions });
    if (path === `/notes/${note}/sign`) {
      current = { ...current, status: "signed", current_version: 2 };
      versions.push({ ...versions[0], id: "v2", version: 2, change_kind: "sign" });
      return send(current);
    }
    if (path === `/notes/${note}/addenda`) {
      const body = req.postDataJSON();
      current = { ...current, body: body.body, current_version: 3 };
      versions.push({ ...versions[0], id: "v3", version: 3, body: body.body, change_kind: "addendum", change_reason: body.reason });
      return send(current);
    }
    throw new Error(`Unexpected ${req.method()} ${path}`);
  });
  await page.goto("/session-notes");
  await page.getByRole("button", { name: /Nota inicial ficticia/ }).click();
  await page.getByRole("button", { name: "Firmar nota" }).click();
  await expect(page.getByText("Estado: firmada", { exact: false })).toBeVisible();
  await expect(page.getByLabel("Contenido")).toBeDisabled();
  await page.getByLabel("Texto de adenda").fill("Adenda exclusivamente ficticia");
  await page.getByLabel("Motivo de adenda").fill("Corrección aclaratoria");
  await page.getByRole("button", { name: "Guardar adenda" }).click();
  await page.getByRole("button", { name: "Ver historial de versiones" }).click();
  await expect(page.getByText("Corrección aclaratoria")).toBeVisible();
});
