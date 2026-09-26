"use client";
import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/http/api-client";
import {
  ClinicalSession,
  Consent,
  granted,
} from "@/features/session-workspace/model";
import { Report, Document } from "./model";
import {
  useClinicalRead,
  useClinicalCommand,
  Fields,
  DocumentEditor,
  ReadError,
  Reference,
  RunProvenance,
  Paged,
  useClinicalClock,
} from "./shared";
export function Reports({
  client,
  write,
  sessions,
  onDiff,
}: {
  client: string;
  write: boolean;
  sessions: ClinicalSession[];
  onDiff: (id: string) => void;
}) {
  const [selected, setSelected] = useState(sessions[0]?.id ?? "");
  const current = sessions.find((s) => s.id === selected) ?? sessions[0];
  return (
    <section className="space-y-4">
      <h3 className="text-xl font-semibold">Reportes de sesión</h3>
      <p>
        El reporte aprobado es contexto revisado por una persona. Sus candidatos
        de inferencia e hipótesis siguen siendo candidatos.
      </p>
      {!current ? (
        <p>No hay sesiones para revisar.</p>
      ) : (
        <>
          <label>
            Sesión fuente
            <select
              className="block border p-2 w-full"
              value={current.id}
              onChange={(e) => setSelected(e.target.value)}
            >
              {sessions.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.started_at} · {s.status}
                </option>
              ))}
            </select>
          </label>
          <SessionReports
            key={current.id}
            client={client}
            session={current}
            write={write}
            onDiff={onDiff}
          />
        </>
      )}
    </section>
  );
}
function SessionReports({
  client,
  session,
  write,
  onDiff,
}: {
  client: string;
  session: ClinicalSession;
  write: boolean;
  onDiff: (id: string) => void;
}) {
  const now = useClinicalClock();
  const cache = useQueryClient();
  const command = useClinicalCommand();
  const [sessionText, setSessionText] = useState("");
  const q = useClinicalRead<{ items: Report[] }>(
    client,
    `/clinical-sessions/${session.id}/reports`,
  );
  const c = useClinicalRead<{ items: Consent[] }>(
    client,
    `/clients/${client}/consents`,
  );
  const generate = useMutation({
    mutationFn: async () => {
      const currentConsent = await c.refetch();
      if (currentConsent.error || !granted(currentConsent.data?.items ?? [], "LOCAL_AI_PROCESSING", Date.now()))
        throw { status: 403 };
      return command<Report>(`/clinical-sessions/${session.id}/reports/generate`, "POST", {
        session_text: sessionText.trim(),
      });
    },
    onSuccess: async () => {
      setSessionText("");
      await cache.invalidateQueries({ queryKey: ["clinical-review"] });
    },
    retry: false,
  });
  const canGenerate = write && session.status === "completed" && !c.error &&
    granted(c.data?.items ?? [], "LOCAL_AI_PROCESSING", now);
  return q.error ? (
    <ReadError error={q.error} />
  ) : q.data ? (
    <div className="space-y-4">
      <div className="rounded-xl border p-4 space-y-3">
        <h4 className="font-semibold">Generar informe desde esta sesión</h4>
        <p className="text-sm">Se crea un borrador sujeto a revisión; el texto no se convierte automáticamente en conocimiento aprobado.</p>
        <label className="block space-y-1">
          <span>Texto de la sesión</span>
          <textarea className="block w-full rounded border p-2" rows={5} maxLength={40000}
            value={sessionText} onChange={(event) => setSessionText(event.target.value)}
            disabled={!canGenerate || generate.isPending} />
        </label>
        {!canGenerate && <p role="status" className="text-sm">Se requiere una sesión completada, asignación tratante y consentimiento de IA local vigente.</p>}
        {generate.error && <ReadError error={generate.error} />}
        <Button disabled={!canGenerate || generate.isPending || sessionText.trim().length < 20}
          onClick={() => generate.mutate()}>
          {generate.isPending ? "Generando borrador…" : "Generar borrador de informe"}
        </Button>
      </div>
      <Paged
      items={q.data.items ?? []}
      empty="No hay informes todavía. Complete la sesión y genere un borrador."
      render={(r) => (
        <ReportCard
          key={`${r.id}/${r.revision}/${r.status}`}
          report={r}
          client={client}
          write={write}
          analysisConsent={
            !c.error && granted(c.data?.items ?? [], "LOCAL_AI_PROCESSING", now)
          }
          verifyConsent={async () => {
            const out = await c.refetch();
            return (
              !out.error &&
              granted(out.data?.items ?? [], "LOCAL_AI_PROCESSING", Date.now())
            );
          }}
          completed={session.status === "completed"}
          onDiff={onDiff}
        />
      )}
      />
    </div>
  ) : (
    <p role="status">Cargando reportes…</p>
  );
}
function ReportCard({
  report: r,
  client,
  write,
  completed,
  analysisConsent,
  verifyConsent,
  onDiff,
}: {
  report: Report;
  client: string;
  write: boolean;
  completed: boolean;
  analysisConsent: boolean;
  verifyConsent: () => Promise<boolean>;
  onDiff: (id: string) => void;
}) {
  const cache = useQueryClient();
  const command = useClinicalCommand();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<Document>(structuredClone(r.report_json));
  const [confirm, setConfirm] = useState(false);
  const mutation = useMutation({
    mutationFn: async (action: string) => {
      if (action === "interpret") {
        if (!(await verifyConsent())) throw { status: 403 };
        return command<{ diff: { id: string } }>(
          `/clinical-sessions/${r.clinical_session_id}/longitudinal-analysis`,
          "POST",
        );
      }
      return command<Report>(
        `/session-reports/${r.id}${action === "approve" ? "/approve" : ""}`,
        action === "approve" ? "POST" : "PUT",
        action === "edit"
          ? { expected_revision: r.revision, report: draft }
          : { expected_revision: r.revision },
      );
    },
    onSuccess: async (out) => {
      setConfirm(false);
      setEditing(false);
      await cache.invalidateQueries({ queryKey: ["clinical-review"] });
      if ("diff" in out) onDiff(out.diff.id);
    },
    retry: false,
  });
  return (
    <article className="border rounded p-4 space-y-4">
      <h4 className="font-semibold">
        Reporte v{r.version} · {r.status} · revisión {r.revision}
      </h4>
      <p>
        Creado {r.created_at} · Aprobado {r.approved_at ?? "Todavía no"}
      </p>
      <Reference label="Sesión fuente" id={r.clinical_session_id} />
      <Reference label="Reporte" id={r.id} />
      <details>
        <summary>Provenance técnica y transcript fuente</summary>
        <RunProvenance client={client} id={r.source_ai_run_id} />
        {!r.source_ai_run_id && (
          <p>
            Versión humana sin AIRun directo; no se infiere una fuente técnica.
          </p>
        )}
      </details>
      <Fields value={r.report_json} />
      {mutation.error && <ReadError error={mutation.error} />}
      {mutation.error instanceof ApiError && mutation.error.status === 409 && (
        <Button variant="outline" onClick={() => {
          setConfirm(false);
          setEditing(false);
          mutation.reset();
          void cache.invalidateQueries({ queryKey: ["clinical-review"] });
        }}>
          Recargar reporte
        </Button>
      )}
      <div className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          disabled={!write || mutation.isPending || r.status === "superseded"}
          onClick={() => setEditing(!editing)}
        >
          {r.status === "approved"
            ? "Crear borrador desde reporte aprobado"
            : "Revisar borrador"}
        </Button>
        <Button
          disabled={!write || r.status !== "draft" || mutation.isPending}
          onClick={() => setConfirm(true)}
        >
          Aprobar reporte v{r.version}
        </Button>
        <Button
          variant="outline"
          disabled={
            !write ||
            r.status !== "approved" ||
            !completed ||
            !analysisConsent ||
            mutation.isPending
          }
          onClick={() => mutation.mutate("interpret")}
        >
          Solicitar interpretación longitudinal
        </Button>
      </div>
      <p>
        La interpretación usa el reporte aprobado actual de esta sesión,
        no una versión histórica elegida. Requiere consentimiento de IA
        local.
      </p>
      {confirm && (
        <div role="group" aria-label="Confirmar aprobación de reporte">
          <p>
            Aprobar este reporte lo hace disponible como contexto clínico
            aprobado. No aprueba hipótesis longitudinales ni estrategia y no
            ejecuta un merge.
          </p>
          <Button
            disabled={mutation.isPending}
            onClick={() => mutation.mutate("approve")}
          >
            Confirmar aprobación de reporte
          </Button>
          <Button variant="outline" onClick={() => setConfirm(false)}>
            Volver
          </Button>
        </div>
      )}
      {editing && (
        <div>
          <p>
            {r.status === "approved"
              ? "Se creará otra versión draft. El aprobado no se sobrescribe."
              : "Se incrementará la revisión del borrador actual."}
          </p>
          <DocumentEditor
            value={draft}
            onChange={setDraft}
            disabled={mutation.isPending}
          />
          <Button
            disabled={!write || mutation.isPending}
            onClick={() => mutation.mutate("edit")}
          >
            Guardar revisión de reporte
          </Button>
        </div>
      )}
    </article>
  );
}
