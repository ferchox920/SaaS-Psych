import { expect, test, type Page } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { toDateTimeLocalValue } from "../src/features/appointments/lib/appointment-datetime";

const tenant = "11111111-1111-1111-1111-111111111111";
const client = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11";
const screenshots = resolve(__dirname, "../../../docs/screenshots");

async function expectMobileFit(page: Page) {
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.setViewportSize({ width: 1440, height: 1000 });
}

test.skip(process.env.RUN_LIVE_DEMO !== "1", "Requires opt-in local demo API and seeded PostgreSQL");

test("local demo shows real fictional data and enforces clinical assignment", async ({ page }) => {
  if (process.env.UPDATE_DEMO_SCREENSHOTS === "1") await mkdir(screenshots, { recursive: true });
  const capture = async (name: string) => {
    if (process.env.UPDATE_DEMO_SCREENSHOTS === "1") await page.screenshot({ path: resolve(screenshots, name), fullPage: true });
  };
  await page.goto("/login");
  await expect(page.getByText("Demo simulada · solo datos ficticios")).toBeVisible();
  await expect(page.getByLabel("Tenant ID")).toHaveValue(tenant);
  await expect(page.getByLabel("Email")).toHaveValue("owner@tenant-a.local");
  await expect(page.getByLabel("Password")).toHaveValue("ChangeMe123!");
  await capture("01-login.png");
  await page.getByLabel("Tenant ID").fill(tenant);
  await page.getByLabel("Email").fill("therapist@tenant-a.local");
  await page.getByLabel("Password").fill("ChangeMe123!");
  await page.getByRole("button", { name: "Entrar" }).click();
  await expect(page).toHaveURL(/dashboard/);
  await expect(page.getByText("Demo simulada · datos ficticios · no es inferencia clínica")).toBeVisible();
  await expect(page.getByText("Cargando", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Sesion iniciada.")).toHaveCount(0);
  await capture("02-dashboard.png");
  await expectMobileFit(page);

  await page.goto("/clients");
  await expect(page.getByRole("button", { name: /Paciente Ficticia Aurora/ }).first()).toBeVisible();
  await capture("02a-patients.png");
  await expectMobileFit(page);
  await page.goto("/appointments");
  await expect(page.getByText("Videollamada ficticia")).toBeVisible();
  const slot = new Date();
  slot.setDate(slot.getDate() + 7);
  slot.setHours(10, 0, 0, 0);
  let appointment = "";
  for (let attempt = 0; attempt < 8 && !appointment; attempt++) {
    const startsAt = new Date(slot.getTime() + attempt * 2 * 60 * 60 * 1000);
    const endsAt = new Date(startsAt.getTime() + 60 * 60 * 1000);
    await page.getByLabel("Cliente").selectOption(client);
    await page.getByLabel("Inicio").fill(toDateTimeLocalValue(startsAt.toISOString()));
    await page.getByLabel("Fin").fill(toDateTimeLocalValue(endsAt.toISOString()));
    await page.getByLabel("Ubicacion").fill("Cita ficticia del recorrido");
    const createdResponse = page.waitForResponse((response) => response.url().endsWith("/api/v1/appointments") && response.request().method() === "POST");
    await page.getByRole("button", { name: "Crear cita" }).click();
    const response = await createdResponse;
    if (response.status() === 201) {
      appointment = (await response.json()).id as string;
    } else if (response.status() !== 409) {
      throw new Error(`Creating the fictional appointment returned HTTP ${response.status()}`);
    }
  }
  expect(appointment, "No free fictional demo appointment slot in the next eight attempts").not.toBe("");
  await expect(page.getByRole("link", { name: "Abrir sesión clínica de esta cita" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Abrir sesión clínica de esta cita" })).toHaveAttribute("href", `/clients/${client}/session?appointmentId=${appointment}`);
  await expect(page.getByText("Cita creada.", { exact: true })).toHaveCount(0);
  await capture("02b-appointments.png");
  await expectMobileFit(page);
  await page.getByRole("link", { name: "Abrir sesión clínica de esta cita" }).click();
  await expect(page).toHaveURL(new RegExp(`/clients/${client}/session\\?appointmentId=${appointment}`));
  await expect(page.getByRole("heading", { level: 2, name: "Sesión clínica · Paciente Ficticia Aurora" })).toBeVisible();
  await page.getByRole("button", { name: "Iniciar sesión clínica" }).click();
  await expect(page.getByText("Estado: in_progress", { exact: false })).toBeVisible();
  await expect(page.getByText(`Cita: ${appointment}`, { exact: false })).toBeVisible();
  const sessionId = await page.getByLabel("Seleccionar sesión").inputValue();
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByRole("button", { name: "Completar sesión", exact: true }).click();
  await expect(page.getByText("Estado: completed", { exact: false })).toBeVisible();
  await capture("02c-session.png");

  await page.goto(`/clients/${client}/clinical`);
  await expect(page.getByRole("heading", { name: "Revisión clínica y estado longitudinal" })).toBeVisible();
  await expect(page.getByText("Paciente: Paciente Ficticia Aurora", { exact: false })).toBeVisible();
  await page.getByRole("button", { name: "Reportes", exact: true }).click();
  await page.getByLabel("Sesión fuente").selectOption(sessionId);
  await expect(page.getByText("No hay informes todavía.", { exact: false })).toBeVisible();
  await page.getByLabel("Texto de la sesión").fill("Relato exclusivamente ficticio para verificar generación desde una sesión completada.");
  await page.getByRole("button", { name: "Generar borrador de informe" }).click();
  await expect(page.getByLabel("Texto de la sesión")).toHaveValue("");
  const draft = page.locator("article").filter({ has: page.getByRole("heading", { name: /Reporte v\d+ · draft/ }) }).first();
  await expect(draft).toBeVisible();
  await expect(draft.getByText(`Sesión fuente: ${sessionId}`)).toBeHidden();
  await draft.getByText("Procedencia técnica y transcripción fuente").click();
  await expect(draft.getByText(`Sesión fuente: ${sessionId}`)).toBeVisible();
  await draft.getByText("Procedencia técnica y transcripción fuente").click();
  await expect(draft.getByText("SIMULACIÓN:", { exact: false }).first()).toBeVisible();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await capture("03-clinical-review.png");
  await expectMobileFit(page);
  await draft.getByRole("button", { name: /Aprobar reporte v\d+/ }).click();
  await draft.getByRole("button", { name: "Confirmar aprobación de reporte" }).click();
  const approved = page.locator("article").filter({ has: page.getByRole("heading", { name: /Reporte v\d+ · approved/ }) }).first();
  await expect(approved).toBeVisible();
  await approved.getByRole("button", { name: "Solicitar interpretación longitudinal" }).click();
  const detail = page.getByRole("region", { name: "Detalle de propuesta" });
  await expect(detail).toBeVisible();
  await detail.getByRole("button", { name: "Aprobar operación 1" }).click();
  await expect(detail.getByText("1/1 revisadas", { exact: false })).toBeVisible();
  await detail.getByRole("button", { name: "Fusionar cambios aprobados" }).click();
  await detail.getByRole("button", { name: "Confirmar merge humano" }).click();
  await expect(detail.getByText("Fusionado una sola vez", { exact: false })).toBeVisible();

  await page.getByRole("button", { name: "Cerrar sesion" }).click();
  await page.goto("/login");
  await page.getByLabel("Tenant ID").fill(tenant);
  await page.getByLabel("Email").fill("owner@tenant-a.local");
  await page.getByLabel("Password").fill("ChangeMe123!");
  await page.getByRole("button", { name: "Entrar" }).click();
  await expect(page).toHaveURL(/dashboard/);
  await page.goto("/audit");
  await expect(page.getByRole("heading", { name: "Auditoría" })).toBeVisible();
  await expect(page.locator("main")).not.toContainText("apps/api");
  await page.getByLabel("Prefijo de acción").fill("clinical_diff.merged");
  await page.getByRole("button", { name: "Aplicar filtros" }).click();
  await expect(page.getByText("Cambios longitudinales fusionados").first()).toBeVisible();
  await page.getByText("Detalle técnico").first().click();
  await expect(page.getByText("clinical_diff.merged", { exact: true }).first()).toBeVisible();
  await expectMobileFit(page);
  await capture("04-audit.png");

  await page.getByRole("button", { name: "Cerrar sesion" }).click();
  await page.goto("/login");
  await page.getByLabel("Tenant ID").fill(tenant);
  await page.getByLabel("Email").fill("unassigned@tenant-a.local");
  await page.getByLabel("Password").fill("ChangeMe123!");
  await page.getByRole("button", { name: "Entrar" }).click();
  await expect(page).toHaveURL(/dashboard/);
  await page.goto(`/clients/${client}/clinical`);
  await expect(page.getByText("No autorizado para esta acción clínica", { exact: false })).toBeVisible();
  await expect(page.getByText("SIMULACIÓN:", { exact: false })).toHaveCount(0);

  await page.getByRole("button", { name: "Cerrar sesion" }).click();
  await page.goto("/login");
  await page.getByLabel("Tenant ID").fill("22222222-2222-2222-2222-222222222222");
  await page.getByLabel("Email").fill("member@tenant-b.local");
  await page.getByLabel("Password").fill("ChangeMe123!");
  await page.getByRole("button", { name: "Entrar" }).click();
  await expect(page).toHaveURL(/dashboard/);
  await page.goto(`/clients/${client}/clinical`);
  await expect(page.getByText("No autorizado para esta acción clínica", { exact: false })).toBeVisible();
});
