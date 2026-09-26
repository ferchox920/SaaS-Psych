"use client";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Transcript, Segment } from "./model";

export function TranscriptPanel({
  versions,
  writable,
  busy,
  analyzeAllowed,
  correct,
  analyze,
}: {
  versions: Transcript[];
  writable: boolean;
  busy: boolean;
  analyzeAllowed: boolean;
  correct: (id: string, segments: Segment[]) => void;
  analyze: (id: string) => void;
}) {
  const [selected, setSelected] = useState("");
  const ordered = [...versions].sort(
    (a, b) => b.version - a.version || a.id.localeCompare(b.id),
  );
  const current = ordered.find((v) => v.id === selected) ?? ordered[0];
  return (
    <section className="space-y-4" aria-labelledby="transcript-title">
      <h3 id="transcript-title" className="text-xl font-semibold">
        Transcripción y versiones
      </h3>
      <p>
        La transcripción automática puede contener errores de reconocimiento o
        de hablante. No es un hecho clínico validado.
      </p>
      {!current ? (
        <p>Todavía no hay transcripciones.</p>
      ) : (
        <>
          <label className="block">
            Historial de versiones
            <select
              className="block border rounded p-2 w-full"
              value={current.id}
              onChange={(e) => setSelected(e.target.value)}
            >
              {ordered.map((v) => (
                <option key={v.id} value={v.id}>
                  v{v.version} ·{" "}
                  {v.origin === "machine"
                    ? "Original automática"
                    : "Corrección humana ASR"}{" "}
                  ·{" "}
                  {v.status === "deleted"
                    ? "Contenido eliminado"
                    : v.id === ordered[0].id
                      ? "Más reciente"
                      : "Histórica"}
                </option>
              ))}
            </select>
          </label>
          <p className="text-sm">
            v{current.version} · {new Date(current.created_at).toLocaleString()}{" "}
            · {current.engine} / {current.model}
          </p>
          <p className="text-sm break-all">
            Fuente de audio: {current.source_artifact_id}
          </p>
          {current.status !== "available" ? (
            <p>El contenido de esta versión ya no está disponible.</p>
          ) : (
            <>
              <div className="max-h-96 overflow-auto space-y-3 rounded border p-3">
                {current.segments.map((s, i) => (
                  <p key={i} className="whitespace-pre-wrap">
                    <span className="text-muted-foreground">
                      {s.start.toFixed(1)}–{s.end.toFixed(1)} s ·{" "}
                    </span>
                    {s.text}
                  </p>
                ))}
              </div>
              <Correction
                key={current.id}
                transcript={current}
                disabled={!writable || busy}
                save={correct}
              />
              <Button
                disabled={!analyzeAllowed || busy}
                onClick={() => analyze(current.id)}
              >
                Analizar esta transcripción · v{current.version}
              </Button>
              {!analyzeAllowed && (
                <p className="text-sm">
                  Requiere sesión completada, consentimiento de IA local y
                  permiso de escritura.
                </p>
              )}
            </>
          )}
        </>
      )}
    </section>
  );
}
function Correction({
  transcript,
  disabled,
  save,
}: {
  transcript: Transcript;
  disabled: boolean;
  save: (id: string, segments: Segment[]) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [segments, setSegments] = useState(transcript.segments);
  return (
    <div className="space-y-2">
      <Button
        variant="outline"
        disabled={disabled}
        onClick={() => setEditing(!editing)}
      >
        {editing ? "Cerrar corrección" : "Corregir reconocimiento ASR"}
      </Button>
      {editing && (
        <div className="space-y-3">
          <p>
            Se creará una nueva versión. La original se conserva. No edita
            evidencia clínica ni representa una corrección clínica del paciente.
          </p>
          {segments.map((s, i) => (
            <label key={i} className="block">
              Segmento {i + 1} · {s.start}–{s.end} s
              <Textarea
                disabled={disabled}
                value={s.text}
                onChange={(e) =>
                  setSegments((previous) =>
                    previous.map((v, n) =>
                      n === i ? { ...v, text: e.target.value } : v,
                    ),
                  )
                }
              />
            </label>
          ))}
          <Button
            disabled={disabled || segments.some((s) => !s.text.trim())}
            onClick={() =>
              save(
                transcript.id,
                segments.map((s) => ({ ...s, text: s.text.trim() })),
              )
            }
          >
            Guardar nueva versión
          </Button>
        </div>
      )}
    </div>
  );
}
