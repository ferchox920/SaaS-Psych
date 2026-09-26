import { test, expect, type Page } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  scopes,
  type Consent,
  type ClinicalSession,
  type Artifact,
  type Job,
  type Transcript,
  type Scope,
} from "../src/features/session-workspace/model";

const tenant = "11111111-1111-4111-8111-111111111111",
  client = "22222222-2222-4222-8222-222222222222",
  sid = "33333333-3333-4333-8333-333333333333";
const timestamp = "2026-01-01T12:00:00Z";
const session: ClinicalSession = {
  id: sid,
  client_id: client,
  status: "in_progress",
  started_at: timestamp,
};
const transcript: Transcript = {
  id: "55555555-5555-4555-8555-555555555555",
  version: 1,
  origin: "machine",
  status: "available",
  text: "Texto sintético original",
  segments: [{ start: 0, end: 2, text: "Texto sintético original" }],
  created_at: timestamp,
  source_artifact_id: "audio-synthetic",
  engine: "faster-whisper",
  model: "synthetic-only",
};
const job: Job = {
  id: "66666666-6666-4666-8666-666666666666",
  session_id: sid,
  job_type: "transcribe_session_audio",
  status: "queued",
  attempt: 0,
  max_attempts: 3,
};
function consent(scope: Scope): Consent {
  return {
    id: `grant-${scope}`,
    scope,
    status: "granted",
    definition_version: 1,
    granted_at: timestamp,
    effective_from: timestamp,
  };
}

test("pending poll aborted by cancel cannot overwrite confirmed cancellation", async ({
  page,
}) => {
  const s = await mock(page, true);
  s.jobs = [{ ...job, status: "running" }];
  await open(page);
  await expect(
    page.getByRole("heading", { name: "Transcripción · En ejecución" }),
  ).toBeVisible();
  let release!: () => void;
  let held = false;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route(`**/api/v1/clinical-jobs/${job.id}`, async (route) => {
    if (held) return route.fallback();
    held = true;
    await pending;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ...job, status: "succeeded" }),
    });
  });
  await page
    .getByRole("button", { name: "Actualizar estado", exact: true })
    .click();
  await expect.poll(() => held).toBe(true);
  await page.getByRole("button", { name: "Cancelar trabajo" }).click();
  await expect(
    page.getByRole("heading", { name: "Transcripción · Cancelado" }),
  ).toBeVisible();
  release();
  await page.waitForTimeout(100);
  await expect(
    page.getByRole("heading", { name: "Transcripción · Cancelado" }),
  ).toBeVisible();
  expect(s.errors).toEqual([]);
});

test("session workspace follows a second bounded session page", async ({ page }) => {
  await mock(page, true);
  const later = { ...session, id: "77777777-7777-4777-8777-777777777777", started_at: "2026-01-02T12:00:00Z" };
  const offsets: number[] = [];
  await page.route("**/api/v1/clients/*/clinical-sessions?*", (route) => {
    const offset = Number(new URL(route.request().url()).searchParams.get("offset"));
    offsets.push(offset);
    return route.fulfill({ contentType: "application/json", body: JSON.stringify(offset === 0
      ? { items: [session], can_write: true, next_offset: 100 }
      : { items: [later], can_write: true, next_offset: null }) });
  });
  await open(page);
  const selector = page.getByLabel("Seleccionar sesión");
  await expect(selector.locator("option")).toHaveCount(2);
  await selector.selectOption(later.id);
  await expect(selector).toHaveValue(later.id);
  expect(offsets.slice(0, 2)).toEqual([0, 100]);
});

test("microphone denial safely offers upload alternative", async ({ page }) => {
  const s = await mock(page, true);
  s.consents = [consent("AUDIO_RECORDING")];
  await page.addInitScript(() =>
    Object.defineProperty(navigator, "mediaDevices", {
      value: {
        getUserMedia: async () => {
          throw new Error("SECRET permission diagnostic");
        },
      },
    }),
  );
  await open(page);
  await page.getByRole("button", { name: "Grabar con micrófono" }).click();
  await expect(page.locator("p[role=alert]")).toContainText(
    "No se pudo habilitar el micrófono",
  );
  await expect(page.getByLabel("O subir un archivo local")).toBeEnabled();
  await expect(page.locator("body")).not.toContainText("SECRET");
  expect(s.requests).toHaveLength(0);
});

test("server rejects small upload with 413 without reporting uploaded or transcript ready", async ({
  page,
}) => {
  const s = await mock(page, true);
  s.consents = [consent("AUDIO_RECORDING")];
  s.deny = 413;
  await open(page);
  await page
    .getByLabel("O subir un archivo local")
    .setInputFiles({
      name: "synthetic.wav",
      mimeType: "audio/wav",
      buffer: Buffer.from("synthetic"),
    });
  await page.getByRole("button", { name: "Subir audio", exact: true }).click();
  await expect(
    page.locator("p[role=alert]").filter({ hasText: "25 MiB" }),
  ).toBeVisible();
  await expect(
    page.getByText("Audio subido. Todavía no implica una transcripción."),
  ).toHaveCount(0);
  expect(s.artifacts).toHaveLength(0);
});

test("ASR validation rejection preserves original version", async ({
  page,
}) => {
  const s = await mock(page, true);
  s.transcripts = [transcript];
  s.deny = 422;
  await open(page);
  await page
    .getByRole("button", { name: "Corregir reconocimiento ASR" })
    .click();
  await page
    .getByLabel("Segmento 1", { exact: false })
    .fill("Corrección sintética rechazada");
  await page.getByRole("button", { name: "Guardar nueva versión" }).click();
  await expect(page.locator("p[role=alert]")).toContainText("no es válido");
  await expect(page.getByLabel("Historial de versiones")).toHaveValue(
    transcript.id,
  );
  expect(s.transcripts).toHaveLength(1);
});

test("missing resource fails closed and never exposes cached source", async ({
  page,
}) => {
  const s = await mock(page, true);
  await page.route("**/api/v1/clients/*/clinical-sessions?*", (route) =>
    route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({ error: { message: "SECRET foreign ID" } }),
    }),
  );
  await page.goto(`/clients/${client}/session`);
  await expect(page.locator("p[role=alert]")).toContainText(
    "Recurso no disponible",
  );
  await expect(
    page.getByRole("button", { name: "Grabar con micrófono" }),
  ).toHaveCount(0);
  expect(s.requests).toHaveLength(0);
});

test("expired authentication uses existing refresh and clears the workspace", async ({
  page,
}) => {
  const s = await mock(page, true);
  await open(page);
  await page.route("**/api/v1/auth/refresh", (route) =>
    route.fulfill({ status: 401, contentType: "application/json", body: "{}" }),
  );
  await page.route("**/api/v1/clients/*/clinical-sessions?*", (route) =>
    route.fulfill({ status: 401, contentType: "application/json", body: "{}" }),
  );
  // Reload always issues the protected read. Background refetch may also hit
  // the 401 first, so the assertion must not depend on a transient button.
  await page.reload();
  await expect(
    page.getByRole("heading", { name: /Sesión clínica ·/ }),
  ).toHaveCount(0);
  await expect
    .poll(() =>
      page.evaluate(() => localStorage.getItem("sessionflow.auth.tenant")),
    )
    .toBeNull();
  expect(s.requests.filter((r) => !r.path.includes("auth"))).toHaveLength(0);
});
async function mock(page: Page, initial = false) {
  const state = {
    sessions: initial ? [{ ...session }] : ([] as ClinicalSession[]),
    consents: [] as Consent[],
    artifacts: [] as Artifact[],
    transcripts: [] as Transcript[],
    jobs: [] as Job[],
    canWrite: true,
    deny: 0,
    denyRead: false,
    jobReads: 0,
    requests: [] as { path: string; body: unknown; type: string }[],
    unknown: [] as string[],
    errors: [] as string[],
  };
  page.on("pageerror", (e) => state.errors.push(e.message));
  await page.addInitScript(
    (t) => localStorage.setItem("sessionflow.auth.tenant", t),
    tenant,
  );
  await page.route("**/api/v1/**", async (route) => {
    const r = route.request(),
      path = new URL(r.url()).pathname.replace("/api/v1", "");
    const send = (data: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(data),
      });
    if (path === "/auth/refresh")
      return send({
        access_token: "synthetic-token",
        token_type: "Bearer",
        expires_in: 3600,
      });
    if (path === "/auth/me")
      return send({
        tenant_id: tenant,
        user_id: "synthetic-user",
        role: "member",
      });
    if (path === "/auth/logout") return send({});
    expect(r.headers()["x-tenant-id"]).toBe(tenant);
    expect(r.headers()["authorization"]).toBe("Bearer synthetic-token");
    if (r.method() === "GET") {
      if (state.denyRead)
        return send(
          {
            error: { code: "forbidden", message: "SECRET clinical diagnostic" },
          },
          403,
        );
      if (path === `/clients/${client}`)
        return send({ fullname: "Paciente ficticio de aceptación" });
      if (path.endsWith("/clinical-sessions"))
        return send({ items: state.sessions, can_write: state.canWrite });
      if (path.endsWith("/consents")) return send({ items: state.consents });
      if (path.endsWith("/artifacts")) return send({ items: state.artifacts });
      if (path.endsWith("/transcripts"))
        return send({ items: state.transcripts });
      if (path.endsWith("/jobs")) return send({ items: state.jobs });
      if (path.startsWith("/clinical-jobs/")) {
        state.jobReads++;
        return send(state.jobs.find((j) => path.endsWith(j.id)));
      }
    } else if (r.method() === "POST") {
      const isAudio = path.endsWith("/audio");
      const body = isAudio
        ? r.postDataBuffer()?.length
        : r.postData()
          ? r.postDataJSON()
          : undefined;
      state.requests.push({ path, body, type: r.headers()["content-type"] });
      if (state.deny)
        return send(
          {
            error: {
              code: state.deny === 503 ? "provider_unavailable" : "conflict",
              message: "SECRET SQL prompt",
            },
          },
          state.deny,
        );
      if (path === "/clinical-sessions") {
        const created = { ...session, appointment_id: body.appointment_id };
        state.sessions.push(created);
        return send(created, 201);
      }
      if (path.endsWith("/complete")) {
        state.sessions[0] = {
          ...session,
          status: "completed",
          ended_at: timestamp,
        };
        return send(state.sessions[0]);
      }
      if (path.endsWith("/revoke")) {
        const c = state.consents.find((c) => path.includes(c.id))!;
        c.status = "revoked";
        c.revoked_at = timestamp;
        return send(c);
      }
      if (path.endsWith("/consents")) {
        const c = consent(body.scope);
        state.consents.push(c);
        return send(c, 201);
      }
      if (isAudio) {
        const a: Artifact = {
          id: "audio-synthetic",
          status: "available",
          created_at: timestamp,
          size_bytes: body,
          retention_until: "2099-01-01T00:00:00Z",
        };
        state.artifacts.push(a);
        return send(a, 201);
      }
      if (path.endsWith("/transcriptions")) {
        state.jobs.push({ ...job });
        return send(state.jobs.at(-1), 202);
      }
      if (path.endsWith("/revisions")) {
        const v: Transcript = {
          ...transcript,
          id: "77777777-7777-4777-8777-777777777777",
          version: 2,
          origin: "human_asr_correction",
          parent_version_id: transcript.id,
          text: body.text,
          segments: body.segments,
        };
        state.transcripts.push(v);
        return send(v, 201);
      }
      if (path.endsWith("/analysis-jobs")) {
        const j: Job = {
          ...job,
          id: "88888888-8888-4888-8888-888888888888",
          job_type: "analyze_session",
          transcript_version_id: body.transcript_version_id,
        };
        state.jobs.push(j);
        return send(j, 202);
      }
      if (path.endsWith("/cancel")) {
        const j = state.jobs.find((j) => path.includes(j.id))!;
        j.status = "cancelled";
        return send(j);
      }
      if (path.endsWith("/retry"))
        return send(state.jobs.find((j) => path.includes(j.id)));
    }
    state.unknown.push(`${r.method()} ${path}`);
    return send({}, 404);
  });
  return state;
}

test("starting from an appointment preserves its link on the clinical session", async ({ page }) => {
  const appointmentId = "99999999-9999-4999-8999-999999999999";
  const state = await mock(page);
  await page.goto(`/clients/${client}/session?appointmentId=${appointmentId}`);
  await expect(page.getByText("Cita seleccionada:", { exact: false })).toContainText(appointmentId);
  const review = await page.getByRole("link", { name: "Abrir revisión clínica y estado longitudinal" }).boundingBox();
  const patients = await page.getByRole("link", { name: "Volver a pacientes" }).boundingBox();
  expect(review && patients && patients.x - (review.x + review.width)).toBeGreaterThanOrEqual(8);
  await expect(page.getByRole("link", { name: "Abrir revisión clínica y estado longitudinal" })).toHaveCSS("text-decoration-line", "underline");
  await page.getByRole("button", { name: "Iniciar sesión clínica" }).click();
  await expect.poll(() => state.requests.find((request) => request.path === "/clinical-sessions")?.body).toMatchObject({
    client_id: client,
    appointment_id: appointmentId,
  });
  expect(state.errors).toEqual([]);
});

test("an appointment with a completed session cannot start another non-void session", async ({ page }) => {
  const appointmentId = "99999999-9999-4999-8999-999999999999";
  const state = await mock(page);
  state.sessions = [{ ...session, status: "completed", appointment_id: appointmentId, ended_at: timestamp }];
  await page.goto(`/clients/${client}/session?appointmentId=${appointmentId}`);
  await expect(page.getByText("Esta cita ya tiene una sesión clínica", { exact: false })).toBeVisible();
  await expect(page.getByRole("button", { name: "Iniciar sesión clínica" })).toBeDisabled();
  expect(state.requests.filter((request) => request.path === "/clinical-sessions")).toHaveLength(0);
});

async function open(page: Page) {
  await page.goto(`/clients/${client}/session`);
  await expect(
    page.getByRole("heading", { name: /Sesión clínica ·/ }),
  ).toBeVisible();
}
async function grant(page: Page, scope: Scope) {
  const section = page.locator("section").filter({
    has: page.getByRole("heading", { name: scopes[scope], exact: true }),
  });
  await section
    .getByRole("button", { name: "Registrar consentimiento" })
    .click();
  await expect(
    page.getByRole("group", { name: "Confirmar cambio de consentimiento" }),
  ).toContainText(scopes[scope]);
  await page
    .getByRole("button", { name: "Confirmar registro", exact: true })
    .click();
  await expect(section).toContainText("Otorgado");
}

test("synthetic full flow: session, scopes, raw upload, jobs, versioned ASR, explicit analysis and draft", async ({
  page,
}) => {
  const s = await mock(page);
  await open(page);
  await page.getByRole("button", { name: "Iniciar sesión clínica" }).click();
  await expect(
    page.getByText("Estado: in_progress", { exact: false }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Grabar con micrófono" }),
  ).toHaveCount(0);
  await grant(page, "AUDIO_RECORDING");
  await page.getByLabel("O subir un archivo local").setInputFiles({
    name: "synthetic.wav",
    mimeType: "audio/wav",
    buffer: Buffer.from("RIFF synthetic acceptance audio"),
  });
  await page.getByRole("button", { name: "Subir audio", exact: true }).click();
  await expect(
    page.getByText("Audio subido. Todavía no implica una transcripción."),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Solicitar transcripción" }),
  ).toBeDisabled();
  await grant(page, "LOCAL_TRANSCRIPTION");
  await page.getByRole("button", { name: "Solicitar transcripción" }).click();
  await expect(
    page.getByRole("heading", { name: "Transcripción · En cola" }),
  ).toBeVisible();
  s.jobs[0].status = "running";
  await page
    .getByRole("button", { name: "Actualizar estado", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Transcripción · En ejecución" }),
  ).toBeVisible();
  s.jobs[0].status = "succeeded";
  s.jobs[0].attempt = 1;
  s.transcripts.push({ ...transcript });
  await page
    .getByRole("button", { name: "Actualizar estado", exact: true })
    .click();
  await expect(
    page.getByText("Texto sintético original", { exact: false }),
  ).toBeVisible();
  await expect(page.getByText(/No es un hecho clínico validado/)).toBeVisible();
  await page
    .getByRole("button", { name: "Corregir reconocimiento ASR" })
    .click();
  await page
    .getByLabel("Segmento 1", { exact: false })
    .fill("Texto sintético corregido");
  await page.getByRole("button", { name: "Guardar nueva versión" }).click();
  await expect(page.getByLabel("Historial de versiones")).toHaveValue(
    "77777777-7777-4777-8777-777777777777",
  );
  await page.getByLabel("Historial de versiones").selectOption(transcript.id);
  await expect(
    page.getByText("Texto sintético original", { exact: false }),
  ).toBeVisible();
  page.on("dialog", (dialog) => dialog.accept());
  await page
    .getByRole("button", { name: "Completar sesión", exact: true })
    .click();
  await grant(page, "LOCAL_AI_PROCESSING");
  await page
    .getByRole("button", { name: "Analizar esta transcripción · v1" })
    .click();
  await expect(
    page.getByRole("heading", { name: "Análisis · En cola" }),
  ).toBeVisible();
  expect(s.jobs[1].transcript_version_id).toBe(transcript.id); // clicked historical v1, NOT latest v2
  s.jobs[1].status = "succeeded";
  await page
    .getByRole("button", { name: "Actualizar estado", exact: true })
    .click();
  await expect(
    page.getByText("SessionReport borrador disponible. No está aprobado."),
  ).toBeVisible();
  expect(s.requests.find((r) => r.path.endsWith("/audio"))?.type).toBe(
    "audio/wav",
  );
  expect(s.requests.filter((r) => r.path.endsWith("/revisions"))).toHaveLength(
    1,
  );
  expect(s.unknown).toEqual([]);
  expect(s.errors).toEqual([]);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: join(tmpdir(), "stage3a-workspace-mobile.png"),
    fullPage: true,
  });
});

test("supervisor, four scopes, failed job, readonly version history", async ({
  page,
}) => {
  const s = await mock(page, true);
  s.canWrite = false;
  s.transcripts = [transcript];
  s.jobs = [
    { ...job, status: "failed", attempt: 3, error_code: "invalid_output" },
  ];
  await open(page);
  for (const label of Object.values(scopes))
    await expect(
      page.getByRole("heading", { name: label, exact: true }),
    ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Completar sesión", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Registrar consentimiento" }).first(),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Corregir reconocimiento ASR" }),
  ).toBeDisabled();
  await expect(
    page.getByRole("heading", { name: "Transcripción · Fallido" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Adelantar reintento" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Cancelar trabajo" }),
  ).toHaveCount(0);
  const reads = s.jobReads;
  await page.waitForTimeout(4500);
  expect(s.jobReads).toBe(reads);
  expect(s.requests).toEqual([]);
});

test("retry budget, 409, cancellation and stale success response", async ({
  page,
}) => {
  const s = await mock(page, true);
  s.jobs = [{ ...job, attempt: 1, error_code: "timeout" }];
  s.consents = [consent("LOCAL_TRANSCRIPTION")];
  await open(page);
  await expect(
    page.getByRole("button", { name: "Adelantar reintento" }),
  ).toBeVisible();
  s.deny = 409;
  await page.getByRole("button", { name: "Adelantar reintento" }).click();
  await expect(page.locator("p[role=alert]")).toContainText(
    "no se reinició el presupuesto",
  );
  s.deny = 0;
  await page.getByRole("button", { name: "Adelantar reintento" }).click();
  await expect(page.getByText(/Intentos 1\/3/)).toBeVisible();
  await page.getByRole("button", { name: "Cancelar trabajo" }).click();
  await expect(
    page.getByRole("heading", { name: "Transcripción · Cancelado" }),
  ).toBeVisible();
  // Simulate a lagging read replica returning success after confirmed cancellation.
  s.jobs[0].status = "succeeded";
  await page
    .getByRole("button", { name: "Actualizar estado", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Transcripción · Cancelado" }),
  ).toBeVisible();
  await expect(
    page.getByText("Transcripción completada; consulta las versiones."),
  ).toHaveCount(0);
});

for (const status of [403, 503])
  test(`safe ${status} mutation rejection never grants consent`, async ({
    page,
  }) => {
    const s = await mock(page, true);
    s.deny = status;
    await open(page);
    await page
      .getByRole("button", { name: "Registrar consentimiento" })
      .first()
      .click();
    await page
      .getByRole("button", { name: "Confirmar registro", exact: true })
      .click();
    await expect(page.locator("p[role=alert]")).toContainText(
      status === 403 ? "No autorizado" : "temporalmente",
    );
    await expect(page.locator("body")).not.toContainText("SECRET");
    expect(s.consents).toHaveLength(0);
  });

test("oversized or invalid local file never uploads; permission revocation hides cached content", async ({
  page,
}) => {
  const s = await mock(page, true);
  s.consents = [consent("AUDIO_RECORDING")];
  s.transcripts = [transcript];
  await open(page);
  await page.getByLabel("O subir un archivo local").setInputFiles({
    name: "synthetic.wav",
    mimeType: "audio/wav",
    buffer: Buffer.alloc(25 * 1024 * 1024 + 1),
  });
  await expect(page.locator("p[role=alert]")).toContainText("25 MiB");
  await expect(
    page.getByRole("button", { name: "Subir audio", exact: true }),
  ).toBeDisabled();
  expect(s.requests).toHaveLength(0);
  s.denyRead = true;
  await page
    .getByRole("button", { name: "Actualizar estado", exact: true })
    .click();
  await expect(
    page.getByText("Texto sintético original", { exact: false }),
  ).toHaveCount(0);
  await expect(page.locator("p[role=alert]")).toContainText("No autorizado");
});

test("synthetic MediaRecorder pause, stop, cleanup on consent revocation; no hardware capture", async ({
  page,
}) => {
  const s = await mock(page, true);
  s.consents = [consent("AUDIO_RECORDING")];
  await page.addInitScript(() => {
    let stops = 0;
    Object.defineProperty(window, "syntheticTrackStops", { get: () => stops });
    Object.defineProperty(navigator, "mediaDevices", {
      value: {
        getUserMedia: async () => ({
          getTracks: () => [{ stop: () => stops++ }],
        }),
      },
    });
    class FakeRecorder {
      static isTypeSupported() {
        return true;
      }
      state = "inactive";
      ondataavailable?: (e: { data: Blob }) => void;
      onstop?: () => void;
      start() {
        this.state = "recording";
      }
      pause() {
        this.state = "paused";
      }
      resume() {
        this.state = "recording";
      }
      stop() {
        this.state = "inactive";
        this.ondataavailable?.({ data: new Blob(["synthetic audio"]) });
        this.onstop?.();
      }
    }
    Object.defineProperty(window, "MediaRecorder", { value: FakeRecorder });
  });
  await open(page);
  await page.getByRole("button", { name: "Grabar con micrófono" }).click();
  await expect(page.getByText("Grabando", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Pausar", exact: true }).click();
  await expect(
    page.getByText("Captura pausada", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Continuar captura" }).click();
  await page.getByRole("button", { name: "Detener", exact: true }).click();
  await expect(page.getByText(/Audio pendiente de envío/)).toBeVisible();
  await page.getByRole("button", { name: "Grabar con micrófono" }).click();
  await expect(page.getByText("Grabando", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Revocar", exact: true }).click();
  await expect(
    page.getByRole("group", { name: "Confirmar cambio de consentimiento" }),
  ).toContainText("No se eliminan automáticamente");
  await page.getByRole("button", { name: "Confirmar revocación" }).click();
  await expect(
    page.getByRole("button", { name: "Grabar con micrófono" }),
  ).toHaveCount(0);
  expect(
    await page.evaluate(() => Reflect.get(window, "syntheticTrackStops")),
  ).toBeGreaterThanOrEqual(2);
  expect(s.requests.some((r) => r.path.endsWith("/audio"))).toBe(false);
});
