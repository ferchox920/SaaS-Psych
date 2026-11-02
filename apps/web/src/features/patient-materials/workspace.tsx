"use client";

import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSession } from "@/features/auth/hooks/use-session";
import { workspaceRequest } from "@/features/session-workspace/api";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

type Message = {
  id: string; role: string; text: string; recorded_at?: string;
  start_seconds?: number; end_seconds?: number;
};
type Material = {
  id: string; source_id: string; source_kind: string; project_id: string;
  source_created_at: string | null; source_updated_at: string | null;
  candidate_session_date: string | null; session_date_basis: string | null;
  clinical_session_id: string | null; revision_id: string; content_hash: string;
  review_status: string; imported_at: string; version_count: number;
  content: {
    title: string; text: string; messages: Message[] | null;
    files: { name: string; local_path: string; sha256: string; size_bytes: number; media_type: string }[] | null;
    metadata: Record<string, unknown> | null;
  };
};
const labels: Record<string, string> = {
  chatgpt_project: "Conversación de GPT", project_instructions: "Instrucciones del proyecto",
  desktop_recording: "Grabación de escritorio", phone_recording: "Grabación de teléfono",
  local_transcript: "Transcripción local", comparison: "Prueba y comparación",
};
const roles: Record<string, string> = {
  user: "Texto enviado / dictado", assistant: "Supervisión de GPT",
  patient: "Paciente · canal PC", therapist: "Terapeuta · micrófono", system: "Observación de la fuente",
};
function dateLabel(m: Material) {
  if (!m.candidate_session_date) return "Fecha de sesión por confirmar";
  return `${m.candidate_session_date} · ${m.session_date_basis === "professional_confirmed" ? "fecha confirmada" : "fecha candidata"}`;
}
function timeLabel(value: number) {
  return `${Math.floor(value / 60).toString().padStart(2, "0")}:${(value % 60).toFixed(1).padStart(4, "0")}`;
}
export function PatientMaterials({ clientId }: { clientId: string }) {
  const { session } = useSession();
  if (!session) return <p role="status">Verificando acceso…</p>;
  return <Materials key={`${session.tenantId}/${session.userId}/${clientId}`} clientId={clientId}
    identity={`${session.tenantId}/${session.userId}`} />;
}
function Materials({ clientId, identity }: { clientId: string; identity: string }) {
  const { authenticatedRequest } = useSession();
  const [primary, setPrimary] = useState("");
  const [secondary, setSecondary] = useState("");
  const [compare, setCompare] = useState(false);
  const list = useQuery({
    queryKey: ["patient-materials", identity, clientId], retry: false, gcTime: 0,
    queryFn: ({ signal }) => authenticatedRequest(async (auth) => {
      const items: Material[] = []; let offset = 0;
      for (;;) {
        const page = await workspaceRequest<{ items: Material[]; next_offset: number | null }>(auth,
          `/clients/${clientId}/materials?limit=100&offset=${offset}`, "GET", undefined, signal);
        items.push(...page.items);
        if (page.next_offset == null) return items;
        if (page.next_offset <= offset || page.next_offset > 100000) throw new Error("Paginación inválida");
        offset = page.next_offset;
      }
    }),
  });
  const client = useQuery({
    queryKey: ["patient-materials-name", identity, clientId], retry: false, gcTime: 0,
    queryFn: ({ signal }) => authenticatedRequest((a) => workspaceRequest<{ fullname: string }>(a, `/clients/${clientId}`, "GET", undefined, signal)),
  });
  const items = list.data ?? [];
  const first = items.find((m) => m.id === primary) ?? items[0];
  const second = items.find((m) => m.id === secondary) ?? items.find((m) => m.id !== first?.id);
  return <div className="space-y-6">
    <header className="space-y-3">
      <div className="flex flex-wrap gap-4"><Link className="text-primary underline" href="/clients">Pacientes</Link>
        <Link className="text-primary underline" href={`/clients/${clientId}/session`}>Sesión clínica</Link></div>
      <h1 className="text-3xl font-semibold">Materiales de {client.data?.fullname ?? "paciente"}</h1>
      <p className="max-w-3xl text-muted-foreground">Conversaciones, supervisión y versiones de transcripción conservadas con su procedencia. El texto dictado puede mezclar voces del paciente y del terapeuta; las respuestas de GPT son supervisión generada.</p>
      <div className="flex flex-wrap items-center gap-3"><Badge variant="outline">{items.length} materiales</Badge>
        <Button variant="outline" onClick={() => void list.refetch()} disabled={list.isFetching}>Actualizar lista</Button>
        <Button variant="outline" onClick={() => setCompare(!compare)} disabled={items.length < 2}>{compare ? "Ver un material" : "Comparar materiales"}</Button></div>
    </header>
    {list.isPending ? <p role="status">Cargando materiales…</p> : null}
    {list.error ? <p role="alert">No se pudieron cargar los materiales. Verifica tu asignación clínica e intenta nuevamente.</p> : null}
    {list.isSuccess && !items.length ? <p>No hay materiales importados. La asociación con un proyecto no importa su contenido automáticamente.</p> : null}
    {first ? <div className={compare && second ? "grid gap-5 xl:grid-cols-2" : "space-y-4"}>
      <div className="min-w-0 space-y-3"><label className="block font-medium" htmlFor="primary-material">Material principal</label>
        <select id="primary-material" className="w-full rounded-xl border bg-background p-3" value={first.id} onChange={(e) => setPrimary(e.target.value)}>
          {items.map((m) => <option value={m.id} key={m.id}>{m.content.title} · {labels[m.source_kind] ?? m.source_kind}</option>)}
        </select><Reader key={first.id} material={first} clientId={clientId} identity={identity} /></div>
      {compare && second ? <div className="min-w-0 space-y-3"><label className="block font-medium" htmlFor="secondary-material">Material para comparar</label>
        <select id="secondary-material" className="w-full rounded-xl border bg-background p-3" value={second.id} onChange={(e) => setSecondary(e.target.value)}>
          {items.map((m) => <option value={m.id} key={m.id}>{m.content.title} · {labels[m.source_kind] ?? m.source_kind}</option>)}
        </select><Reader key={second.id} material={second} clientId={clientId} identity={identity} /></div> : null}
    </div> : null}
    {compare ? <p className="text-sm text-muted-foreground">Compara el mismo intervalo de audio antes de evaluar diferencias. Los tiempos de transcripción son aproximados y se refieren al inicio de su grabación, que puede ser parcial.</p> : null}
  </div>;
}
function Reader({ material, clientId, identity }: { material: Material; clientId: string; identity: string }) {
  const { authenticatedRequest } = useSession();
  const [revision, setRevision] = useState("");
  const selectedRevision = revision || material.revision_id;
  const versions = useQuery({
    queryKey: ["patient-material-versions", identity, clientId, material.id, material.revision_id], gcTime: 0, retry: false,
    enabled: material.version_count > 1,
    queryFn: ({ signal }) => authenticatedRequest((a) => workspaceRequest<{ items: Material[] }>(a, `/clients/${clientId}/materials/${material.id}/revisions?limit=100`, "GET", undefined, signal)),
  });
  const detail = useQuery({
    queryKey: ["patient-material", identity, clientId, material.id, selectedRevision], gcTime: 0, retry: false,
    queryFn: ({ signal }) => authenticatedRequest((a) => workspaceRequest<Material>(a, `/clients/${clientId}/materials/${material.id}?revision_id=${selectedRevision}`, "GET", undefined, signal)),
  });
  const data = detail.data;
  const warnings = data?.content.metadata?.warnings;
  const calendar = data?.content.metadata?.calendar_date_evidence;
  const plannedStart = calendar && typeof calendar === "object" && "starts_at" in calendar && typeof calendar.starts_at === "string" ? calendar.starts_at : null;
  return <Card>
    <CardHeader><div className="flex flex-wrap gap-2"><Badge variant="outline">{labels[material.source_kind] ?? material.source_kind}</Badge>
      <Badge variant="outline">{(data?.review_status ?? material.review_status) === "accepted" ? "Revisión de fuente aceptada" : "Material por revisar"}</Badge></div>
      <CardTitle>{data?.content.title ?? material.content.title}</CardTitle><CardDescription>{dateLabel(material)} · {material.version_count} versión(es)</CardDescription>
      {material.session_date_basis === "source_timestamp" ? <p className="text-xs text-muted-foreground">La fecha del chat se conserva como referencia; no confirma por sí sola la fecha de atención.</p> : null}
      {plannedStart ? <p className="text-xs text-muted-foreground">Coincide con una cita agendada para {new Date(plannedStart).toLocaleString("es-AR", { timeZone: "America/Buenos_Aires", hour12: false })}.</p> : null}
    </CardHeader>
    <CardContent className="space-y-4">
      {versions.data?.items.length ? <label className="block space-y-2 text-sm"><span>Versión conservada</span><select className="w-full rounded-xl border bg-background p-2" value={selectedRevision} onChange={(e) => setRevision(e.target.value)}>{versions.data.items.map((v) => <option key={v.revision_id} value={v.revision_id}>{new Date(v.imported_at).toLocaleString("es-AR", { timeZone: "America/Buenos_Aires", hour12: false })} {v.revision_id === material.revision_id ? "· actual" : "· anterior"}</option>)}</select>{material.version_count > 100 ? <span>Se muestran las últimas 100 versiones.</span> : null}</label> : null}
      {detail.isPending ? <p role="status">Cargando contenido…</p> : null}
      {detail.error ? <p role="alert">No se pudo leer el contenido de este material.</p> : null}
      {Array.isArray(warnings) && warnings.length ? <div className="rounded-xl border border-amber-500 p-3 text-sm"><p className="font-semibold">Observaciones de la fuente</p><ul className="list-disc pl-5">{warnings.map((w, i) => <li key={i}>{String(w)}</li>)}</ul></div> : null}
      {data ? <>
        {data.content.text ? <pre className="max-h-[65vh] overflow-auto whitespace-pre-wrap break-words rounded-xl bg-muted p-4 font-sans text-sm">{data.content.text}</pre> : null}
        <div className="max-h-[70vh] space-y-4 overflow-auto">
          {(data.content.messages ?? []).map((m) => <article key={m.id} className={`rounded-xl border p-4 ${m.role === "assistant" ? "bg-muted/50" : ""}`}>
            <div className="mb-2 flex flex-wrap items-center gap-2"><Badge variant="outline">{roles[m.role] ?? m.role}</Badge>
              {m.start_seconds != null && m.end_seconds != null ? <span className="text-xs text-muted-foreground">{timeLabel(m.start_seconds)}–{timeLabel(m.end_seconds)}</span> : null}</div>
            {m.recorded_at ? <p className="mb-2 text-xs text-muted-foreground">Registro del mensaje: {new Date(m.recorded_at).toLocaleString("es-AR", { timeZone: "America/Buenos_Aires", hour12: false })}</p> : null}
            <pre className="whitespace-pre-wrap break-words font-sans text-sm">{m.text}</pre>
          </article>)}
        </div>
        {(data.content.files ?? []).length ? <section className="space-y-3"><h3 className="font-semibold">Originales conservados en el equipo</h3>
          <p className="text-xs text-muted-foreground">Estas referencias conservan ubicación e integridad. El archivo no se subió al almacén de audio de la app.</p>
          {data.content.files?.map((file) => <div key={file.sha256} className="rounded-xl border p-3 text-sm"><p className="font-medium">{file.name} · {(file.size_bytes / 1024 / 1024).toFixed(1)} MB</p><p className="break-all text-xs text-muted-foreground">{file.local_path}</p></div>)}
        </section> : null}
        <details className="rounded-xl border p-3 text-sm"><summary className="cursor-pointer">Procedencia y versión</summary>
          <dl className="mt-3 space-y-2 break-all"><div><dt>Proyecto</dt><dd>{data.project_id}</dd></div><div><dt>Fuente</dt><dd>{data.source_id}</dd></div><div><dt>Huella del contenido</dt><dd>{data.content_hash}</dd></div><div><dt>Revisión</dt><dd>{data.revision_id}</dd></div></dl>
          <p className="mt-3 text-muted-foreground">Importar conserva el material. No aprueba informes ni incorpora conclusiones a la memoria clínica.</p>
        </details>
      </> : null}
    </CardContent>
  </Card>;
}
