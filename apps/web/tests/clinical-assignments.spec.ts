import { expect, test, type Page } from "@playwright/test";

const tenant = "11111111-1111-4111-8111-111111111111";
const client = "22222222-2222-4222-8222-222222222222";
const user = "33333333-3333-4333-8333-333333333333";

async function setup(page: Page, role: "owner" | "member") {
  const assignments: Array<{ id: string; user_id: string; relationship: string; ends_at?: string }> = [];
  const requests: string[] = [];
  await page.addInitScript((t) => localStorage.setItem("sessionflow.auth.tenant", t), tenant);
  await page.route("**/api/v1/**", (route) => {
    const req = route.request();
    const path = new URL(req.url()).pathname.replace("/api/v1", "");
    requests.push(`${req.method()} ${path}`);
    const send = (body: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (path === "/auth/refresh") return send({ access_token: "synthetic", token_type: "Bearer", expires_in: 3600 });
    if (path === "/auth/me") return send({ tenant_id: tenant, user_id: "owner-user", role });
    if (path === "/clients" && req.method() === "GET") return send({ items: [{ id: client, fullname: "Paciente Ficticia Aurora", contact: "", notes_public: "", updated_at: "2026-09-01" }] });
    if (path === "/clients/archived") return send({ items: [] });
    if (path === `/clients/${client}/assignments/users` && role === "owner") return send({ items: [{ id: user, email: "therapist@tenant-a.local" }] });
    if (path === `/clients/${client}/assignments/assignment-1` && req.method() === "DELETE") {
      const body = req.postDataJSON();
      if (body.reason === "forbidden") return send({ error: "forbidden" }, 403);
      assignments[0].ends_at = "2026-09-28T00:00:00Z";
      return send({});
    }
    if (path === `/clients/${client}/assignments` && role === "owner") {
      if (req.method() === "GET") return send({ items: assignments });
      const body = req.postDataJSON();
      const assignment = { id: "assignment-1", user_id: body.user_id, relationship: body.relationship };
      assignments.push(assignment);
      return send(assignment, 201);
    }
    throw new Error(`Unexpected API request ${req.method()} ${path}`);
  });
  return { assignments, requests };
}

test("admin can grant an explicit clinical assignment; member cannot open management", async ({ page }) => {
  const owner = await setup(page, "owner");
  await page.goto("/clients");
  await page.getByRole("button", { name: /Paciente Ficticia Aurora/ }).click();
  await expect(page.getByRole("heading", { name: "Asignaciones clínicas" })).toBeVisible();
  await page.getByLabel("Profesional").selectOption({ label: "therapist@tenant-a.local" });
  await page.getByLabel("Relación clínica").selectOption("treating");
  await page.getByRole("button", { name: "Asignar profesional" }).click();
  await expect(page.getByText(/therapist@tenant-a.local · tratante/)).toBeVisible();
  expect(owner.assignments).toHaveLength(1);
  await page.close();
});

test("member UI does not request or expose assignment administration", async ({ page }) => {
  const state = await setup(page, "member");
  await page.goto("/clients");
  await page.getByRole("button", { name: /Paciente Ficticia Aurora/ }).click();
  await expect(page.getByRole("heading", { name: "Asignaciones clínicas" })).toHaveCount(0);
  expect(state.requests.some((path) => path.includes("/assignments"))).toBe(false);
});

test("ending an assignment uses a validated, cancellable dialog with visible errors", async ({ page }) => {
  const state = await setup(page, "owner");
  await page.goto("/clients");
  await page.getByRole("button", { name: /Paciente Ficticia Aurora/ }).click();
  await page.getByLabel("Profesional").selectOption({ label: "therapist@tenant-a.local" });
  await page.getByRole("button", { name: "Asignar profesional" }).click();
  const trigger = page.getByRole("button", { name: "Finalizar asignación" });
  await trigger.click();
  const dialog = page.getByRole("dialog", { name: "Finalizar asignación clínica" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByLabel("Motivo de finalización")).toBeFocused();
  await dialog.getByRole("button", { name: "Confirmar finalización" }).click();
  await expect(dialog.getByRole("alert")).toContainText("Escribe un motivo");
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(trigger).toBeFocused();
  await page.setViewportSize({ width: 390, height: 844 });
  await trigger.click();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await dialog.getByLabel("Motivo de finalización").fill("forbidden");
  await dialog.getByRole("button", { name: "Confirmar finalización" }).click();
  await expect(dialog.getByRole("alert")).toContainText("No tienes permiso");
  await dialog.getByLabel("Motivo de finalización").fill("Reasignación ficticia");
  await dialog.getByRole("button", { name: "Confirmar finalización" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText(/therapist@tenant-a.local · tratante · finalizada/)).toBeVisible();
  await expect(page.getByRole("heading", { name: "Asignaciones clínicas" })).toBeFocused();
  expect(state.assignments[0].ends_at).toBeTruthy();
});

test("ending an assignment cannot submit twice while the API request is pending", async ({ page }) => {
  await setup(page, "owner");
  let releaseRequest: (() => void) | undefined;
  const responseGate = new Promise<void>((resolve) => { releaseRequest = resolve; });
  let deletes = 0;
  await page.route(`**/api/v1/clients/${client}/assignments/assignment-1`, async (route) => {
    if (route.request().method() !== "DELETE") return route.fallback();
    deletes++;
    await responseGate;
    await route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
  });
  await page.goto("/clients");
  await page.getByRole("button", { name: /Paciente Ficticia Aurora/ }).click();
  await page.getByLabel("Profesional").selectOption({ label: "therapist@tenant-a.local" });
  await page.getByRole("button", { name: "Asignar profesional" }).click();
  await page.getByRole("button", { name: "Finalizar asignación" }).click();
  const dialog = page.getByRole("dialog", { name: "Finalizar asignación clínica" });
  await dialog.getByLabel("Motivo de finalización").fill("Cambio ficticio");
  await dialog.getByRole("button", { name: "Confirmar finalización" }).click();
  await expect(dialog.getByRole("button", { name: "Finalizando…" })).toBeDisabled();
  await expect(dialog.getByRole("button", { name: "Cancelar" })).toBeDisabled();
  expect(deletes).toBe(1);
  releaseRequest?.();
  await expect(dialog).toHaveCount(0);
  expect(deletes).toBe(1);
});
