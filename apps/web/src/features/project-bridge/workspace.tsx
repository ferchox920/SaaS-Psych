"use client";
import Link from "next/link";
import { useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/features/auth/hooks/use-session";
import { apiFetch, ApiError } from "@/lib/http/api-client";
import { env } from "@/lib/config/env";
import { Button } from "@/components/ui/button";
import { Consent, granted } from "../session-workspace/model";
import type { Process, Diff } from "../clinical-review-workspace/model";
import {
  Fields,
  ReadError,
  useClinicalClock,
  useClinicalRead,
} from "../clinical-review-workspace/shared";
import {
  ExportRecord,
  ExportPage,
  IMPORT_LIMIT,
  byteSize,
  importInputError,
  bridgeError,
} from "./model";

export function ProjectBridge({
  client,
  write,
  onDiff,
}: {
  client: string;
  write: boolean;
  onDiff: (id: string) => void;
}) {
  const { authenticatedRequest } = useSession();
  const cache = useQueryClient();
  const [offset, setOffset] = useState(0),
    [process, setProcess] = useState("");
  const history = useClinicalRead<ExportPage>(
    client,
    `/clients/${client}/project-exports?offset=${offset}`,
  );
  const processes = useClinicalRead<{ items: Process[] }>(
    client,
    `/clients/${client}/processes`,
  );
  const consents = useClinicalRead<{ items: Consent[] }>(
    client,
    `/clients/${client}/consents`,
  );
  const now = useClinicalClock();
  const allowed =
    !!history.data &&
    !history.error &&
    (history.data.requires_external_manual_consent === false ||
      (!consents.error &&
        granted(
          consents.data?.items ?? [],
          "EXTERNAL_MANUAL_AI_PROCESSING",
          now,
        )));
  const [snapshot, setSnapshot] = useState<ExportRecord>();
  const [ack, setAck] = useState(false),
    [raw, setRaw] = useState("");
  const [result, setResult] = useState<Diff>();
  const [message, setMessage] = useState(""),
    [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const inFlight = useRef(false);
  const api = <T,>(
    path: string,
    method: "GET" | "POST",
    body?: unknown,
    rawBody?: Blob,
  ) =>
    authenticatedRequest((a) =>
      apiFetch<T>({
        baseUrl: env.NEXT_PUBLIC_API_URL,
        path,
        method,
        body,
        rawBody,
        tenantId: a.tenantId,
        accessToken: a.accessToken,
      }),
    );
  const act = async (fn: () => Promise<void>) => {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      await fn();
    } catch (e) {
      if (e instanceof ApiError && (e.status === 403 || e.status === 404)) {
        setSnapshot(undefined);
        setAck(false);
      }
      setError(bridgeError(e));
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  };
  const view = (value: ExportRecord) => {
    if (value.client_id !== client) throw new Error("scope");
    setSnapshot(value);
    setAck(false);
  };
  const generate = () =>
    act(async () => {
      const v = await api<ExportRecord>(
        `/clients/${client}/project-exports`,
        "POST",
        { process_id: process },
      );
      view(v);
      await cache.invalidateQueries({ queryKey: ["clinical-review"] });
    });
  const inspect = (id: string) =>
    act(async () =>
      view(
        await api<ExportRecord>(
          `/clients/${client}/project-exports/${id}`,
          "GET",
        ),
      ),
    );
  const transfer = (download: boolean) =>
    act(async () => {
      if (!snapshot || !ack || !allowed) return;
      // Reauthorize the existing read endpoint immediately before releasing a cached artifact.
      const fresh = await api<ExportRecord>(
        `/clients/${client}/project-exports/${snapshot.export_id}`,
        "GET",
      );
      if (
        fresh.client_id !== client ||
        fresh.content_hash !== snapshot.content_hash ||
        fresh.markdown !== snapshot.markdown
      )
        throw new Error("snapshot changed");
      if (download) {
        const url = URL.createObjectURL(
          new Blob([snapshot.markdown], {
            type: "text/markdown;charset=utf-8",
          }),
        );
        const anchor = document.createElement("a");
        anchor.href = url;
        anchor.download = `clinical-project-export-v1-${snapshot.content_hash.slice(0, 12)}.md`;
        anchor.click();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
        setMessage("Archivo generado localmente para transferencia manual.");
      } else {
        await navigator.clipboard.writeText(snapshot.markdown);
        setMessage("Copiado al portapapeles.");
      }
    });
  const submit = () =>
    act(async () => {
      const invalid = importInputError(raw);
      if (invalid) {
        setError(invalid);
        return;
      }
      const d = await api<Diff>(
        `/clients/${client}/project-imports`,
        "POST",
        undefined,
        new Blob([raw], { type: "application/json" }),
      );
      if (d.client_id !== client) throw new Error("scope");
      setResult(d);
      await cache.invalidateQueries({ queryKey: ["clinical-review"] });
    });
  const loadFile = async (file?: File) => {
    if (!file) return;
    await act(async () => {
      setRaw("");
      setResult(undefined);
      setError("");
      if (file.size > IMPORT_LIMIT) {
        setError("El límite de importación es 1 MiB (UTF-8).");
        return;
      }
      try {
        const text = new TextDecoder("utf-8", {
          fatal: true,
          ignoreBOM: true,
        }).decode(await file.arrayBuffer());
        setRaw(text);
      } catch {
        setError("El archivo debe contener texto JSON UTF-8 válido.");
      }
    });
  };
  return (
    <section className="space-y-5" aria-label="GPT Project Bridge">
      <header>
        <h3 className="text-2xl font-semibold">
          GPT Project — transferencia manual
        </h3>
        <p>
          SaaS-Psi prepara → vos revisás → vos transferís → vos importás el
          resultado.
        </p>
        <p>
          El contenido que copies o descargues será transferido por vos a un
          servicio externo. Nada se ha enviado externamente.
        </p>
      </header>
      <section className="border rounded p-4 space-y-2">
        <h4>1. Consentimiento y contexto</h4>
        <p>EXTERNAL_MANUAL_AI_PROCESSING es distinto de LOCAL_AI_PROCESSING.</p>
        <p>
          {history.data?.requires_external_manual_consent === false
            ? "El servidor no exige consentimiento externo en esta configuración."
            : allowed
              ? "Consentimiento externo manual vigente."
              : "Se requiere consentimiento externo manual vigente para generar o recuperar contenido."}
        </p>
        <Link className="underline" href={`/clients/${client}/session`}>
          Gestionar consentimiento en el workspace de sesión
        </Link>
        {history.error && <ReadError error={history.error} />}
        <Button
          variant="outline"
          onClick={() => {
            void history.refetch();
            void consents.refetch();
          }}
        >
          Actualizar consentimiento y exports
        </Button>
        <label className="block">
          Seleccionar proceso clínico
          <select
            className="block border rounded p-2 max-w-full"
            value={process}
            onChange={(e) => setProcess(e.target.value)}
            disabled={!write || busy}
          >
            <option value="">Seleccioná un proceso aprobado</option>
            {(processes.data?.items ?? [])
              .filter((p) => p.approval_status === "approved")
              .map((p) => (
                <option key={p.id} value={p.id}>
                  {p.title} · {p.approval_status} · {p.clinical_status} · v
                  {p.version}
                </option>
              ))}
          </select>
        </label>
        {processes.error && <ReadError error={processes.error} />}
        <p>
          Modo fijo: MINIMIZED. Incluye runtime del proceso seleccionado,
          evidencia activa vinculada, eventos e hipótesis aprobados,
          targets/goals y GIRA aprobada vigente según el selector V1. No incluye
          reportes completos ni todo el historial.
        </p>
        <Button
          disabled={!write || !allowed || !process || !!processes.error || busy}
          onClick={() => void generate()}
        >
          Generar contexto para GPT Project
        </Button>
      </section>
      {snapshot && allowed && (
        <section
          className="border rounded p-4 space-y-3"
          aria-label="Preview de export"
        >
          <h4>2. Preview de privacidad — nada se ha enviado externamente</h4>
          <p>
            Aliases portables en lugar de IDs clínicos internos; se excluyen
            procesos no relacionados. Los identificadores directos se minimizan.
          </p>
          <p className="font-semibold">
            La minimización no garantiza anonimato. La narrativa clínica puede
            seguir siendo identificable, incluso respecto de terceros. Si
            encontrás datos indebidos, no transfieras este snapshot.
          </p>
          <p>
            {byteSize(snapshot.markdown)} bytes · state v
            {snapshot.state_version}
          </p>
          <div data-testid="portable-preview">
            <Fields value={snapshot.artifact} />
          </div>
          <details>
            <summary>
              Contenido canónico completo que se copia o descarga (Markdown)
            </summary>
            <pre
              className="whitespace-pre-wrap break-all text-sm"
              data-testid="canonical-artifact"
            >
              {snapshot.markdown}
            </pre>
          </details>
          <p className="break-all">
            Hash del artifact: {snapshot.content_hash}
          </p>
          <details>
            <summary>Recibo local de correlación para el import</summary>
            <p className="break-all">source_export_id: {snapshot.export_id}</p>
            <p className="break-all">
              source_export_hash: {snapshot.content_hash}
            </p>
            <p>
              El UUID del recibo identifica este export, no al paciente. No
              forma parte del artifact clínico. Transcribí estos campos al sobre
              de importación.
            </p>
          </details>
          <label className="flex gap-2">
            <input
              type="checkbox"
              checked={ack}
              onChange={(e) => setAck(e.target.checked)}
            />
            Revisé todo el contenido y la privacidad antes de transferirlo
            manualmente.
          </label>
          <div className="flex flex-wrap gap-2">
            <Button
              disabled={!ack || busy}
              onClick={() => void transfer(false)}
            >
              Copiar para GPT Project
            </Button>
            <Button
              variant="outline"
              disabled={!ack || busy}
              onClick={() => void transfer(true)}
            >
              Descargar artifact Markdown
            </Button>
          </div>
          <p>
            Snapshot inmutable: para cambiar el contenido, generá un nuevo
            export. No se marca como usado ni se sincroniza.
          </p>
        </section>
      )}
      <section className="border rounded p-4 space-y-2">
        <h4>3. Historial de exports locales</h4>
        {!history.data && !history.error && (
          <p role="status">Consultando exports…</p>
        )}
        {history.data?.items.length === 0 && <p>No hay exports generados.</p>}
        {history.data?.items.map((x) => (
          <article key={x.export_id} className="border-b p-2 break-all">
            <p>
              {new Date(x.generated_at).toLocaleString()} · {x.process_ref}{" "}
              (alias propio del snapshot) · state v{x.state_version}
            </p>
            <p>Hash: {x.content_hash}</p>
            <Button
              variant="outline"
              disabled={!allowed || busy}
              onClick={() => void inspect(x.export_id)}
            >
              Inspeccionar snapshot {x.content_hash.slice(0, 8)}
            </Button>
          </article>
        ))}
        <Button
          variant="outline"
          disabled={offset === 0 || busy}
          onClick={() => setOffset(Math.max(0, offset - 25))}
        >
          Exports anteriores de la página
        </Button>
        <Button
          variant="outline"
          disabled={!history.data?.has_more || busy}
          onClick={() => setOffset(offset + 25)}
        >
          Más exports históricos
        </Button>
      </section>
      <section className="border rounded p-4 space-y-3">
        <h4>4. Importar análisis desde GPT Project</h4>
        <p>
          Entrada externa no confiable. Pegá ClinicalProjectImportV1
          (clinical-project-import-v1), con recibo/hash del export. Solo JSON;
          no se reparan estructura ni referencias. Límite: 1 MiB.
        </p>
        <details>
          <summary>Provenance opcional declarada por terapeuta</summary>
          <p>
            Incluí source_type=manual_external_ai y
            provenance_assertion=user_supplied dentro de provenance.
            provider_name, model_name, surface y project_context_version son
            opcionales (máximo 200 bytes cada uno). Se registran como declarados
            por vos, no verificados. Editalos explícitamente en el JSON original
            antes de enviarlo; la UI no lo reescribe.
          </p>
        </details>
        <label className="block">
          Resultado estructurado JSON
          <textarea
            className="block w-full border rounded p-2 min-h-48 font-mono"
            value={raw}
            disabled={!write || busy}
            onChange={(e) => {
              setRaw(e.target.value);
              setResult(undefined);
            }}
          />
        </label>
        <label className="block">
          Archivo estructurado JSON
          <input
            className="block max-w-full"
            type="file"
            accept=".json,application/json,text/plain"
            disabled={!write || busy}
            onChange={(e) => void loadFile(e.target.files?.[0])}
          />
        </label>
        <p>
          {byteSize(raw)} / {IMPORT_LIMIT} bytes. El archivo se validará antes de
          importarlo.
        </p>
        <Button
          disabled={!write || busy || !!result || !!importInputError(raw)}
          onClick={() => void submit()}
        >
          Validar e importar propuesta
        </Button>
        {raw && importInputError(raw) && (
          <p role="alert">{importInputError(raw)}</p>
        )}
        {!write && (
          <p>Solo lectura: no podés generar ni importar propuestas.</p>
        )}
        {result && (
          <div role="status">
            <p>
              Propuesta externa aceptada como ClinicalDiff {result.status}. No
              se aplicaron cambios clínicos; requiere revisión y merge humano.
            </p>
            <Button onClick={() => onDiff(result.id)}>
              Revisar propuesta clínica
            </Button>
          </div>
        )}
      </section>
      <details>
        <summary>5. Packaging manual V1</summary>
        <p>
          Skill general versionada (sin editar) → CASE_RUNTIME_PROFILE actual
          reemplazable → historia seleccionada intencionalmente → contrato
          ClinicalProjectImportV1. Un nuevo snapshot reemplaza conceptualmente
          el runtime anterior solo cuando vos actualizás el Project. No acumules
          automáticamente cada sesión ni mezcles aliases de exports diferentes.
        </p>
      </details>
      {busy && <p role="status">Procesando acción local…</p>}
      {message && <p role="status">{message}</p>}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
