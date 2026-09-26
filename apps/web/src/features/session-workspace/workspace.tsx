"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/features/auth/hooks/use-session";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { listClinicalSessions, workspaceRequest, uploadAudio } from "./api";
import {
  Artifact,
  ClinicalSession,
  Consent,
  granted,
  Job,
  safeError,
  Scope,
  scopes,
  Segment,
  terminal,
  Transcript,
  workspaceKey,
} from "./model";
import { AudioPanel } from "./audio-panel";
import { JobPanel } from "./job-panel";
import { TranscriptPanel } from "./transcript-panel";

function useClock() {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 15000);
    return () => clearInterval(timer);
  }, []);
  return now;
}
type List<T> = { items: T[] };
export function SessionWorkspace({ clientId, appointmentId }: { clientId: string; appointmentId?: string }) {
  const { session } = useSession();
  if (!session) return <p role="status">Verificando acceso…</p>;
  return (
    <Workspace
      key={`${session.tenantId}/${session.userId}/${clientId}/${appointmentId ?? ""}`}
      clientId={clientId}
      appointmentId={appointmentId}
      tenantId={session.tenantId}
      userId={session.userId}
    />
  );
}
function Workspace({
  clientId,
  appointmentId,
  tenantId,
  userId,
}: {
  clientId: string;
  appointmentId?: string;
  tenantId: string;
  userId: string;
}) {
  const { authenticatedRequest } = useSession();
  const cache = useQueryClient();
  const now = useClock();
  const base = workspaceKey(tenantId, userId, clientId);
  const [selected, setSelected] = useState("");
  const sessions = useQuery({
    queryKey: [...base, "sessions"],
    queryFn: ({ signal }) =>
      authenticatedRequest((a) => listClinicalSessions(a, clientId, signal)),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchInterval: 15000,
    refetchOnWindowFocus: true,
  });
  const consent = useQuery({
    queryKey: [...base, "consent"],
    queryFn: ({ signal }) =>
      authenticatedRequest((a) =>
        workspaceRequest<List<Consent>>(
          a,
          `/clients/${clientId}/consents`,
          "GET",
          undefined,
          signal,
        ),
      ),
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchInterval: 15000,
    refetchOnWindowFocus: true,
  });
  const client = useQuery({
    queryKey: [...base, "name"],
    queryFn: ({ signal }) =>
      authenticatedRequest((a) =>
        workspaceRequest<{ fullname: string }>(
          a,
          `/clients/${clientId}`,
          "GET",
          undefined,
          signal,
        ),
      ),
    gcTime: 0,
    retry: false,
  });
  const refresh = () => cache.invalidateQueries({ queryKey: base });
  const mutation = useMutation({
    mutationFn: ({ path, body }: { path: string; body?: unknown }) =>
      authenticatedRequest((a) =>
        workspaceRequest<ClinicalSession>(a, path, "POST", body),
      ),
    onSuccess: async (out) => {
      if (out.client_id === clientId) setSelected(out.id);
      await refresh();
    },
    onSettled: refresh,
  });
  const linkedSession = appointmentId
    ? sessions.data?.items.find((item) => item.appointment_id === appointmentId && item.status !== "voided")
    : undefined;
  const current =
    sessions.data?.items.find((s) => s.id === selected) ??
    linkedSession ??
    sessions.data?.items.find((s) => s.status === "in_progress") ??
    sessions.data?.items[0];
  const writable = sessions.data?.can_write === true && !sessions.isError;
  const consents = consent.isError ? [] : (consent.data?.items ?? []);
  const [decision, setDecision] = useState<{
    scope: Scope;
    id?: string;
  } | null>(null);
  const grantMutation = useMutation({
    mutationFn: async (d: { scope: Scope; id?: string }) => {
      await cache.cancelQueries({ queryKey: [...base, "consent"] });
      return authenticatedRequest((a) =>
        workspaceRequest(
          a,
          `/clients/${clientId}/consents${d.id ? `/${d.id}/revoke` : ""}`,
          "POST",
          d.id ? undefined : { scope: d.scope, definition_version: 1 },
        ),
      );
    },
    onSuccess: () => setDecision(null),
    onSettled: refresh,
  });
  const error =
    sessions.error || consent.error || mutation.error || grantMutation.error;
  if (sessions.isError)
    return (
      <div>
        <Link href="/clients">Volver a pacientes</Link>
        <p role="alert">{safeError(sessions.error)}</p>
        <Button onClick={() => void refresh()}>Actualizar estado</Button>
      </div>
    );
  return (
    <div className="space-y-6 max-w-5xl mx-auto">
      <header className="space-y-2">
        <div className="flex flex-wrap gap-x-4 gap-y-1">
          <Link className="text-primary !underline" href={`/clients/${clientId}/clinical`}>Abrir revisión clínica y estado longitudinal</Link>
          <Link className="text-primary !underline" href="/clients">Volver a pacientes</Link>
        </div>
        <h2 className="text-3xl font-semibold">
          Sesión clínica · {client.data?.fullname ?? "Paciente"}
        </h2>
        <p>Consentimiento, audio y transcripción · procesamiento local</p>
        {appointmentId ? <p>Cita seleccionada: {appointmentId}</p> : null}
        <Button variant="outline" onClick={() => void refresh()}>
          Actualizar estado
        </Button>
      </header>
      {error && <p role="alert">{safeError(error)}</p>}
      {sessions.isPending ? (
        <p role="status">Cargando sesiones…</p>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Sesión clínica</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <p>
              {writable
                ? "Acceso de terapeuta tratante"
                : "Acceso de lectura o permisos sin confirmar. Las escrituras están deshabilitadas."}
            </p>
            {linkedSession ? (
              <p>
                Esta cita ya tiene una sesión clínica {linkedSession.status === "in_progress" ? "en curso. Continúe y complétela antes de generar un informe" : "completada. Puede continuar desde la revisión clínica"}; no se iniciará otra sesión para la misma cita.
              </p>
            ) : null}
            {!!sessions.data?.items.length && (
              <label className="block">
                Seleccionar sesión
                <select
                  className="block w-full border rounded p-2"
                  value={current?.id ?? ""}
                  onChange={(e) => setSelected(e.target.value)}
                >
                  {sessions.data.items.map((s) => (
                    <option key={s.id} value={s.id}>
                      {new Date(s.started_at).toLocaleString()} · {s.status}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {current ? (
              <>
                <p>
                  Estado: {current.status} · Inicio:{" "}
                  {new Date(current.started_at).toLocaleString()}
                </p>
                <p>
                  Finalización:{" "}
                  {current.ended_at
                    ? new Date(current.ended_at).toLocaleString()
                    : "Pendiente"}{" "}
                  · Cita: {current.appointment_id ?? "Sin cita vinculada"}
                </p>
              </>
            ) : (
              <p>No hay una sesión clínica todavía.</p>
            )}
            <div className="flex gap-2 flex-wrap">
              <Button
                disabled={
                  !writable ||
                  mutation.isPending ||
                  !!linkedSession ||
                  sessions.data?.items.some((s) => s.status === "in_progress")
                }
                onClick={() =>
                  mutation.mutate({
                    path: "/clinical-sessions",
                    body: {
                      client_id: clientId,
                      ...(appointmentId ? { appointment_id: appointmentId } : {}),
                      started_at: new Date().toISOString(),
                    },
                  })
                }
              >
                Iniciar sesión clínica
              </Button>
              <Button
                variant="outline"
                disabled={
                  !writable ||
                  mutation.isPending ||
                  current?.status !== "in_progress"
                }
                onClick={() => {
                  if (
                    current &&
                    window.confirm(
                      "¿Completar esta sesión clínica? No se aprobará ningún reporte.",
                    )
                  )
                    mutation.mutate({
                      path: `/clinical-sessions/${current.id}/complete`,
                    });
                }}
              >
                Completar sesión
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle>Consentimientos independientes</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {consent.isPending ? (
            <p role="status">Consultando consentimientos…</p>
          ) : (
            (Object.entries(scopes) as [Scope, string][]).map(
              ([scope, label]) => {
                const active = consents.find(
                  (c) => c.scope === scope && c.status === "granted",
                );
                return (
                  <section
                    key={scope}
                    className="rounded-xl border p-3 space-y-2"
                  >
                    <h3 className="font-semibold">{label}</h3>
                    <p>
                      {granted(consents, scope, now)
                        ? "Otorgado"
                        : "No vigente"}{" "}
                      · Definición v{active?.definition_version ?? 1}
                    </p>
                    <Button
                      variant="outline"
                      disabled={
                        !writable || grantMutation.isPending || consent.isError
                      }
                      onClick={() => setDecision({ scope, id: active?.id })}
                    >
                      {active ? "Revocar" : "Registrar consentimiento"}
                    </Button>
                    <details>
                      <summary className="cursor-pointer">
                        Historial de este alcance
                      </summary>
                      {consents
                        .filter((c) => c.scope === scope)
                        .map((c) => (
                          <p key={c.id} className="text-sm">
                            v{c.definition_version} · {c.status} · Otorgado{" "}
                            {new Date(c.granted_at).toLocaleString()}
                            {c.revoked_at
                              ? ` · Revocado ${new Date(c.revoked_at).toLocaleString()}`
                              : ""}
                          </p>
                        ))}
                    </details>
                  </section>
                );
              },
            )
          )}
          {decision && (
            <div
              role="group"
              aria-label="Confirmar cambio de consentimiento"
              className="border-2 rounded p-4 space-y-3"
            >
              <p>
                {decision.id ? "Revocar" : "Registrar"}:{" "}
                <strong>{scopes[decision.scope]}</strong>
              </p>
              <p>
                {decision.id
                  ? "Se bloqueará el procesamiento futuro de este alcance y pueden cancelarse trabajos pendientes o activos. No se eliminan automáticamente registros clínicos existentes."
                  : "Registra únicamente el consentimiento efectivamente otorgado para este alcance. No habilita los demás alcances."}
              </p>
              <Button
                disabled={grantMutation.isPending}
                onClick={() => grantMutation.mutate(decision)}
              >
                Confirmar {decision.id ? "revocación" : "registro"}
              </Button>{" "}
              <Button variant="outline" onClick={() => setDecision(null)}>
                Volver
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
      {current ? (
        <SessionBody
          key={current.id}
          current={current}
          base={base}
          writable={writable}
          consents={consents}
          verifyAudio={async () => {
            const [s, c] = await Promise.all([
              sessions.refetch(),
              consent.refetch(),
            ]);
            return (
              !s.isError &&
              !c.isError &&
              s.data?.can_write === true &&
              granted(c.data?.items ?? [], "AUDIO_RECORDING", Date.now())
            );
          }}
        />
      ) : (
        <p>
          Inicia una sesión para habilitar el flujo de audio y transcripción.
        </p>
      )}
    </div>
  );
}
function SessionBody({
  current,
  base,
  writable,
  consents,
  verifyAudio,
}: {
  current: ClinicalSession;
  base: readonly string[];
  writable: boolean;
  consents: Consent[];
  verifyAudio: () => Promise<boolean>;
}) {
  const now = useClock();
  const { authenticatedRequest } = useSession();
  const cache = useQueryClient();
  const key = [...base, current.id];
  const path = `/clinical-sessions/${current.id}`;
  const artifacts = useQuery({
    queryKey: [...key, "artifacts"],
    queryFn: ({ signal }) =>
      authenticatedRequest((a) =>
        workspaceRequest<List<Artifact>>(
          a,
          `${path}/artifacts`,
          "GET",
          undefined,
          signal,
        ),
      ),
    gcTime: 0,
    retry: false,
  });
  const transcripts = useQuery({
    queryKey: [...key, "transcripts"],
    queryFn: ({ signal }) =>
      authenticatedRequest((a) =>
        workspaceRequest<List<Transcript>>(
          a,
          `${path}/transcripts`,
          "GET",
          undefined,
          signal,
        ),
      ),
    gcTime: 0,
    retry: false,
  });
  const jobs = useQuery({
    queryKey: [...key, "jobs"],
    queryFn: ({ signal }) =>
      authenticatedRequest((a) =>
        workspaceRequest<List<Job>>(
          a,
          `${path}/jobs`,
          "GET",
          undefined,
          signal,
        ),
      ),
    gcTime: 0,
    retry: false,
    refetchInterval: (q) =>
      q.state.error
        ? false
        : q.state.data?.items.some((j) => !terminal(j))
          ? 15000
          : false,
  });
  const refresh = () => cache.invalidateQueries({ queryKey: key });
  const command = useMutation({
    mutationFn: ({ url, body }: { url: string; body: unknown }) =>
      authenticatedRequest((a) => workspaceRequest(a, url, "POST", body)),
    onSettled: refresh,
  });
  const audio = useMutation({
    mutationFn: ({ blob, mime }: { blob: Blob; mime: string }) =>
      authenticatedRequest((a) => uploadAudio(a, current.id, blob, mime)),
    onSettled: refresh,
  });
  const canWrite = writable && current.status !== "voided";
  const errors = [
    artifacts.error,
    transcripts.error,
    jobs.error,
    command.error,
    audio.error,
  ].filter(Boolean);
  const readError = artifacts.error || transcripts.error || jobs.error;
  if (readError)
    return (
      <div>
        <p role="alert">{safeError(readError)}</p>
        <Button onClick={() => void refresh()}>Actualizar contenido</Button>
      </div>
    );
  return (
    <div className="space-y-6">
      {!!errors.length && <p role="alert">{safeError(errors[0])}</p>}
      <Card>
        <CardHeader>
          <CardTitle>Audio de la sesión</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {canWrite && granted(consents, "AUDIO_RECORDING", now) ? (
            <AudioPanel
              verify={verifyAudio}
              upload={async (blob, mime) => {
                await audio.mutateAsync({ blob, mime });
              }}
            />
          ) : (
            <p>
              Grabación y upload deshabilitados: requiere sesión válida, permiso
              de escritura y consentimiento AUDIO_RECORDING.
            </p>
          )}
          {artifacts.isPending ? (
            <p role="status">Cargando artefactos…</p>
          ) : !artifacts.data?.items.length ? (
            <p>No hay audio disponible.</p>
          ) : (
            artifacts.data.items.map((a) => (
              <article key={a.id} className="border rounded p-3 space-y-2">
                <p>
                  Audio {a.id.slice(0, 8)} ·{" "}
                  {new Date(a.created_at).toLocaleString()} ·{" "}
                  {a.size_bytes === undefined
                    ? "Tamaño pendiente"
                    : `${(a.size_bytes / 1024 / 1024).toFixed(2)} MiB`}{" "}
                  · {a.status}
                </p>
                <Button
                  disabled={
                    !canWrite ||
                    !granted(consents, "LOCAL_TRANSCRIPTION", now) ||
                    a.status !== "available" ||
                    Date.parse(a.retention_until) <= now ||
                    command.isPending
                  }
                  onClick={() =>
                    command.mutate({
                      url: `${path}/transcriptions`,
                      body: { artifact_id: a.id },
                    })
                  }
                >
                  Solicitar transcripción
                </Button>
              </article>
            ))
          )}
          {!granted(consents, "LOCAL_TRANSCRIPTION", now) && (
            <p>
              La transcripción requiere consentimiento LOCAL_TRANSCRIPTION
              independiente.
            </p>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>Trabajos de transcripción y análisis</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {jobs.isPending ? (
            <p role="status">Consultando trabajos…</p>
          ) : !jobs.data?.items.length ? (
            <p>No hay trabajos solicitados.</p>
          ) : (
            jobs.data.items.map((j) => (
              <JobPanel
                key={j.id}
                initial={j}
                scopeKey={key}
                writable={canWrite}
                retryConsent={granted(
                  consents,
                  j.job_type === "analyze_session"
                    ? "LOCAL_AI_PROCESSING"
                    : "LOCAL_TRANSCRIPTION",
                  now,
                )}
              />
            ))
          )}
        </CardContent>
      </Card>
      <Card>
        <CardContent className="pt-6">
          {transcripts.isPending ? (
            <p role="status">Consultando versiones…</p>
          ) : (
            <TranscriptPanel
              versions={transcripts.data?.items ?? []}
              writable={canWrite}
              busy={command.isPending}
              analyzeAllowed={
                canWrite &&
                current.status === "completed" &&
                granted(consents, "LOCAL_AI_PROCESSING", now)
              }
              correct={(id: string, segments: Segment[]) =>
                command.mutate({
                  url: `/clinical-transcripts/${id}/revisions`,
                  body: {
                    text: segments.map((s) => s.text).join(" "),
                    segments,
                  },
                })
              }
              analyze={(id) =>
                command.mutate({
                  url: `${path}/analysis-jobs`,
                  body: { transcript_version_id: id },
                })
              }
            />
          )}
        </CardContent>
      </Card>
    </div>
  );
}
