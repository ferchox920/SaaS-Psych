export const scopes = {
  LOCAL_AI_PROCESSING: "Procesamiento de IA local",
  AUDIO_RECORDING: "Grabación y almacenamiento de audio",
  LOCAL_TRANSCRIPTION: "Transcripción local",
  EXTERNAL_MANUAL_AI_PROCESSING: "Procesamiento externo manual de IA",
} as const;
export type Scope = keyof typeof scopes;
export type Consent = {
  id: string;
  scope: Scope;
  status: "granted" | "revoked";
  definition_version: number;
  granted_at: string;
  revoked_at?: string;
  effective_from: string;
};
export type ClinicalSession = {
  id: string;
  client_id: string;
  status: "in_progress" | "completed" | "voided";
  appointment_id?: string;
  started_at: string;
  ended_at?: string;
};
export type Artifact = {
  id: string;
  status: string;
  created_at: string;
  size_bytes?: number;
  retention_until: string;
};
export type Segment = { start: number; end: number; text: string };
export type Transcript = {
  id: string;
  version: number;
  origin: "machine" | "human_asr_correction";
  status: string;
  text: string;
  segments: Segment[];
  created_at: string;
  source_artifact_id: string;
  engine: string;
  model: string;
  parent_version_id?: string;
};
export type Job = {
  id: string;
  session_id: string;
  job_type: string;
  status: "queued" | "running" | "succeeded" | "failed" | "cancelled";
  attempt: number;
  max_attempts: number;
  error_code?: string;
  transcript_version_id?: string;
  artifact_id?: string;
};
export const MAX_AUDIO_BYTES = 25 * 1024 * 1024;
export const audioTypes = [
  "audio/wav",
  "audio/webm",
  "audio/ogg",
  "audio/mp4",
  "audio/x-m4a",
];
export function validateAudio(size: number, type: string) {
  return size < 1
    ? "El archivo está vacío."
    : size > MAX_AUDIO_BYTES
      ? "El límite de audio es 25 MiB."
      : !audioTypes.includes(type)
        ? "Formato no compatible. Usa WAV, WebM, OGG, MP4 o M4A."
        : null;
}
export function granted(items: Consent[], scope: Scope, now: number) {
  return items.some(
    (c) =>
      c.scope === scope &&
      c.status === "granted" &&
      Date.parse(c.effective_from) <= now,
  );
}
export function terminal(j: Job) {
  return ["succeeded", "failed", "cancelled"].includes(j.status);
}
// Mirrors the public RetryJob contract: expedite a queued retry, never reopen a terminal.
export function canRetry(j: Job) {
  return (
    j.status === "queued" &&
    j.attempt < j.max_attempts &&
    [
      "sidecar_unavailable",
      "provider_unavailable",
      "timeout",
      "worker_lost",
      "storage_unavailable",
    ].includes(j.error_code ?? "")
  );
}
export function canCancel(j: Job) {
  return ["queued", "running"].includes(j.status);
}
export function acceptJob(previous: Job | undefined, incoming: Job) {
  return previous && terminal(previous) ? previous : incoming;
}
export function jobLabel(j: Job) {
  return canRetry(j)
    ? "Reintento programado"
    : {
        queued: "En cola",
        running: "En ejecución",
        succeeded: "Completado",
        failed: "Fallido",
        cancelled: "Cancelado",
      }[j.status];
}
export function safeError(error: unknown): string {
  const e = error as { status?: number; code?: string } | null;
  if (e?.status === 401)
    return "La sesión de acceso venció. Vuelve a iniciar sesión.";
  if (e?.status === 403)
    return "No autorizado. Revisa la asignación clínica y el consentimiento de este alcance.";
  if (e?.status === 404)
    return "Recurso no disponible para este paciente, o ingestión no habilitada.";
  if (e?.status === 409)
    return "El estado cambió o el trabajo es terminal. Actualiza antes de continuar; no se reinició el presupuesto.";
  if (e?.status === 413)
    return "El audio supera el límite permitido de 25 MiB.";
  if ([400, 415, 422].includes(e?.status ?? 0))
    return "La operación o el archivo no es válido. Revisa formato, tamaño y contenido.";
  if (
    e?.status === 503 ||
    ["provider_unavailable", "sidecar_unavailable"].includes(e?.code ?? "")
  )
    return "El servicio local no está disponible temporalmente.";
  return "No se pudo completar la operación. Actualiza el estado antes de reintentar.";
}
export function workspaceKey(
  tenant: string,
  user: string,
  client: string,
  session = "",
) {
  return ["session-workspace", tenant, user, client, session] as const;
}
