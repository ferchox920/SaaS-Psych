import { test, expect, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import {
  importInputError,
  IMPORT_LIMIT,
} from "../src/features/project-bridge/model";
const client = "22222222-2222-4222-8222-222222222222",
  tenant = "11111111-1111-4111-8111-111111111111",
  process = "33333333-3333-4333-8333-333333333333";
const forbidden = [
  "Ana Perez",
  "ana@example.test",
  "5555 1212",
  "Calle Falsa 123",
  "TEST-DNI-XYZ",
  "Carlos Gomez",
  client,
  tenant,
  process,
];
const artifact = {
  schema_version: "clinical-project-export-v1",
  state_version: 5,
  privacy_mode: "minimized",
  approval_label: "APPROVED",
  epistemic_labels: {
    evidence_1: "PATIENT_REPORT",
    hypothesis_1: "CLINICAL_HYPOTHESIS",
  },
  case_runtime_profile: {
    scope: "selected_process",
    process_ref: "process_1",
    process_status: "active",
    hypothesis_refs: ["hypothesis_1"],
    goal_refs: [],
    gira_refs: [],
    open_questions: ["Explore hypothesis_1"],
    unavailable_sections: ["unselected_session_reports"],
  },
  selected_clinical_sources: {
    process: {
      ref: "process_1",
      title: "Proceso seleccionado",
      clinical_status: "active",
      version: 1,
    },
    evidence: [
      {
        ref: "evidence_1",
        epistemic_type: "patient_report",
        statement:
          "Paciente [person]; pareja [person]; [email]; [phone]; [location]; [document]",
        version: 1,
      },
    ],
    events: [],
    hypotheses: [
      {
        ref: "hypothesis_1",
        statement: "Hipótesis exploratoria",
        confidence_level: "yellow",
        supporting_evidence_refs: ["evidence_1"],
        contradicting_evidence_refs: [],
      },
    ],
    current_strategy: { targets: [], goals: [], giras: [] },
  },
  warnings: ["Minimization is not anonymization"],
};
const canonical =
  "# Clinical Project Export\n\n```json\n" +
  JSON.stringify(artifact, null, 2) +
  "\n```\n";
const snapshot = {
  export_id: "44444444-4444-4444-8444-444444444444",
  client_id: client,
  tenant_id: tenant,
  generated_by_user_id: "55555555-5555-4555-8555-555555555555",
  state_version: 5,
  generated_at: "2026-09-09T10:00:00Z",
  content_hash: "a".repeat(64),
  artifact,
  markdown: canonical,
};
const diff = {
  id: "synthetic-diff",
  client_id: client,
  status: "pending_review",
  revision: 1,
  base_state_version: 5,
  created_at: "2026-09-09",
  source_external_proposal_id: "synthetic-external",
  operations: [
    {
      id: "op1",
      sequence: 1,
      operation_type: "create_hypothesis",
      target_entity_id: "reserved-synthetic",
      review_status: "pending",
      original_proposal: { statement: "Exploratoria" },
    },
  ],
};
const proposal = {
  schema_version: "clinical-project-import-v1",
  source_export_id: snapshot.export_id,
  source_export_hash: snapshot.content_hash,
  provenance: {
    source_type: "manual_external_ai",
    provenance_assertion: "user_supplied",
    provider_name: "OpenAI",
    model_name: "GPT-5.6 Sol",
    surface: "ChatGPT Project",
  },
  operations: [
    {
      id: "p1",
      operation_type: "create_hypothesis",
      target_ref: "new_hypothesis_1",
      expected_entity_version: null,
      source_refs: ["evidence_1"],
      rationale: "Explorar",
      proposal: {
        process_ref: "process_1",
        statement: "Exploratoria",
        confidence_level: "yellow",
        supporting_evidence_refs: ["evidence_1"],
        contradicting_evidence_refs: [],
      },
    },
  ],
  open_questions: [],
  supervision_observations: [],
};
async function setup(page: Page) {
  const state = {
    write: true,
    consent: true,
    required: true,
    denyRead: false,
    status: 201,
    raw: "",
    imports: 0,
    generates: 0,
    merged: 0,
    external: [] as string[],
    pageErrors: [] as string[],
  };
  page.on("pageerror", (e) => state.pageErrors.push(e.message));
  await page.addInitScript(
    (t) => localStorage.setItem("sessionflow.auth.tenant", t),
    tenant,
  );
  await page.route("**/*", async (route) => {
    const req = route.request(),
      url = new URL(req.url());
    if (!["localhost", "127.0.0.1"].includes(url.hostname)) {
      state.external.push(req.url());
      await route.abort();
      return;
    }
    if (!url.pathname.startsWith("/api/v1/")) {
      if (url.origin !== "http://127.0.0.1:3103")
        state.external.push(req.url());
      return route.continue();
    }
    const path = url.pathname.replace("/api/v1", "");
    const send = (value: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(value),
      });
    if (path === "/auth/refresh")
      return send({
        access_token: "synthetic",
        token_type: "Bearer",
        expires_in: 3600,
      });
    if (path === "/auth/me")
      return send({
        user_id: "synthetic-user",
        tenant_id: tenant,
        role: "member",
      });
    expect(req.headers()["x-tenant-id"]).toBe(tenant);
    if (path.endsWith("/merge")) {
      state.merged++;
      throw new Error("Stage3C attempted merge");
    }
    if (req.method() === "GET") {
      if (path === `/clients/${client}`)
        return send({
          id: client,
          tenant_id: tenant,
          fullname: "Paciente Ficticio del Proyecto",
          contact: "synthetic@example.invalid",
          notes_public: "Ficha ficticia.",
          created_at: "2026-09-01",
          updated_at: "2026-09-01",
        });
      if (path.endsWith("/clinical-sessions"))
        return send({ items: [], can_write: state.write });
      if (path.endsWith("/longitudinal-state"))
        return send({
          state_version: 5,
          processes: [],
          active_evidence: [],
          open_proposals: [],
          recent_events: [],
        });
      if (path.endsWith("/processes"))
        return send({
          items: [
            {
              id: process,
              title: "Proceso seleccionado",
              approval_status: "approved",
              clinical_status: "active",
              version: 1,
              description: forbidden.join("; "),
            },
            {
              id: "unapproved",
              title: "No aprobado",
              approval_status: "proposed",
            },
          ],
        });
      if (path.endsWith("/consents"))
        return send({
          items: [
            {
              id: "c-local",
              scope: "LOCAL_AI_PROCESSING",
              status: "granted",
              effective_from: "2026-01-01",
            },
            ...(state.consent
              ? [
                  {
                    id: "c-external",
                    scope: "EXTERNAL_MANUAL_AI_PROCESSING",
                    status: "granted",
                    effective_from: "2026-01-01",
                  },
                ]
              : []),
          ],
        });
      if (path.endsWith("/project-exports"))
        return send({
          items: [{ ...snapshot, artifact: undefined, markdown: undefined }],
          offset: 0,
          has_more: false,
          requires_external_manual_consent: state.required,
        });
      if (path.endsWith(`/project-exports/${snapshot.export_id}`))
        return state.denyRead
          ? send({ error: { message: "raw internal consent error" } }, 403)
          : send(snapshot);
      if (path.endsWith("/clinical-diffs")) return send({ items: [diff] });
      if (path === `/clinical-diffs/${diff.id}`) return send(diff);
      if (path.endsWith("/project-proposals/synthetic-external"))
        return send({
          source_export_id: snapshot.export_id,
          content_hash: "b".repeat(64),
          proposal,
        });
    }
    if (path.endsWith("/project-exports") && req.method() === "POST") {
      state.generates++;
      expect(req.postDataJSON()).toEqual({ process_id: process });
      return send(snapshot, 201);
    }
    if (path.endsWith("/project-imports") && req.method() === "POST") {
      state.imports++;
      state.raw = req.postData() ?? "";
      return state.status === 201
        ? send(diff, 201)
        : send(
            {
              error: {
                code: "validation_error",
                message: "raw parser stack SQL private narrative",
              },
            },
            state.status,
          );
    }
    throw new Error(`Unexpected SaaS API request ${req.method()} ${path}`);
  });
  await page.goto(`/clients/${client}/clinical`);
  await page.getByRole("button", { name: "GPT Project", exact: true }).click();
  return state;
}
async function generate(page: Page) {
  await page.getByLabel("Seleccionar proceso clínico").selectOption(process);
  const start = Date.now();
  await page
    .getByRole("button", { name: "Generar contexto para GPT Project" })
    .click();
  await expect(page.getByTestId("portable-preview")).toBeVisible();
  console.log(
    `Stage3C synthetic preview_ms=${Date.now() - start} artifact_bytes=${Buffer.byteLength(canonical)}`,
  );
}
test("A-D: canonical preview, privacy, clipboard and download; no external network", async ({
  page,
  context,
}) => {
  const s = await setup(page);
  await context.grantPermissions(["clipboard-read", "clipboard-write"], {
    origin: "http://127.0.0.1:3103",
  });
  await generate(page);
  await page.evaluate(() => {
    const nativeWrite = navigator.clipboard.writeText.bind(navigator.clipboard);
    navigator.clipboard.writeText = async (text: string) => {
      Reflect.set(window, "stage3cCanonicalCopy", text);
      await nativeWrite(text);
    };
  });
  const preview = page.getByRole("region", { name: "Preview de export" }); // section has an accessible name
  await expect(
    page.getByRole("button", { name: "Copiar para GPT Project" }),
  ).toBeDisabled();
  const content = await page.getByTestId("portable-preview").innerText();
  for (const x of forbidden) expect(content).not.toContain(x);
  await page.getByLabel("Revisé todo el contenido", { exact: false }).check();
  await page.getByRole("button", { name: "Copiar para GPT Project" }).click();
  await expect(
    page.getByText("Copiado al portapapeles.", { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(() => Reflect.get(window, "stage3cCanonicalCopy")),
  ).toBe(canonical);
  // Native Windows clipboard normalizes LF to CRLF, outside the application.
  expect(
    (await page.evaluate(() => navigator.clipboard.readText())).replaceAll(
      "\r\n",
      "\n",
    ),
  ).toBe(canonical);
  const downloadEvent = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "Descargar artifact Markdown" })
    .click();
  const download = await downloadEvent;
  const data = await readFile((await download.path())!, "utf8");
  const clipboard = await page.evaluate(() => navigator.clipboard.readText());
  console.log(
    `Stage3C copy_api_bytes=${Buffer.byteLength(canonical)} native_clipboard_bytes=${Buffer.byteLength(clipboard)} download_bytes=${Buffer.byteLength(data)}`,
  );
  expect(data).toBe(canonical);
  expect(download.suggestedFilename()).toContain(
    snapshot.content_hash.slice(0, 12),
  );
  for (const x of forbidden) expect(data).not.toContain(x);
  expect(data).toContain("clinical-project-export-v1");
  expect(data).toContain("process_1");
  await expect(preview).toBeVisible();
  expect(s.external).toEqual([]);
  expect(s.merged).toBe(0);
  expect(s.pageErrors).toEqual([]);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "../../output/playwright/stage3c-bridge-mobile.png",
    fullPage: true,
  });
});
test("E,J: exact original JSON import and user supplied provenance handoff; no merge", async ({
  page,
}) => {
  const s = await setup(page);
  const original = " \n" + JSON.stringify(proposal, null, 2) + "\n ";
  await page.getByLabel("Resultado estructurado JSON").fill(original);
  await page
    .getByRole("button", { name: "Validar e importar propuesta" })
    .click();
  await expect(
    page.getByText(
      /Propuesta externa aceptada como ClinicalDiff pending_review/,
    ),
  ).toBeVisible();
  expect(s.raw).toBe(original);
  expect(s.imports).toBe(1);
  await page
    .getByRole("button", { name: "Revisar propuesta clínica", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Inspeccionar provenance externa" })
    .click();
  await expect(page.getByText(/Datos declarados por terapeuta/)).toBeVisible();
  await expect(
    page.getByText("GPT-5.6 Sol", { exact: true }).first(),
  ).toBeVisible();
  expect(s.external).toEqual([]);
  expect(s.merged).toBe(0);
});
for (const [label, status, edit] of [
  ["F unknown exact ref", 400, "badref"],
  ["G stale export", 409, "stale"],
  ["H invalid schema", 400, "schema"],
  ["cross-patient safe", 404, "foreign"],
] as const) {
  test(label, async ({ page }) => {
    const s = await setup(page);
    s.status = status;
    const input = JSON.stringify({
      ...proposal,
      ...(edit === "schema" ? { schema_version: "wrong" } : {}),
      ...(edit === "badref"
        ? {
            operations: [
              { ...proposal.operations[0], source_refs: ["hypothesis_7"] },
            ],
          }
        : {}),
    });
    await page.getByLabel("Resultado estructurado JSON").fill(input);
    await page
      .getByRole("button", { name: "Validar e importar propuesta" })
      .click();
    const alert = page
      .getByRole("region", { name: "GPT Project Bridge", exact: true })
      .getByRole("alert");
    await expect(alert).toBeVisible();
    expect(await alert.innerText()).not.toContain("raw parser");
    expect(s.raw).toBe(input);
    expect(s.imports).toBe(1);
    await expect(
      page.getByRole("button", {
        name: "Revisar propuesta clínica",
        exact: true,
      }),
    ).toHaveCount(0);
    expect(s.merged).toBe(0);
    expect(s.external).toEqual([]);
  });
}
test("I: read-only permits authorized preview but not generate/import", async ({
  page,
}) => {
  const s = await setup(page);
  s.write = false;
  await page.reload();
  await page.getByRole("button", { name: "GPT Project", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Generar contexto para GPT Project" }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Validar e importar propuesta" }),
  ).toBeDisabled();
  await page.getByRole("button", { name: /Inspeccionar snapshot/ }).click();
  await expect(page.getByTestId("portable-preview")).toBeVisible();
  expect(s.generates).toBe(0);
  expect(s.imports).toBe(0);
  expect(s.external).toEqual([]);
});
test("external consent distinct and revoked read cannot release cached artifact", async ({
  page,
}) => {
  const s = await setup(page);
  s.consent = false;
  await page
    .getByRole("button", { name: "Actualizar consentimiento y exports" })
    .click();
  await page.getByLabel("Seleccionar proceso clínico").selectOption(process);
  await expect(
    page.getByRole("button", { name: "Generar contexto para GPT Project" }),
  ).toBeDisabled();
  s.consent = true;
  await page
    .getByRole("button", { name: "Actualizar consentimiento y exports" })
    .click();
  await generate(page);
  await page.getByLabel("Revisé todo el contenido", { exact: false }).check();
  s.denyRead = true;
  await page.getByRole("button", { name: "Copiar para GPT Project" }).click();
  await expect(
    page
      .getByRole("region", { name: "GPT Project Bridge", exact: true })
      .getByRole("alert"),
  ).toContainText("consentimiento externo");
  await expect(
    page.getByText("Copiado al portapapeles.", { exact: true }),
  ).toHaveCount(0);
  await expect(page.getByTestId("portable-preview")).toHaveCount(0);
  expect(s.external).toEqual([]);
});

test("oversized file clears previous input; consent policy is server-derived", async ({
  page,
}) => {
  const s = await setup(page);
  s.consent = false;
  s.required = false;
  await page
    .getByRole("button", { name: "Actualizar consentimiento y exports" })
    .click();
  await generate(page);
  await page
    .getByLabel("Resultado estructurado JSON")
    .fill(JSON.stringify(proposal));
  await page
    .getByLabel("Archivo estructurado JSON")
    .setInputFiles({
      name: "too-large.json",
      mimeType: "application/json",
      buffer: Buffer.alloc(IMPORT_LIMIT + 1, 32),
    });
  await expect(page.getByLabel("Resultado estructurado JSON")).toHaveValue("");
  await expect(
    page.getByRole("button", { name: "Validar e importar propuesta" }),
  ).toBeDisabled();
  expect(s.imports).toBe(0);
  expect(s.external).toEqual([]);
});
test("file import preserves duplicate keys for backend rejection and bounds UTF8", async ({
  page,
}) => {
  const s = await setup(page);
  s.status = 400;
  const duplicate =
    '{"schema_version":"wrong","schema_version":"clinical-project-import-v1"}';
  await page.getByLabel("Archivo estructurado JSON").setInputFiles({
    name: "synthetic.json",
    mimeType: "application/json",
    buffer: Buffer.from(duplicate),
  });
  await expect(page.getByLabel("Resultado estructurado JSON")).toHaveValue(
    duplicate,
  );
  await page
    .getByRole("button", { name: "Validar e importar propuesta" })
    .click();
  await expect(
    page
      .getByRole("region", { name: "GPT Project Bridge", exact: true })
      .getByRole("alert"),
  ).toBeVisible();
  expect(s.raw).toBe(duplicate);
  expect(importInputError("{" + "é".repeat(IMPORT_LIMIT / 2))).toContain(
    "1 MiB",
  );
  expect(importInputError("```json\n{}\n``` ")).toContain("Formato");
  expect(importInputError(duplicate)).toBeNull();
  expect(s.external).toEqual([]);
});
