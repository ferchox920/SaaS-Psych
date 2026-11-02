"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { reviewCalendarPatients, type PatientSuggestion } from "@/features/appointments/lib/calendar-patient-review";
import type { Client, GoogleCalendarCandidate } from "@/types/api";

const labels = { existing: "Paciente existente sugerido", possible: "Requiere confirmar coincidencia", new: "Posible paciente nuevo", unresolved: "Identidad pendiente" };

function examples(): GoogleCalendarCandidate[] {
  return ["Sesión: Diego", "Diego", "Consulta: Lucía Fernández", "Sesión", "Trámite", "Paciente: Aurora"].map((summary, i) => ({
    event_id: `example-${i}`, summary, location: "", starts_at: "2026-10-27T12:00:00-03:00", ends_at: "2026-10-27T13:00:00-03:00",
  }));
}

export function CalendarPatientReview({ events, clients, connected, ready }: { events: GoogleCalendarCandidate[]; clients: Client[]; connected: boolean; ready: boolean }) {
  const [result, setResult] = useState<{ groups: PatientSuggestion[]; milliseconds: number; source: string; count: number } | null>(null);
  const [titles, setTitles] = useState("");
  const run = (input: GoogleCalendarCandidate[], source: string) => {
    const start = performance.now();
    const groups = reviewCalendarPatients(input, clients);
    setResult({ groups, milliseconds: performance.now() - start, source, count: input.length });
  };
  return (
    <section aria-label="Revisión de posibles pacientes" className="space-y-4 rounded-2xl border p-4">
      <div>
        <h3 className="font-semibold">Revisar posibles pacientes</h3>
        <p className="text-sm text-muted-foreground">Agrupa eventos pendientes del rango activo y busca coincidencias entre tus pacientes visibles. Cada propuesta requiere revisar la identidad.</p>
      </div>
      <div className="flex flex-wrap gap-2">
        <Button type="button" disabled={!ready || !connected || events.length === 0} onClick={() => run(events, "Google Calendar · rango activo")}>Analizar eventos del calendario</Button>
        <Button type="button" disabled={!ready} variant="outline" onClick={() => run(examples(), "Ejemplos ficticios · prueba del método")}>Probar con ejemplos ficticios</Button>
      </div>
      {!connected ? <p className="text-sm text-muted-foreground">Conecta Google Calendar para revisar tu agenda real. Puedes probar el análisis pegando títulos o usando los ejemplos.</p> : null}
      {!ready ? <p role="status" className="text-sm">Esperando la lista de pacientes. Si la carga falla, vuelve a intentarlo antes de analizar.</p> : null}
      <details>
        <summary className="cursor-pointer text-sm">Probar con títulos copiados del calendario</summary>
        <label className="mt-3 block text-sm" htmlFor="calendar-review-titles">Un título por línea (máximo 500)</label>
        <textarea id="calendar-review-titles" className="my-2 min-h-24 w-full rounded-xl border p-3" maxLength={50000} value={titles} onChange={(event) => { setTitles(event.target.value); setResult(null); }} />
        <Button type="button" variant="outline" disabled={!ready || !titles.trim() || titles.split(/\r?\n/).filter((title) => title.trim()).length > 500} onClick={() => run(titles.split(/\r?\n/).map((title) => title.trim()).filter(Boolean).map((summary, i) => ({ event_id: `manual-${i}`, summary, starts_at: "", ends_at: "", location: "" })), "Títulos pegados · sin horarios verificados")}>Analizar títulos pegados</Button>
      </details>
      {result ? <div className="space-y-3" aria-live="polite">
        <p className="text-sm font-medium">{result.source}</p>
        <p className="text-sm">{result.count} eventos → {result.groups.length} grupos · {result.groups.filter((group) => group.status === "new").length} posibles altas · comparación local: {result.milliseconds.toFixed(2)} ms</p>
        <p className="text-xs text-muted-foreground">Vista previa de esta ejecución. El tiempo excluye Google y la carga de pacientes. Cambiar el rango o actualizar los datos descarta el resultado.</p>
        <p className="text-xs text-muted-foreground">Los títulos iguales se agrupan para revisar; podrían representar personas distintas. Las palabras genéricas se detectan con reglas simples que iremos ajustando.</p>
        <div className="overflow-x-auto"><table className="w-full text-left text-sm">
          <caption className="sr-only">Sugerencias de pacientes; esta revisión no escribe en la base de datos</caption>
          <thead><tr><th className="p-2">Nombre propuesto / eventos</th><th className="p-2">Resultado</th><th className="p-2">Coincidencias</th></tr></thead>
          <tbody>{result.groups.map((group) => <tr key={group.key} className="border-t align-top">
            <td className="p-2"><strong>{group.name}</strong><ul>{group.events.map((event) => <li key={event.event_id}>{event.summary || "Sin título"}{event.starts_at ? ` · ${new Date(event.starts_at).toLocaleString("es-AR", { timeZone: "America/Argentina/Buenos_Aires", hourCycle: "h23" })}` : ""}</li>)}</ul></td>
            <td className="p-2">{labels[group.status]}<p className="text-xs text-muted-foreground">{group.events.length} evento(s)</p></td>
            <td className="p-2">{group.matches.length ? <ul>{group.matches.map((match) => <li key={match.id}>{match.name}<p className="text-xs text-muted-foreground">{match.reason}</p></li>)}</ul> : group.status === "unresolved" ? "Seleccionar persona o ignorar evento" : "Sin coincidencias; revisar nombre antes de crear"}</td>
          </tr>)}</tbody>
        </table></div>
        <p className="text-sm text-muted-foreground">Esta etapa prepara la lista. Las fichas y las citas se crean después de confirmar las asociaciones.</p>
      </div> : null}
    </section>
  );
}
