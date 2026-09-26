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
  await page.getByLabel("ID del profesional").fill(user);
  await page.getByLabel("Relación clínica").selectOption("treating");
  await page.getByRole("button", { name: "Asignar profesional" }).click();
  await expect(page.getByText(user)).toBeVisible();
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
