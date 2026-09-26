"use client";
import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { ExternalProvenance } from "../project-bridge/provenance";
import {
  Diff,
  Operation,
  counts,
  mergeAllowed,
  decisionAllowed,
  sourceLabel,
  Document,
} from "./model";
import {
  useClinicalRead,
  useClinicalCommand,
  Fields,
  DocumentEditor,
  ReadError,
  RunProvenance,
  Paged,
  Reference,
} from "./shared";

export function DiffInbox({
  client,
  write,
  selected,
  onSelect,
  onMerged,
}: {
  client: string;
  write: boolean;
  selected: string;
  onSelect: (id: string) => void;
  onMerged: () => Promise<void>;
}) {
  const q = useClinicalRead<{ items: Diff[] }>(
    client,
    `/clients/${client}/clinical-diffs`,
  );
  return (
    <section className="space-y-4">
      <h3 className="text-xl font-semibold">Propuestas ClinicalDiff</h3>
      <p>
        Propuesta de IA ≠ operación revisada ≠ estado clínico fusionado. GIRA
        solo cambia por este flujo; no hay guardado directo.
      </p>
      {q.error ? (
        <ReadError error={q.error} />
      ) : q.data ? (
        <Paged
          items={q.data.items ?? []}
          empty="No hay propuestas clínicas."
          render={(d) => {
            const c = counts(d);
            return (
              <article className="border rounded p-3" key={d.id}>
                <p>
                  {sourceLabel(d)} · {d.status} · Base v{d.base_state_version} ·{" "}
                  {new Date(d.created_at).toLocaleString()}
                </p>
                <p>
                  {c.reviewed}/{c.total} operaciones revisadas
                </p>
                <Button variant="outline" onClick={() => onSelect(d.id)}>
                  Abrir propuesta {d.id.slice(0, 8)}
                </Button>
              </article>
            );
          }}
        />
      ) : (
        <p role="status">Consultando propuestas…</p>
      )}
      {selected && (
        <DiffDetail
          key={selected}
          client={client}
          id={selected}
          write={write}
          onMerged={onMerged}
        />
      )}
    </section>
  );
}
export function DiffDetail({
  client,
  id,
  write,
  onMerged,
}: {
  client: string;
  id: string;
  write: boolean;
  onMerged: () => Promise<void>;
}) {
  const cache = useQueryClient();
  const command = useClinicalCommand();
  const q = useClinicalRead<Diff>(client, `/clinical-diffs/${id}`);
  const [confirm, setConfirm] = useState(false);
  const mutation = useMutation({
    mutationFn: async (input: {
      op?: string;
      decision?: string;
      modification?: Document;
      revision: number;
    }) =>
      command<Diff>(
        `/clinical-diffs/${id}${input.op ? `/operations/${input.op}/decision` : "/merge"}`,
        input.op ? "PUT" : "POST",
        {
          expected_diff_revision: input.revision,
          ...(input.op
            ? {
                decision: input.decision,
                ...(input.modification
                  ? { modification: input.modification }
                  : {}),
              }
            : {}),
        },
      ),
    onSuccess: async (d, input) => {
      if (d.client_id !== client) return;
      await cache.invalidateQueries({ queryKey: ["clinical-review"] });
      if (!input.op) {
        setConfirm(false);
        await onMerged();
      }
    },
    retry: false,
  });
  if (q.error) return <ReadError error={q.error} />;
  if (!q.data) return <p role="status">Cargando diff…</p>;
  const d = q.data;
  if (d.client_id !== client)
    return <p role="alert">Propuesta no disponible para este paciente.</p>;
  const c = counts(d);
  return (
    <section
      className="border-2 rounded p-4 space-y-4"
      aria-label="Detalle de propuesta"
    >
      <h3 className="text-xl">ClinicalDiff · {d.status}</h3>
      <Reference label="ID" id={d.id} />
      <p>
        {sourceLabel(d)} · revisión {d.revision} · Base v{d.base_state_version}{" "}
        · {c.reviewed}/{c.total} revisadas
      </p>
      <Reference label="Reporte fuente" id={d.source_session_report_id} />
      <Reference
        label="Propuesta externa manual"
        id={d.source_external_proposal_id}
      />
      <RunProvenance client={client} id={d.source_ai_run_id} />
      <ExternalProvenance client={client} id={d.source_external_proposal_id} />
      {d.is_stale && (
        <p role="alert">
          El estado clínico cambió desde que se generó esta propuesta. Contrasta
          con el estado actual. No puede fusionarse este diff obsoleto.
        </p>
      )}
      {mutation.error && <ReadError error={mutation.error} />}
      <Paged
        items={[...d.operations].sort((a, b) => a.sequence - b.sequence)}
        render={(o) => (
          <OperationCard
            key={`${o.id}/${o.review_status}`}
            op={o}
            enabled={decisionAllowed(d, o, write) && !mutation.isPending}
            decide={(decision, modification) =>
              mutation.mutate({
                op: o.id,
                decision,
                modification,
                revision: d.revision,
              })
            }
          />
        )}
      />
      <details>
        <summary>Incertidumbres / por qué explorar</summary>
        <Fields value={d.uncertainties} />
      </details>
      <p>
        Aprobadas {c.approved} · Modificadas {c.modified} · Rechazadas{" "}
        {c.rejected} · Aplicables {c.approved + c.modified}
      </p>
      {d.status === "merged" ? (
        <p role="status">
          Fusionado una sola vez · estado v{d.merged_state_version} ·{" "}
          {d.merged_at} · persona {d.merged_by_user_id}
        </p>
      ) : (
        <Button
          disabled={
            !mergeAllowed(d, write) || mutation.isPending || !!mutation.error
          }
          onClick={() => setConfirm(true)}
        >
          Fusionar cambios aprobados
        </Button>
      )}
      {confirm && mergeAllowed(d, write) && (
        <div role="group" aria-label="Confirmar merge humano">
          <p>
            Base v{d.base_state_version}. Se aplicarán {c.approved} aprobadas y{" "}
            {c.modified} modificadas; {c.rejected} rechazadas quedan excluidas.
            Esta acción cambia el estado clínico.
          </p>
          <Button
            disabled={mutation.isPending || !!mutation.error}
            onClick={() => mutation.mutate({ revision: d.revision })}
          >
            Confirmar merge humano
          </Button>
          <Button variant="outline" onClick={() => setConfirm(false)}>
            Volver
          </Button>
        </div>
      )}
      {mutation.error && (
        <Button
          variant="outline"
          onClick={async () => {
            setConfirm(false);
            await q.refetch();
            mutation.reset();
          }}
        >
          Consultar estado actual
        </Button>
      )}
    </section>
  );
}
function OperationCard({
  op,
  enabled,
  decide,
}: {
  op: Operation;
  enabled: boolean;
  decide: (decision: string, modification?: Document) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<Document>(
    structuredClone(op.original_proposal),
  );
  return (
    <article className="border rounded p-4 space-y-3">
      <h4 className="font-semibold">
        Operación {op.sequence} · {op.operation_type}
      </h4>
      <Reference label="Entidad objetivo" id={op.target_entity_id} />
      <p>
        Versión esperada: {op.expected_entity_version ?? "Entidad nueva"} ·
        Decisión humana: {op.review_status}
      </p>
      <h5 className="font-semibold">
        Propuesta original de IA / fuente externa
      </h5>
      <Fields value={op.original_proposal} />
      {op.human_modification && (
        <>
          <h5 className="font-semibold">Modificación humana</h5>
          <Fields value={op.human_modification} />
        </>
      )}
      {op.reviewed_at && (
        <p>
          Revisada {op.reviewed_at} · {op.reviewed_by_user_id}
        </p>
      )}
      <div className="flex flex-wrap gap-2">
        <Button disabled={!enabled} onClick={() => decide("approved")}>
          Aprobar operación {op.sequence}
        </Button>
        <Button
          variant="outline"
          disabled={!enabled}
          onClick={() => setEditing(!editing)}
        >
          Modificar operación {op.sequence}
        </Button>
        <Button
          variant="outline"
          disabled={!enabled}
          onClick={() => decide("rejected")}
        >
          Rechazar operación {op.sequence}
        </Button>
      </div>
      {editing && (
        <div>
          <h5>Edición humana — original conservado arriba</h5>
          <DocumentEditor
            value={draft}
            onChange={setDraft}
            disabled={!enabled}
          />
          <Button disabled={!enabled} onClick={() => decide("modified", draft)}>
            Guardar modificación {op.sequence}
          </Button>
        </div>
      )}
    </article>
  );
}
