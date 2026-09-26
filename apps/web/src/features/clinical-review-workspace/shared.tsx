"use client";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSession } from "@/features/auth/hooks/use-session";
import { apiFetch } from "@/lib/http/api-client";
import { env } from "@/lib/config/env";
import { listClinicalSessions } from "@/features/session-workspace/api";
import type { ClinicalSession } from "@/features/session-workspace/model";
import { Button } from "@/components/ui/button";
import { Document, Entity, clinicalError } from "./model";
export function useClinicalClock() {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 15000);
    return () => clearInterval(timer);
  }, []);
  return now;
}

export function useClinicalRead<T>(
  client: string,
  path: string,
  enabled = true,
) {
  const { session, authenticatedRequest } = useSession();
  return useQuery({
    queryKey: [
      "clinical-review",
      session?.tenantId,
      session?.userId,
      client,
      path,
    ],
    enabled: !!session && enabled,
    queryFn: ({ signal }) =>
      authenticatedRequest((a) =>
        apiFetch<T>({
          baseUrl: env.NEXT_PUBLIC_API_URL,
          path,
          method: "GET",
          signal,
          tenantId: a.tenantId,
          accessToken: a.accessToken,
        }),
      ),
    retry: false,
    staleTime: 10000,
    gcTime: 0,
  });
}
export function useClinicalSessions(client: string) {
  const { session, authenticatedRequest } = useSession();
  return useQuery<{ items: ClinicalSession[]; can_write: boolean }>({
    queryKey: ["clinical-review", session?.tenantId, session?.userId, client, "clinical-sessions"],
    enabled: !!session,
    queryFn: ({ signal }) => authenticatedRequest((auth) => listClinicalSessions(auth, client, signal)),
    retry: false,
    staleTime: 10000,
    gcTime: 0,
  });
}
export function useClinicalCommand() {
  const { authenticatedRequest } = useSession();
  return <T,>(path: string, method: "POST" | "PUT", body?: unknown) =>
    authenticatedRequest((a) =>
      apiFetch<T>({
        baseUrl: env.NEXT_PUBLIC_API_URL,
        path,
        method,
        body,
        tenantId: a.tenantId,
        accessToken: a.accessToken,
      }),
    );
}
export function Status({ item }: { item: Entity }) {
  return (
    <p>
      Aprobación: <strong>{item.approval_status ?? "No aplica"}</strong> ·
      Estado clínico:{" "}
      <strong>{item.clinical_status ?? item.status ?? "No informado"}</strong> ·
      Versión {item.version ?? "—"}
    </p>
  );
}
export function Reference({ id, label }: { id?: string; label: string }) {
  return (
    <p className="break-all text-sm">
      {label}: {id ?? "No disponible"}
    </p>
  );
}
const fieldLabels: Record<string, string> = {
  facts: "Hechos reportados", summary: "Resumen", interventions: "Intervenciones",
  open_questions: "Preguntas abiertas", safety_signals: "Señales de seguridad",
  schema_version: "Versión del esquema", affective_nodes: "Nodos afectivos",
  relevant_changes: "Cambios relevantes", patient_responses: "Respuestas del paciente",
  inference_candidates: "Inferencias candidatas", hypothesis_candidates: "Hipótesis candidatas",
  longitudinal_candidates: "Propuestas longitudinales", statement: "Enunciado",
  category: "Categoría", description: "Descripción", evidence_refs: "Referencias de evidencia",
  traffic_light: "Nivel de alerta", id: "Identificador", source_item_id: "Elemento fuente",
  epistemic_type: "Tipo epistémico",
};
export function Fields({ value }: { value: unknown }) {
  if (value === null || value === undefined) return <span>No informado</span>;
  if (Array.isArray(value))
    return value.length ? (
      <ul className="space-y-2 pl-4">
        {value.map((v, i) => (
          <li key={i}>
            <Fields value={v} />
          </li>
        ))}
      </ul>
    ) : (
      <span>Sin elementos</span>
    );
  if (typeof value === "object")
    return (
      <dl className="space-y-2">
        {Object.entries(value).map(([k, v]) => (
          <div key={k}>
            <dt className="font-semibold">{fieldLabels[k] ?? k.replaceAll("_", " ")}</dt>
            <dd className="pl-3 whitespace-pre-wrap break-words">
              <Fields value={v} />
            </dd>
          </div>
        ))}
      </dl>
    );
  return <span>{String(value)}</span>;
}
// Edits preserve structure and IDs. No raw JSON textarea or silent schema coercion.
const reportItemTemplates: Record<string, Document> = {
  facts: { statement: "", category: "" },
  relevant_changes: { description: "", category: "" },
  interventions: { type: "", description: "" },
  patient_responses: { response_type: "", description: "" },
  affective_nodes: { description: "" },
  inference_candidates: { statement: "", evidence_refs: [] },
  hypothesis_candidates: {
    statement: "",
    traffic_light: "",
    evidence_refs: [],
  },
  safety_signals: {
    description: "",
    category: "",
    requires_human_assessment: true,
  },
  open_questions: { question: "" },
  longitudinal_candidates: { operation: "", rationale: "" },
};
export function DocumentEditor({
  value,
  onChange,
  path = "",
  disabled = false,
}: {
  value: Document;
  onChange: (d: Document) => void;
  path?: string;
  disabled?: boolean;
}) {
  return (
    <div className="space-y-3">
      {Object.entries(value).map(([key, v]) => {
        const label = `${path}${key}`;
        const set = (next: unknown) => onChange({ ...value, [key]: next });
        if (typeof v === "string")
          return (
            <label className="block" key={key}>
              {label}
              <textarea
                className="block border rounded w-full p-2"
                aria-label={label}
                disabled={disabled || key === "id" || key === "schema_version"}
                value={v}
                onChange={(e) => set(e.target.value)}
              />
            </label>
          );
        if (typeof v === "number")
          return (
            <label className="block" key={key}>
              {label}
              <input
                className="border rounded p-2"
                type="number"
                disabled={disabled}
                value={v}
                onChange={(e) => set(Number(e.target.value))}
              />
            </label>
          );
        if (typeof v === "boolean")
          return (
            <label key={key}>
              <input
                type="checkbox"
                disabled={disabled}
                checked={v}
                onChange={(e) => set(e.target.checked)}
              />
              {label}
            </label>
          );
        if (Array.isArray(v))
          return (
            <fieldset className="border rounded p-3" key={key}>
              <legend>{label}</legend>
              {v.map((item, i) => (
                <div key={i}>
                  <DocumentEditor
                    key={i}
                    disabled={disabled}
                    path={`${label}[${i}].`}
                    value={
                      typeof item === "object" && item !== null
                        ? item
                        : { value: item }
                    }
                    onChange={(next) =>
                      set(
                        v.map((old, n) =>
                          n === i
                            ? typeof item === "object" && item !== null
                              ? next
                              : next.value
                            : old,
                        ),
                      )
                    }
                  />
                  <Button
                    variant="outline"
                    disabled={disabled}
                    onClick={() => set(v.filter((_, n) => n !== i))}
                  >
                    Quitar {label}[{i}]
                  </Button>
                </div>
              ))}
              {(key.endsWith("_ids") ||
                key.endsWith("_refs") ||
                reportItemTemplates[key]) && (
                <Button
                  variant="outline"
                  disabled={disabled}
                  onClick={() =>
                    set([
                      ...v,
                      reportItemTemplates[key]
                        ? {
                            id: "",
                            ...structuredClone(reportItemTemplates[key]),
                          }
                        : "",
                    ])
                  }
                >
                  Añadir {label}
                </Button>
              )}
              {reportItemTemplates[key] && (
                <p>
                  Los ítems nuevos reciben su ID estable del servidor al
                  guardar; los existentes lo conservan.
                </p>
              )}
            </fieldset>
          );
        if (v && typeof v === "object")
          return (
            <DocumentEditor
              key={key}
              value={v as Document}
              disabled={disabled}
              path={`${label}.`}
              onChange={set}
            />
          );
        return (
          <div key={key}>
            <p>{label}: no informado</p>
            <Button
              variant="outline"
              disabled={disabled}
              onClick={() => set(key.endsWith("_version") ? 1 : "")}
            >
              Definir {label}
            </Button>
          </div>
        );
      })}
    </div>
  );
}
export function Paged<T>({
  items,
  render,
  empty = "Sin elementos",
}: {
  items: T[];
  render: (x: T) => React.ReactNode;
  empty?: string;
}) {
  const [page, setPage] = useState(0);
  const index = Math.min(page, Math.max(0, Math.ceil(items.length / 20) - 1));
  return (
    <div className="space-y-4">
      {!items.length ? (
        <p>{empty}</p>
      ) : (
        items.slice(index * 20, index * 20 + 20).map(render)
      )}
      {items.length > 20 && (
        <div className="flex gap-3">
          <Button disabled={index === 0} onClick={() => setPage(index - 1)}>
            Anterior
          </Button>
          <span>
            Página {index + 1} · {items.length} elementos
          </span>
          <Button
            disabled={(index + 1) * 20 >= items.length}
            onClick={() => setPage(index + 1)}
          >
            Siguiente
          </Button>
        </div>
      )}
    </div>
  );
}
export function ReadError({ error }: { error: unknown }) {
  return <p role="alert">{clinicalError(error)}</p>;
}
export function RunProvenance({ client, id }: { client: string; id?: string }) {
  const [open, setOpen] = useState(false);
  const q = useClinicalRead<Document>(
    client,
    `/clients/${client}/clinical-runs/${id}/provenance`,
    open && !!id,
  );
  return (
    <div>
      <Button variant="outline" disabled={!id} onClick={() => setOpen(!open)}>
        Inspeccionar AIRun
      </Button>
      {open &&
        (q.error ? (
          <ReadError error={q.error} />
        ) : q.data ? (
          <Fields value={q.data} />
        ) : (
          <p role="status">Cargando provenance…</p>
        ))}
    </div>
  );
}
