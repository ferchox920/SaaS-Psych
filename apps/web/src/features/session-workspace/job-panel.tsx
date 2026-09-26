"use client";
import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/features/auth/hooks/use-session";
import { Button } from "@/components/ui/button";
import { workspaceRequest } from "./api";
import {
  acceptJob,
  canCancel,
  canRetry,
  Job,
  jobLabel,
  safeError,
  terminal,
} from "./model";

export function JobPanel({
  initial,
  scopeKey,
  writable,
  retryConsent,
}: {
  initial: Job;
  scopeKey: readonly string[];
  writable: boolean;
  retryConsent: boolean;
}) {
  const { authenticatedRequest } = useSession();
  const cache = useQueryClient();
  const key = [...scopeKey, "job", initial.id];
  const query = useQuery({
    queryKey: key,
    initialData: initial,
    gcTime: 0,
    staleTime: 0,
    retry: false,
    queryFn: async ({ signal }) => {
      const incoming = await authenticatedRequest((a) =>
        workspaceRequest<Job>(
          a,
          `/clinical-jobs/${initial.id}`,
          "GET",
          undefined,
          signal,
        ),
      );
      return acceptJob(cache.getQueryData<Job>(key), incoming);
    },
    refetchInterval: (q) =>
      q.state.error || (q.state.data && terminal(q.state.data)) ? false : 4000,
  });
  const job = query.data;
  const scope = JSON.stringify(scopeKey);
  useEffect(() => {
    if (["succeeded", "failed", "cancelled"].includes(job.status))
      void cache.invalidateQueries({
        queryKey: JSON.parse(scope),
        predicate: (q) => !q.queryKey.includes("job"),
      });
  }, [job.status, job.id, cache, scope]);
  const mutation = useMutation({
    mutationFn: async (action: "retry" | "cancel") => {
      await cache.cancelQueries({ queryKey: key });
      const result = await authenticatedRequest((a) =>
        workspaceRequest<Job>(a, `/clinical-jobs/${job.id}/${action}`, "POST"),
      );
      cache.setQueryData(key, result);
      return result;
    },
    onSettled: () => cache.invalidateQueries({ queryKey: scopeKey }),
  });
  return (
    <article className="rounded-xl border p-4 space-y-2">
      <h4 className="font-medium">
        {job.job_type === "analyze_session" ? "Análisis" : "Transcripción"} ·{" "}
        {jobLabel(job)}
      </h4>
      <p className="text-sm">
        Intentos {job.attempt}/{job.max_attempts} · referencia{" "}
        {job.id.slice(0, 8)}
      </p>
      {job.transcript_version_id && (
        <p className="text-sm">Versión fuente: {job.transcript_version_id}</p>
      )}
      {job.error_code && (
        <p>
          {[
            "provider_unavailable",
            "sidecar_unavailable",
            "timeout",
            "worker_lost",
            "storage_unavailable",
          ].includes(job.error_code)
            ? "Servicio local temporalmente no disponible; consulta el estado del reintento."
            : "No se pudo procesar esta fuente. Revisa consentimiento y estado."}
        </p>
      )}
      {job.status === "succeeded" && (
        <p role="status">
          {job.job_type === "analyze_session"
            ? "SessionReport borrador disponible. No está aprobado."
            : "Transcripción completada; consulta las versiones."}
        </p>
      )}
      <div className="flex gap-2 flex-wrap">
        {canRetry(job) && (
          <Button
            disabled={!writable || !retryConsent || mutation.isPending}
            onClick={() => mutation.mutate("retry")}
          >
            Adelantar reintento
          </Button>
        )}
        {canRetry(job) && !retryConsent && (
          <p>
            El reintento requiere consentimiento vigente para este
            procesamiento.
          </p>
        )}
        {canCancel(job) && (
          <Button
            variant="outline"
            disabled={!writable || mutation.isPending}
            onClick={() => mutation.mutate("cancel")}
          >
            Cancelar trabajo
          </Button>
        )}
        {query.isError && (
          <Button variant="outline" onClick={() => void query.refetch()}>
            Consultar estado
          </Button>
        )}
      </div>
      {(mutation.error || query.error) && (
        <p role="alert">{safeError(mutation.error || query.error)}</p>
      )}
    </article>
  );
}
