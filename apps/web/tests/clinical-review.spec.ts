import { test, expect, type Page } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  counts,
  decisionAllowed,
  mergeAllowed,
  epistemicLabels,
  sourceLabel,
  type Diff,
  type Report,
  type State,
  type Hypothesis,
  type Goal,
  type GIRA,
} from "../src/features/clinical-review-workspace/model";
const client = "22222222-2222-4222-8222-222222222222",
  tenant = "11111111-1111-4111-8111-111111111111",
  sid = "33333333-3333-4333-8333-333333333333";
const d: Diff = {
  id: "diff-synthetic",
  client_id: client,
  status: "pending_review",
  revision: 1,
  base_state_version: 5,
  is_stale: false,
  created_at: "2026-09-01",
  source_ai_run_id: "run-synthetic",
  source_session_report_id: "report-synthetic",
  operations: [1, 2, 3].map((n) => ({
    id: `operation-${n}`,
    sequence: n,
    operation_type: "create_process",
    target_entity_id: `process-${n}`,
    original_proposal: {
      title: `Propuesta original ${n}`,
      description: `Descripción sintética ${n}`,
      clinical_status: "observing",
      evidence_ids: ["evidence-synthetic"],
    },
    review_status: "pending",
  })),
  uncertainties: [],
};
const r: Report = {
  id: "report-synthetic",
  clinical_session_id: sid,
  version: 1,
  revision: 1,
  status: "draft",
  schema_version: "session-report-v1.1",
  created_at: "2026-09-01",
  source_ai_run_id: "run-synthetic",
  report_json: {
    schema_version: "session-report-v1.1",
    summary: "Resumen sintético",
    facts: [
      {
        id: "fact-1",
        statement: "Relato ficticio",
        category: "patient_report",
      },
    ],
    relevant_changes: [],
    interventions: [],
    patient_responses: [],
    affective_nodes: [],
    inference_candidates: [],
    hypothesis_candidates: [
      {
        id: "hc-1",
        statement: "Candidato no aprobado",
        traffic_light: "yellow",
        evidence_refs: ["fact-1"],
      },
    ],
    safety_signals: [],
    open_questions: [],
    longitudinal_candidates: [],
  },
};
const e = {
  id: "evidence-synthetic",
  epistemic_type: "patient_report",
  source_type: "session_report",
  source_id: r.id,
  source_version: 1,
  source_item_id: "fact-1",
  statement: "Relato del paciente ficticio",
  status: "active",
  version: 1,
};
const contrary = {
  ...e,
  id: "evidence-contrary",
  epistemic_type: "therapist_observation",
  statement: "Observación contradictoria ficticia",
};
async function setup(page: Page) {
  const s = {
    diff: structuredClone(d),
    reports: [structuredClone(r)],
    write: true,
    accessDenied: false,
    state: {
      client_id: client,
      state_version: 5,
      processes: [],
      unassigned_hypotheses: [],
      recent_events: [],
      active_evidence: [],
      open_proposals: [],
    } as State,
    hypotheses: [] as Hypothesis[],
    goals: [] as Goal[],
    giras: [] as GIRA[],
    mutations: [] as { path: string; body: Record<string, unknown> }[],
    gets: [] as string[],
    mergeConflict: false,
    generateUnavailable: false,
    transitions: 0,
    pageErrors: [] as string[],
    eventsPaged: false,
    hypothesesPaged: false,
    targetsPaged: false,
    goalsPaged: false,
    girasPaged: false,
    processesPaged: false,
  };
  page.on("pageerror", (err) => s.pageErrors.push(err.message));
  await page.addInitScript(
    (t) => localStorage.setItem("sessionflow.auth.tenant", t),
    tenant,
  );
  await page.route("**/api/v1/**", async (route) => {
    const req = route.request(),
      path = new URL(req.url()).pathname.replace("/api/v1", "");
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
    if (req.method() === "GET") {
      s.gets.push(path);
      if (path === `/clients/${client}`)
        return send({
          id: client,
          tenant_id: tenant,
          fullname: "Paciente Ficticia Aurora",
          contact: "aurora@example.invalid",
          notes_public: "Ficha sintética.",
          created_at: "2026-09-01",
          updated_at: "2026-09-01",
        });
      if (path.endsWith("/clinical-sessions")) {
        if (s.accessDenied) return send({ error: { code: "forbidden", message: "Forbidden" } }, 403);
        return send({
          items: [
            {
              id: sid,
              client_id: client,
              status: "completed",
              started_at: "2026-09-01",
            },
          ],
          can_write: s.write,
        });
      }
      if (path.endsWith("/longitudinal-state")) return send(s.state);
      if (path.endsWith("/reports")) return send({ items: s.reports });
      if (path.endsWith("/consents"))
        return send({
          items: [
            {
              id: "consent",
              scope: "LOCAL_AI_PROCESSING",
              status: "granted",
              effective_from: "2026-01-01",
              definition_version: 1,
              granted_at: "2026-01-01",
            },
          ],
        });
      if (path.endsWith("/clinical-diffs")) return send({ items: [s.diff] });
      if (path === `/clinical-diffs/${d.id}`) return send(s.diff);
      if (path.endsWith("/hypotheses")) {
        if (!s.hypothesesPaged) return send({ items: s.hypotheses, next_offset: null });
        const offset = Number(new URL(req.url()).searchParams.get("offset"));
        return send({
          items: [{
            id: `hypothesis-${offset}`,
            statement: `Hipótesis ficticia de página ${offset}`,
            confidence_level: "yellow",
            supporting_evidence: [],
            contradicting_evidence: [],
          }],
          next_offset: offset === 0 ? 25 : null,
        });
      }
      if (path.endsWith("/targets")) {
        if (!s.targetsPaged) return send({ items: [], next_offset: null });
        const offset = Number(new URL(req.url()).searchParams.get("offset"));
        return send({
          items: [{
            id: `target-${offset}`,
            title: `Target ficticio de página ${offset}`,
            target_type: "other",
            evidence_ids: [],
            hypothesis_ids: [],
            event_ids: [],
          }],
          next_offset: offset === 0 ? 25 : null,
        });
      }
      if (path.endsWith("/goals")) {
        if (!s.goalsPaged) return send({ items: s.goals, next_offset: null });
        const offset = Number(new URL(req.url()).searchParams.get("offset"));
        return send({
          items: [{
            id: `goal-${offset}`,
            title: `Goal ficticio de página ${offset}`,
            goal_type: "other",
            priority: "high",
            target_ids: [],
            indicators: [],
          }],
          next_offset: offset === 0 ? 25 : null,
        });
      }
      if (path.endsWith("/giras")) {
        if (!s.girasPaged) return send({ items: s.giras, next_offset: null });
        const offset = Number(new URL(req.url()).searchParams.get("offset"));
        return send({
          items: [{
            id: `gira-${offset}`,
            gira_version: offset + 1,
            entity_version: 1,
            title: `GIRA ficticia de página ${offset}`,
            summary: "Resumen ficticio",
            clinical_status: "planned",
            approval_status: "proposed",
            target_ids: [],
            goal_ids: [],
            rationales: [],
            phases: [],
          }],
          next_offset: offset === 0 ? 25 : null,
        });
      }
      if (path.endsWith("/processes")) {
        if (!s.processesPaged) return send({ items: [], next_offset: null });
        const offset = Number(new URL(req.url()).searchParams.get("offset"));
        return send({
          items: [{
            id: `process-${offset}`,
            title: `Proceso ficticio de página ${offset}`,
            description: "Descripción ficticia",
            events: [],
            hypotheses: [],
            therapeutic_strategy: { targets: [], goals: [], rationales: [], giras: [] },
          }],
          next_offset: offset === 0 ? 25 : null,
        });
      }
      if (path.endsWith("/evidence-page"))
        return send({ items: [e, contrary], offset: 0, has_more: false });
      if (path.endsWith("/events")) {
        if (!s.eventsPaged) return send({ items: [], next_offset: null });
        const offset = Number(new URL(req.url()).searchParams.get("offset"));
        return send({
          items: [{
            id: `event-${offset}`,
            event_type: "clinical_observation",
            title: `Evento ficticio de página ${offset}`,
            observed_at: "2026-09-01",
            evidence: [],
          }],
          next_offset: offset === 0 ? 25 : null,
        });
      }
      if (path.endsWith("/provenance"))
        return send({
          id: "run-synthetic",
          provider: "ollama",
          model: "synthetic-local",
          prompt_name: "clinical-review",
          prompt_version: "clinical-review-v1",
          transcript_version_id: "transcript-1",
        });
      if (path.includes("/clinical-history/"))
        return send({ items: [], offset: 0, has_more: false });
      if (path.endsWith("/approved-clinical-context"))
        return send({
          client_id: client,
          session_reports: s.reports.filter((r) => r.status === "approved"),
          sources: [],
          context_hash: "synthetic",
        });
      if (path === "/therapeutic-approaches")
        return send({
          items: [
            {
              slug: "cbt",
              version: 1,
              name: "CBT sintético",
              status: "deprecated",
            },
          ],
        });
      if (path === "/therapeutic-techniques")
        return send({
          items: [
            {
              slug: "experiment",
              version: 1,
              name: "Experimento sintético",
              approach_slug: "cbt",
              approach_version: 1,
              status: "active",
            },
          ],
        });
      if (
        ["/processes", "/events", "/targets"].some((suffix) =>
          path.endsWith(suffix),
        )
      )
        return send({ items: [] });
    } else {
      const body = req.postData() ? req.postDataJSON() : {};
      s.mutations.push({ path, body });
      if (!s.write)
        return send({ error: { message: "SECRET forbidden" } }, 403);
      if (path.endsWith("/reports/generate")) {
        if (s.generateUnavailable)
          return send({ error: { message: "SECRET provider details" } }, 503);
        const generated = { ...structuredClone(r), id: "generated-report", version: 1 };
        s.reports.push(generated);
        return send(generated, 201);
      }
      if (path.endsWith("/approve")) {
        s.reports[0].status = "approved";
        s.reports[0].approved_at = "2026-09-09";
        return send(s.reports[0]);
      }
      if (path === `/session-reports/${r.id}`) {
        if (s.reports[0].status === "approved")
          s.reports.push({
            ...r,
            id: "report-v2",
            version: 2,
            report_json: body.report,
          });
        else {
          s.reports[0].report_json = body.report;
          s.reports[0].revision++;
        }
        return send(s.reports.at(-1));
      }
      if (path.endsWith("/decision")) {
        expect(body.expected_diff_revision).toBe(s.diff.revision);
        const op = s.diff.operations.find((o) => path.includes(o.id))!;
        op.review_status = body.decision;
        if (body.modification) op.human_modification = body.modification;
        s.diff.revision++;
        const c = counts(s.diff);
        s.diff.status =
          c.reviewed === c.total
            ? c.approved + c.modified
              ? "approved"
              : "rejected"
            : "partially_reviewed";
        return send(s.diff);
      }
      if (path.endsWith("/merge")) {
        if (s.mergeConflict) {
          s.state.state_version = 6;
          return send(
            { error: { code: "conflict", message: "SECRET db" } },
            409,
          );
        }
        if (s.diff.status !== "merged") {
          s.transitions++;
          s.state.state_version++;
          s.diff.status = "merged";
          s.diff.merged_state_version = s.state.state_version;
          s.diff.merged_by_user_id = "human-synthetic";
        }
        return send(s.diff);
      }
      if (path.endsWith("/longitudinal-analysis"))
        return send({ diff: s.diff, reused: false }, 201);
    }
    throw new Error(`Unexpected synthetic API call: ${req.method()} ${path}`);
  });
  return s;
}
async function open(page: Page) {
  await page.goto(`/clients/${client}/clinical`);
  await expect(
    page.getByRole("heading", {
      name: "Revisión clínica y estado longitudinal",
    }),
  ).toBeVisible({ timeout: 15000 });
}
async function diff(page: Page) {
  await page.getByRole("button", { name: "Propuestas", exact: true }).click();
  await page.getByRole("button", { name: "Abrir propuesta diff-syn" }).click();
}
test("clinical review identifies the authorized patient by name", async ({ page }) => {
  const state = await setup(page);
  await open(page);
  const header = page.getByRole("heading", { name: "Revisión clínica y estado longitudinal" }).locator("..");
  await expect(header).toContainText("Paciente: Paciente Ficticia Aurora");
  expect(state.gets).toContain(`/clients/${client}`);
  await expect(header).not.toContainText(client);
});
test("clinical review does not fetch patient identity without clinical access", async ({ page }) => {
  const state = await setup(page);
  state.accessDenied = true;
  await page.goto(`/clients/${client}/clinical`);
  await expect(page.getByText("No autorizado para esta acción clínica", { exact: false })).toBeVisible();
  expect(state.gets).toContain(`/clients/${client}/clinical-sessions`);
  expect(state.gets).not.toContain(`/clients/${client}`);
});
test("model guards: approval, partial review, stale, readonly, epistemic types and source", () => {
  expect(mergeAllowed(d, true)).toBe(false);
  expect(decisionAllowed(d, d.operations[0], true)).toBe(true);
  expect(decisionAllowed(d, d.operations[0], false)).toBe(false);
  const ready = {
    ...d,
    status: "approved",
    operations: d.operations.map((o) => ({ ...o, review_status: "approved" })),
  };
  expect(mergeAllowed(ready, true)).toBe(true);
  expect(mergeAllowed({ ...ready, is_stale: true }, true)).toBe(false);
  expect(mergeAllowed(ready, false)).toBe(false);
  expect(new Set(Object.values(epistemicLabels)).size).toBe(5);
  expect(sourceLabel({ ...d, source_external_proposal_id: "external" })).toBe(
    "Propuesta externa manual",
  );
});
test("clinical events load server pages and can return to the first page", async ({ page }) => {
  const s = await setup(page);
  s.eventsPaged = true;
  await open(page);
  await page.getByRole("button", { name: "Evidencia y eventos" }).click();
  await expect(page.getByText("Evento ficticio de página 0")).toBeVisible();
  await page.getByRole("button", { name: "Más eventos" }).click();
  await expect(page.getByText("Evento ficticio de página 25")).toBeVisible();
  await expect(page.getByText("Evento ficticio de página 0")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Más eventos" })).toBeDisabled();
  await page.getByRole("button", { name: "Eventos anteriores" }).click();
  await expect(page.getByText("Evento ficticio de página 0")).toBeVisible();
  expect(s.pageErrors).toEqual([]);
});
test("clinical hypotheses load server pages and can return to the first page", async ({ page }) => {
  const s = await setup(page);
  s.hypothesesPaged = true;
  await open(page);
  await page.getByRole("button", { name: "Hipótesis", exact: true }).click();
  await expect(page.getByText("Hipótesis ficticia de página 0")).toBeVisible();
  await page.getByRole("button", { name: "Más hipótesis" }).click();
  await expect(page.getByText("Hipótesis ficticia de página 25")).toBeVisible();
  await expect(page.getByText("Hipótesis ficticia de página 0")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Más hipótesis" })).toBeDisabled();
  await page.getByRole("button", { name: "Hipótesis anteriores" }).click();
  await expect(page.getByText("Hipótesis ficticia de página 0")).toBeVisible();
  expect(s.pageErrors).toEqual([]);
});
test("clinical targets load server pages while goals remain visible", async ({ page }) => {
  const s = await setup(page);
  s.targetsPaged = true;
  await open(page);
  await page.getByRole("button", { name: "Targets y Goals" }).click();
  await expect(page.getByText("Target ficticio de página 0")).toBeVisible();
  await page.getByRole("button", { name: "Más targets" }).click();
  await expect(page.getByText("Target ficticio de página 25")).toBeVisible();
  await expect(page.getByText("Target ficticio de página 0")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Más targets" })).toBeDisabled();
  await page.getByRole("button", { name: "Targets anteriores" }).click();
  await expect(page.getByText("Target ficticio de página 0")).toBeVisible();
  expect(s.pageErrors).toEqual([]);
});
test("clinical goals load server pages while targets remain visible", async ({ page }) => {
  const s = await setup(page);
  s.goalsPaged = true;
  await open(page);
  await page.getByRole("button", { name: "Targets y Goals" }).click();
  await expect(page.getByText("Goal ficticio de página 0")).toBeVisible();
  await page.getByRole("button", { name: "Más goals" }).click();
  await expect(page.getByText("Goal ficticio de página 25")).toBeVisible();
  await expect(page.getByText("Goal ficticio de página 0")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Más goals" })).toBeDisabled();
  await page.getByRole("button", { name: "Goals anteriores" }).click();
  await expect(page.getByText("Goal ficticio de página 0")).toBeVisible();
  expect(s.pageErrors).toEqual([]);
});
test("GIRA versions load server pages and preserve navigation", async ({ page }) => {
  const s = await setup(page);
  s.girasPaged = true;
  await open(page);
  await page.getByRole("button", { name: "GIRA", exact: true }).click();
  await expect(page.getByText("GIRA ficticia de página 0")).toBeVisible();
  await page.getByRole("button", { name: "Más GIRA" }).click();
  await expect(page.getByText("GIRA ficticia de página 25")).toBeVisible();
  await expect(page.getByText("GIRA ficticia de página 0")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Más GIRA" })).toBeDisabled();
  await page.getByRole("button", { name: "GIRA anteriores" }).click();
  await expect(page.getByText("GIRA ficticia de página 0")).toBeVisible();
  expect(s.pageErrors).toEqual([]);
});
test("processes load server pages without retaining the previous page", async ({ page }) => {
  const s = await setup(page);
  s.processesPaged = true;
  await open(page);
  await page.getByRole("button", { name: "Procesos", exact: true }).click();
  await expect(page.getByText("Proceso ficticio de página 0")).toBeVisible();
  await page.getByRole("button", { name: "Más procesos" }).click();
  await expect(page.getByText("Proceso ficticio de página 25")).toBeVisible();
  await expect(page.getByText("Proceso ficticio de página 0")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Más procesos" })).toBeDisabled();
  await page.getByRole("button", { name: "Procesos anteriores" }).click();
  await expect(page.getByText("Proceso ficticio de página 0")).toBeVisible();
  expect(s.pageErrors).toEqual([]);
});
test("A report approval preserves candidates and does not merge; approved edit creates draft v2", async ({
  page,
}) => {
  const s = await setup(page);
  await open(page);
  await page.getByRole("button", { name: "Reportes", exact: true }).click();
  await expect(
    page.getByText("Candidato no aprobado", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Aprobar reporte v1" }).click();
  await expect(
    page.getByRole("group", { name: "Confirmar aprobación de reporte" }),
  ).toContainText("no ejecuta un merge");
  await page
    .getByRole("button", {
      name: "Confirmar aprobación de reporte",
      exact: true,
    })
    .click();
  await expect(
    page.getByRole("heading", { name: /Reporte v1 · approved/ }),
  ).toBeVisible();
  expect(s.transitions).toBe(0);
  expect(s.mutations).toHaveLength(1);
  await page
    .getByRole("button", { name: "Crear borrador desde reporte aprobado" })
    .click();
  await page
    .getByLabel("summary", { exact: true })
    .fill("Resumen humano ficticio");
  await page
    .getByRole("button", { name: "Guardar revisión de reporte" })
    .click();
  await expect(
    page.getByRole("heading", { name: /Reporte v2 · draft/ }),
  ).toBeVisible();
  expect(s.reports[0].report_json.summary).toBe("Resumen sintético");
  expect(s.state.state_version).toBe(5);
});
test("stale report approval displays conflict and reloads the observed revision", async ({ page }) => {
  const s = await setup(page);
  const revisions: number[] = [];
  let conflict = true;
  await page.route("**/api/v1/session-reports/report-synthetic/approve", async (route) => {
    const body = route.request().postDataJSON() as { expected_revision: number };
    revisions.push(body.expected_revision);
    if (conflict) {
      conflict = false;
      s.reports[0].revision = 2;
      return route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: { code: "conflict", message: "synthetic stale revision" } }) });
    }
    s.reports[0].status = "approved";
    s.reports[0].revision = 3;
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(s.reports[0]) });
  });
  await open(page);
  await page.getByRole("button", { name: "Reportes", exact: true }).click();
  await page.getByRole("button", { name: "Aprobar reporte v1" }).click();
  await page.getByRole("button", { name: "Confirmar aprobación de reporte" }).click();
  await expect(page.getByText("El estado clínico cambió", { exact: false })).toBeVisible();
  await expect(page.getByRole("button", { name: "Recargar reporte" })).toBeVisible();
  expect(revisions).toEqual([1]);
  await page.getByRole("button", { name: "Recargar reporte" }).click();
  await expect(page.getByRole("heading", { name: /Reporte v1 · draft · revisión 2/ })).toBeVisible();
  await page.getByRole("button", { name: "Aprobar reporte v1" }).click();
  await page.getByRole("button", { name: "Confirmar aprobación de reporte" }).click();
  await expect(page.getByRole("heading", { name: /Reporte v1 · approved/ })).toBeVisible();
  expect(revisions).toEqual([1, 2]);
});
test("B granular decisions retain original, human modification and rejection; C explicit single merge", async ({
  page,
}) => {
  const s = await setup(page);
  await open(page);
  await diff(page);
  await expect(
    page.getByRole("button", { name: "Fusionar cambios aprobados" }),
  ).toBeDisabled();
  await page
    .getByRole("button", { name: "Aprobar operación 1", exact: true })
    .click();
  await expect(
    page.getByRole("region", { name: "Detalle de propuesta" }),
  ).toContainText("1/3 revisadas");
  await page
    .getByRole("button", { name: "Modificar operación 2", exact: true })
    .click();
  await page
    .getByLabel("title", { exact: true })
    .fill("Modificación humana sintética");
  await page
    .getByRole("button", { name: "Guardar modificación 2", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Rechazar operación 3", exact: true })
    .click();
  await expect(
    page.getByText("Propuesta original 2", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Modificación humana sintética", { exact: true }),
  ).toBeVisible();
  expect(s.transitions).toBe(0);
  await page
    .getByRole("button", { name: "Fusionar cambios aprobados" })
    .click();
  await expect(
    page.getByRole("group", { name: "Confirmar merge humano" }),
  ).toContainText("1 rechazadas quedan excluidas");
  await page
    .getByRole("button", { name: "Confirmar merge humano", exact: true })
    .click();
  await expect(page.getByText(/Fusionado una sola vez/)).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Fusionar cambios aprobados" }),
  ).toHaveCount(0);
  expect(s.transitions).toBe(1);
  expect(s.diff.operations[2].review_status).toBe("rejected");
});
test("D stale conflict is explicit with no automatic retry", async ({
  page,
}) => {
  const s = await setup(page);
  s.diff.status = "approved";
  s.diff.operations.forEach((o) => (o.review_status = "approved"));
  s.mergeConflict = true;
  await open(page);
  await diff(page);
  await page
    .getByRole("button", { name: "Fusionar cambios aprobados" })
    .click();
  await page
    .getByRole("button", { name: "Confirmar merge humano", exact: true })
    .click();
  await expect(page.locator("p[role=alert]")).toContainText(
    "No se reintentó automáticamente",
  );
  await expect(page.locator("body")).not.toContainText("SECRET");
  await page.waitForTimeout(600);
  expect(s.mutations.filter((m) => m.path.endsWith("/merge"))).toHaveLength(1);
  expect(s.transitions).toBe(0);
});
test("E competing hypotheses keep separate support and contradiction", async ({
  page,
}) => {
  const s = await setup(page);
  s.hypotheses = [1, 2].map((i) => ({
    id: `h${i}`,
    statement: `Hipótesis sintética ${i}`,
    approval_status: "approved",
    clinical_status: i === 1 ? "active" : "weakened",
    confidence_level: "yellow",
    version: 1,
    supporting_evidence: [e],
    contradicting_evidence: [contrary],
  }));
  await open(page);
  await page.getByRole("button", { name: "Hipótesis", exact: true }).click();
  for (const i of [1, 2])
    await expect(
      page.getByRole("heading", {
        name: `Hipótesis clínica · Hipótesis sintética ${i}`,
      }),
    ).toBeVisible();
  await expect(page.getByRole("heading", { name: "APOYA · 1" })).toHaveCount(2);
  await expect(
    page.getByRole("heading", { name: "CONTRADICE · 1" }),
  ).toHaveCount(2);
  await expect(
    page.getByText("Observación contradictoria ficticia", { exact: true }),
  ).toHaveCount(2);
});
test("F qualitative indicator is not percentage or achievement", async ({
  page,
}) => {
  const s = await setup(page);
  s.goals = [
    {
      id: "g1",
      title: "Goal ficticio",
      goal_type: "behavior_change",
      priority: "high",
      approval_status: "approved",
      clinical_status: "active",
      target_ids: ["target1"],
      indicators: [
        {
          id: "i1",
          description: "Conversación observada",
          indicator_type: "qualitative",
          baseline: "Evita conversación",
          target_value: "Sostiene conversación",
          status: "active",
          links: [
            {
              source_id: e.id,
              source_type: "evidence",
              relation_type: "supports_progress",
            },
          ],
        },
      ],
    },
  ];
  await open(page);
  await page
    .getByRole("button", { name: "Targets y Goals", exact: true })
    .click();
  await expect(page.getByText(/Tipo qualitative/)).toBeVisible();
  await expect(page.getByRole("progressbar")).toHaveCount(0);
  await expect(page.locator("main").last()).not.toContainText("100%");
  expect(s.goals[0].clinical_status).toBe("active");
  expect(s.mutations).toHaveLength(0);
});
test("G GIRA versions and rationale chain, ordered non-linear phases, on-demand history", async ({
  page,
}) => {
  const s = await setup(page);
  s.giras = [1, 2].map((v) => ({
    id: `gira${v}`,
    title: "Ruta ficticia",
    summary: "Estrategia sintética",
    process_id: "p1",
    gira_version: v,
    entity_version: 1,
    approval_status: "approved",
    clinical_status: v === 1 ? "superseded" : "active",
    target_ids: ["t1"],
    goal_ids: ["g1"],
    supersedes_gira_id: v === 2 ? "gira1" : undefined,
    rationales: [
      {
        id: `r${v}`,
        process_id: "p1",
        target_id: "t1",
        goal_id: "g1",
        approach_slug: "cbt",
        approach_version: 1,
        technique_slug: "experiment",
        technique_version: 1,
        rationale: "Grounding sintético",
        expected_effect: "Señal observable",
        grounding_status: "grounded",
        evidence_ids: [e.id],
        hypothesis_ids: [],
      },
    ],
    phases: [
      {
        id: `phase${v}`,
        position: 2,
        title: "Revisitar",
        clinical_status: "paused",
        entry_criteria: "Entrada sintética",
        exit_criteria: "Salida sintética",
        goal_ids: ["g1"],
        indicator_ids: ["i1"],
        rationale_ids: [`r${v}`],
      },
    ],
  }));
  await open(page);
  expect(s.gets.some((p) => p.includes("clinical-history"))).toBe(false);
  await page.getByRole("button", { name: "GIRA", exact: true }).click();
  await expect(page.getByRole("heading", { name: /GIRA v1/ })).toContainText(
    "superseded",
  );
  await expect(page.getByRole("heading", { name: /GIRA v2/ })).toBeVisible();
  await expect(
    page.getByText(/Approach: CBT sintético v1 · deprecated/),
  ).toHaveCount(2);
  await expect(page.getByText(/Proceso p1 → Target t1 → Goal g1/)).toHaveCount(
    2,
  );
  await page
    .getByRole("button", { name: "Historial de gira gira1", exact: true })
    .click();
  await expect
    .poll(() => s.gets.some((p) => p.includes("clinical-history/gira/gira1")))
    .toBe(true);
  expect(s.mutations).toHaveLength(0);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  expect(s.pageErrors).toEqual([]);
  await page.screenshot({
    path: join(tmpdir(), "stage3b-gira-mobile.png"),
    fullPage: true,
  });
});
test("a completed session can generate a report; provider failure preserves fictional text", async ({ page }) => {
  const s = await setup(page);
  s.reports = [];
  s.generateUnavailable = true;
  await open(page);
  await page.getByRole("button", { name: "Reportes", exact: true }).click();
  const text = "Relato completamente ficticio para comprobar la generación manual del informe.";
  await page.getByLabel("Texto de la sesión").fill(text);
  await page.getByRole("button", { name: "Generar borrador de informe" }).click();
  await expect(page.getByText("No se confirmó la operación", { exact: false })).toBeVisible();
  await expect(page.getByLabel("Texto de la sesión")).toHaveValue(text);
  await expect(page.getByText("SECRET provider details")).toHaveCount(0);
  expect(s.reports).toHaveLength(0);
  s.generateUnavailable = false;
  await page.getByRole("button", { name: "Generar borrador de informe" }).click();
  await expect(page.getByText("Reporte v1 · draft", { exact: false })).toBeVisible();
  expect(s.mutations.filter((m) => m.path.endsWith("/reports/generate"))).toEqual([
    { path: `/clinical-sessions/${sid}/reports/generate`, body: { session_text: text } },
    { path: `/clinical-sessions/${sid}/reports/generate`, body: { session_text: text } },
  ]);
});

test("H supervisor may inspect but cannot approve, review or merge", async ({
  page,
}) => {
  const s = await setup(page);
  s.write = false;
  await open(page);
  await page.getByRole("button", { name: "Reportes", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Aprobar reporte v1" }),
  ).toBeDisabled();
  await diff(page);
  await expect(
    page.getByRole("button", { name: "Aprobar operación 1", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Modificar operación 2", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Fusionar cambios aprobados" }),
  ).toBeDisabled();
  expect(s.mutations).toHaveLength(0);
});

test("stale flag blocks review and merge; manual source is explicitly identified", async ({
  page,
}) => {
  const s = await setup(page);
  s.diff.is_stale = true;
  s.diff.source_external_proposal_id = "external-human-import";
  await open(page);
  await diff(page);
  await expect(page.locator("p[role=alert]")).toContainText("diff obsoleto");
  await expect(
    page.getByRole("region", { name: "Detalle de propuesta" }),
  ).toContainText("Propuesta externa manual");
  await expect(
    page.getByRole("button", { name: "Aprobar operación 1", exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Fusionar cambios aprobados" }),
  ).toBeDisabled();
  expect(s.mutations).toHaveLength(0);
});
test("invalid human modification remains local and original proposal visible", async ({
  page,
}) => {
  const s = await setup(page);
  await open(page);
  await diff(page);
  await page.route(
    "**/api/v1/clinical-diffs/*/operations/*/decision",
    (route) =>
      route.fulfill({
        status: 400,
        contentType: "application/json",
        body: JSON.stringify({ error: { message: "SECRET validation raw" } }),
      }),
  );
  await page
    .getByRole("button", { name: "Modificar operación 1", exact: true })
    .click();
  await page.getByLabel("title", { exact: true }).fill("");
  await page
    .getByRole("button", { name: "Guardar modificación 1", exact: true })
    .click();
  await expect(page.locator("p[role=alert]")).toContainText(
    "no cumple el contrato",
  );
  await expect(
    page.getByText("Propuesta original 1", { exact: true }),
  ).toBeVisible();
  expect(s.diff.operations[0].review_status).toBe("pending");
  expect(s.transitions).toBe(0);
});
test("longitudinal interpretation requires explicit approved-report action and remains a diff", async ({
  page,
}) => {
  const s = await setup(page);
  s.reports[0].status = "approved";
  await open(page);
  await page.getByRole("button", { name: "Reportes", exact: true }).click();
  expect(s.mutations).toHaveLength(0);
  await page
    .getByRole("button", { name: "Solicitar interpretación longitudinal" })
    .click();
  await expect(
    page.getByRole("region", { name: "Detalle de propuesta" }),
  ).toBeVisible();
  expect(
    s.mutations.filter((m) => m.path.endsWith("longitudinal-analysis")),
  ).toHaveLength(1);
  expect(s.transitions).toBe(0);
});
