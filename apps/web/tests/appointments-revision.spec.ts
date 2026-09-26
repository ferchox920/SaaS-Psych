import { expect, test } from "@playwright/test";

test("stale appointment revision shows conflict and reload uses current revision", async ({ page }) => {
  const tenant = "11111111-1111-4111-8111-111111111111";
  const client = "22222222-2222-4222-8222-222222222222";
  const appointmentId = "33333333-3333-4333-8333-333333333333";
  const start = new Date(Date.now() + 24 * 60 * 60 * 1000);
  start.setHours(10, 0, 0, 0);
  let current = {
    id: appointmentId,
    tenant_id: tenant,
    client_id: client,
    starts_at: start.toISOString(),
    ends_at: new Date(start.getTime() + 60 * 60 * 1000).toISOString(),
    status: "scheduled",
    revision: 1,
    location: "Sala ficticia",
    created_at: start.toISOString(),
    updated_at: start.toISOString(),
  };
  const attempts: number[] = [];
  let failedOnce = false;
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.stack ?? error.message));
  await page.addInitScript((id) => localStorage.setItem("sessionflow.auth.tenant", id), tenant);
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const send = (body: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
    if (path === "/auth/refresh") return send({ access_token: "synthetic-token", token_type: "Bearer", expires_in: 3600 });
    if (path === "/auth/me") return send({ user_id: "synthetic-user", tenant_id: tenant, role: "member" });
    expect(request.headers()["x-tenant-id"]).toBe(tenant);
    if (path === "/clients") return send({ items: [{ id: client, fullname: "Paciente ficticio" }] });
    if (path === "/appointments" && request.method() === "GET") return send({ items: [current] });
    if (path === `/appointments/${appointmentId}` && request.method() === "PUT") {
      const body = request.postDataJSON() as { expected_revision: number; location: string };
      attempts.push(body.expected_revision);
      if (!failedOnce) {
        failedOnce = true;
        current = { ...current, revision: 2, location: "Otro cambio ficticio" };
        return send({ error: { code: "conflict", message: "synthetic conflict" } }, 409);
      }
      if (body.expected_revision !== current.revision) return send({ error: { code: "conflict" } }, 409);
      current = { ...current, revision: current.revision + 1, location: body.location };
      return send(current);
    }
    return send({ items: [] });
  });

  await page.goto("/appointments");
  const activeNav = page.locator('nav a[href="/appointments"]');
  const colors = await activeNav.evaluate((element) => {
    const label = element.querySelector("span.block.text-sm");
    if (!label) throw new Error("Active navigation label is missing");
    const rgb = (color: string) => {
      const canvas = document.createElement("canvas");
      const context = canvas.getContext("2d");
      if (!context) throw new Error("Canvas unavailable");
      context.fillStyle = color;
      context.fillRect(0, 0, 1, 1);
      return Array.from(context.getImageData(0, 0, 1, 1).data.slice(0, 3));
    };
    return { foreground: rgb(getComputedStyle(label).color), background: rgb(getComputedStyle(element).backgroundColor) };
  });
  expect(contrastRatio(colors.foreground, colors.background), JSON.stringify(colors)).toBeGreaterThanOrEqual(4.5);
  const startInput = await page.getByLabel("Inicio").boundingBox();
  const endInput = await page.getByLabel("Fin").boundingBox();
  expect(startInput?.width).toBeGreaterThanOrEqual(180);
  expect(endInput?.width).toBeGreaterThanOrEqual(180);
  await page.getByRole("button", { name: /Paciente ficticio/ }).click();
  await expect(page.getByRole("link", { name: "Abrir sesión clínica de esta cita" })).toHaveAttribute(
    "href",
    `/clients/${client}/session?appointmentId=${appointmentId}`,
  );
  await page.getByLabel("Ubicacion").fill("Sala revisada");
  await page.getByRole("button", { name: "Guardar cambios" }).click();
  await expect(page.getByRole("button", { name: "Recargar agenda" })).toBeVisible();
  expect(attempts).toEqual([1]);

  await page.getByRole("button", { name: "Recargar agenda" }).click();
  await expect(page.getByLabel("Ubicacion")).toHaveValue("Otro cambio ficticio");
  await page.getByLabel("Ubicacion").fill("Sala revisada");
  await page.getByRole("button", { name: "Guardar cambios" }).click();
  await expect(page.getByText("Cita actualizada.")).toBeVisible();
  expect(attempts).toEqual([1, 2]);
  expect(current.revision).toBe(3);
  expect(pageErrors).toEqual([]);
});

function contrastRatio(foreground: number[], background: number[]): number {
  const luminance = (channels: number[]) => {
    const [red, green, blue] = channels.map((value) => {
      const srgb = value / 255;
      return srgb <= 0.04045 ? srgb / 12.92 : ((srgb + 0.055) / 1.055) ** 2.4;
    });
    return red * 0.2126 + green * 0.7152 + blue * 0.0722;
  };
  const lighter = Math.max(luminance(foreground), luminance(background));
  const darker = Math.min(luminance(foreground), luminance(background));
  return (lighter + 0.05) / (darker + 0.05);
}
