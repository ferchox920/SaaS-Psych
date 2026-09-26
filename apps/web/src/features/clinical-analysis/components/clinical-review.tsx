"use client";

import { useMemo, useRef, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { AlertTriangle, BrainCircuit, Check, Clock3, Pencil, Square, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { listAppointments } from "@/features/appointments/api/appointments-api";
import { useSession } from "@/features/auth/hooks/use-session";
import { decideClinicalSuggestion, getLocalModelStatus, reviewSessionStream } from "@/features/clinical-analysis/api/clinical-analysis-api";
import { listClients } from "@/features/clients/api/clients-api";
import { ClinicalReviewOutput, ClinicalSuggestionDisposition } from "@/types/api";

function reviewRange() {
  const from = new Date(); from.setFullYear(from.getFullYear() - 2);
  const to = new Date(); to.setDate(to.getDate() + 1);
  return { from: from.toISOString(), to: to.toISOString() };
}

export function ClinicalReview() {
  const { authenticatedRequest } = useSession();
  const abortRef = useRef<AbortController | null>(null);
  const [range] = useState(reviewRange);
  const [clientId, setClientId] = useState("");
  const [appointmentId, setAppointmentId] = useState("");
  const [sessionText, setSessionText] = useState("");
  const [output, setOutput] = useState<ClinicalReviewOutput | null>(null);
  const [progress, setProgress] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isReviewing, setIsReviewing] = useState(false);
  const [correction, setCorrection] = useState("");
  const [decision, setDecision] = useState<ClinicalSuggestionDisposition>("pending");

  const clientsQuery = useQuery({ queryKey: ["clients", "clinical-review"], queryFn: () => authenticatedRequest((session) => listClients(session)) });
  const appointmentsQuery = useQuery({ queryKey: ["appointments", "clinical-review", range], queryFn: () => authenticatedRequest((session) => listAppointments(session, range)) });
  const statusQuery = useQuery({ queryKey: ["clinical-ai", "status"], queryFn: () => authenticatedRequest((session) => getLocalModelStatus(session)), retry: false, refetchInterval: isReviewing ? false : 10_000 });
  const appointments = useMemo(() => (appointmentsQuery.data?.items ?? []).filter((item) => !clientId || item.client_id === clientId), [appointmentsQuery.data?.items, clientId]);
  const decisionMutation = useMutation({
    mutationFn: (input: { disposition: Exclude<ClinicalSuggestionDisposition, "pending">; correction_text?: string }) => {
      if (!output?.suggestion_id) throw new Error("La revisión no tiene un identificador persistido.");
      return authenticatedRequest((session) => decideClinicalSuggestion(session, output.suggestion_id!, input));
    },
    onSuccess: (item) => { setDecision(item.disposition); toast.success("Decisión registrada sin modificar la formulación longitudinal."); },
    onError: (cause) => toast.error(cause instanceof Error ? cause.message : "No se pudo guardar la decisión."),
  });

  const startReview = async () => {
    if (!appointmentId || sessionText.trim().length < 20) return;
    const controller = new AbortController(); abortRef.current = controller;
    setOutput(null); setError(null); setDecision("pending"); setCorrection(""); setIsReviewing(true); setProgress("Validando acceso clínico…");
    try {
      await authenticatedRequest((session) => reviewSessionStream(session, { appointment_id: appointmentId, session_text: sessionText.trim() }, controller.signal, (event) => {
        if (event.type === "status") setProgress(event.data.message ?? "Preparando revisión…");
        if (event.type === "progress") setProgress(`Revisando localmente · ${(event.data.elapsed_ms / 1000).toFixed(1)}s`);
        if (event.type === "result") setOutput(event.data);
        if (event.type === "error") setError(event.data.message);
        if (event.type === "done") setProgress("Revisión validada.");
      }));
    } catch (cause) {
      if (controller.signal.aborted) setProgress("Revisión cancelada.");
      else setError(cause instanceof Error ? cause.message : "Falló la revisión local.");
    } finally { abortRef.current = null; setIsReviewing(false); }
  };

  return <div className="space-y-6">
    <header className="space-y-2"><Badge variant="outline">Revisión posterior local</Badge><h2 className="text-3xl font-semibold">Supervisión longitudinal de la sesión</h2><p className="max-w-3xl text-muted-foreground">Pega una sesión completa o desidentificada. El modelo local combina ese texto transitorio con la formulación aprobada; la revisión queda como sugerencia separada hasta que el profesional decida.</p></header>
    <Card><CardContent className="grid gap-4 pt-6 md:grid-cols-2"><div className="space-y-2"><Label>Paciente</Label><select className="h-11 w-full rounded-xl border bg-background px-3" value={clientId} onChange={(event) => { setClientId(event.target.value); setAppointmentId(""); }}><option value="">Selecciona un paciente</option>{(clientsQuery.data?.items ?? []).map((item) => <option key={item.id} value={item.id}>{item.fullname}</option>)}</select></div><div className="space-y-2"><Label>Sesión</Label><select className="h-11 w-full rounded-xl border bg-background px-3" value={appointmentId} onChange={(event) => setAppointmentId(event.target.value)} disabled={!clientId}><option value="">Selecciona una sesión</option>{appointments.map((item) => <option key={item.id} value={item.id}>{new Date(item.starts_at).toLocaleString()} · {item.status}</option>)}</select></div></CardContent></Card>
    <section className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]"><Card><CardHeader><CardTitle>Material completo de sesión</CardTitle><CardDescription>No se persiste ni se incorpora a una nota. Solo la salida validada se guarda como sugerencia.</CardDescription></CardHeader><CardContent className="space-y-4"><Textarea className="min-h-[520px] resize-y leading-7" value={sessionText} onChange={(event) => setSessionText(event.target.value)} disabled={isReviewing} placeholder="Paciente: …\nTerapeuta: …" /><div className="flex flex-wrap items-center gap-3"><Button disabled={!appointmentId || sessionText.trim().length < 20 || isReviewing || !statusQuery.data?.available} onClick={() => void startReview()}><BrainCircuit className="size-4" /> Revisar sesión localmente</Button>{isReviewing ? <Button variant="destructive" onClick={() => abortRef.current?.abort()}><Square className="size-4" /> Cancelar</Button> : null}<span className="text-sm text-muted-foreground">{progress}</span></div><p className="text-xs text-muted-foreground">Una solicitud del modo sesión tiene prioridad y cancela automáticamente esta revisión para liberar el modelo.</p></CardContent></Card><div className="space-y-4">{error ? <Card className="border-destructive/50"><CardContent className="flex gap-3 pt-6"><AlertTriangle className="size-5 text-destructive" />{error}</CardContent></Card> : null}{output ? <ReviewCard output={output} decision={decision} correction={correction} setCorrection={setCorrection} pending={decisionMutation.isPending} decide={(disposition, text) => decisionMutation.mutate({ disposition, correction_text: text })} /> : !error ? <Card><CardContent className="flex min-h-[320px] flex-col items-center justify-center gap-3 text-center text-muted-foreground"><BrainCircuit className="size-10" /><p>La revisión estructurada aparecerá aquí.</p></CardContent></Card> : null}</div></section>
  </div>;
}

function ReviewCard({ output, decision, correction, setCorrection, pending, decide }: { output: ClinicalReviewOutput; decision: ClinicalSuggestionDisposition; correction: string; setCorrection: (value: string) => void; pending: boolean; decide: (value: Exclude<ClinicalSuggestionDisposition, "pending">, correction?: string) => void }) {
  const result = {
    ...output.result,
    caution: output.result.risk.detected && output.risk_protocol
      ? `${output.result.caution} Protocolo configurado: ${output.risk_protocol}`
      : output.result.caution,
  };
  return <Card className={result.risk.detected ? "border-red-400" : ""}><CardHeader><CardTitle>Revisión validada</CardTitle><CardDescription>{output.prompt_version} · {(output.metrics.total / 1_000_000_000).toFixed(1)}s · {output.metrics.eval_tokens_per_second.toFixed(1)} tok/s</CardDescription></CardHeader><CardContent className="space-y-5 text-sm">{result.risk.detected ? <div className="rounded-xl border border-red-300 bg-red-50 p-3"><strong>Requiere evaluación clínica humana.</strong> El riesgo no está confirmado ni descartado.</div> : null}<Field label="FORMULACIÓN EMERGENTE" value={result.emerging_formulation} /><List label="INTERVENCIONES EFICACES" items={result.effective_interventions} /><List label="INTERVENCIONES DÉBILES / RIESGOSAS" items={result.weak_or_risky_interventions} /><Field label="CALIBRACIÓN DE HIPÓTESIS" value={result.hypothesis_calibration} /><Field label="RESPUESTAS DEL PACIENTE" value={result.patient_responses} /><Field label="ALIANZA" value={result.alliance} /><List label="PATRONES DEL TERAPEUTA" items={result.therapist_patterns} /><List label="PRÓXIMOS FOCOS" items={result.next_focus} /><List label="EVIDENCIA" items={result.evidence.map((item) => `${item.kind.toUpperCase()} · ${item.summary}`)} /><Field label="CUIDADO CON" value={result.caution} /><div className="space-y-3 border-t pt-4"><p className="text-xs text-muted-foreground">Decisión humana separada de notas y formulación.</p><div className="flex flex-wrap gap-2"><Button size="sm" variant={decision === "accepted" ? "default" : "outline"} disabled={pending} onClick={() => decide("accepted")}><Check className="size-4" /> Aceptar</Button><Button size="sm" variant={decision === "corrected" ? "default" : "outline"} disabled={pending} onClick={() => setCorrection(correction || "")}><Pencil className="size-4" /> Corregir</Button><Button size="sm" variant="outline" disabled={pending} onClick={() => decide("discarded")}><Trash2 className="size-4" /> Descartar</Button><Button size="sm" variant="outline" disabled={pending} onClick={() => decide("postponed")}><Clock3 className="size-4" /> Posponer</Button></div><Textarea className="min-h-20" value={correction} onChange={(event) => setCorrection(event.target.value)} placeholder="Corrección clínica opcional" /><Button size="sm" disabled={!correction.trim() || pending} onClick={() => decide("corrected", correction.trim())}>Guardar corrección</Button></div></CardContent></Card>;
}

function Field({ label, value }: { label: string; value: string }) { return <div><p className="text-xs font-semibold tracking-wider text-muted-foreground">{label}</p><p className="mt-1 leading-6">{value || "Sin elementos suficientes."}</p></div>; }
function List({ label, items }: { label: string; items: string[] }) { return <div><p className="text-xs font-semibold tracking-wider text-muted-foreground">{label}</p>{items.length ? <ul className="mt-2 list-disc space-y-1 pl-5">{items.map((item, index) => <li key={`${index}-${item}`}>{item}</li>)}</ul> : <p className="mt-1 text-muted-foreground">Sin elementos suficientes.</p>}</div>; }
