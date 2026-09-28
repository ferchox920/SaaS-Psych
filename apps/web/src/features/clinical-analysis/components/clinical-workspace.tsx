"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, BrainCircuit, Check, Clock3, Mic, Pause, Pencil, Play, Square, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { validateAudioFile } from "@/features/clinical-analysis/lib/audio-file";
import { ClinicalJourneyNav } from "./clinical-journey-nav";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { listAppointments } from "@/features/appointments/api/appointments-api";
import { useSession } from "@/features/auth/hooks/use-session";
import {
  analyzeLiveStream,
  decideClinicalSuggestion,
	getLocalTranscriptionStatus,
  getLocalModelStatus,
  warmLocalModel,
	transcribeLocalAudio,
} from "@/features/clinical-analysis/api/clinical-analysis-api";
import { listClients } from "@/features/clients/api/clients-api";
import { ClinicalAnalysisOutput } from "@/types/api";

type Decision = "accepted" | "correcting" | "discarded" | "postponed" | null;
type RecordingState = "idle" | "recording" | "paused" | "ready" | "transcribing";

function clinicalRange() {
  const from = new Date();
  from.setDate(from.getDate() - 180);
  const to = new Date();
  to.setDate(to.getDate() + 180);
  return { from: from.toISOString(), to: to.toISOString() };
}

const trafficLabels = { green: "VERDE", yellow: "AMARILLO", red: "ROJO" } as const;
const nowLabels: Record<string, string> = {
  listen: "seguir escuchando", clarify: "aclarar", reflect: "reflejar", explore: "explorar",
  confront: "confrontar", restructure: "reestructurar", resignify: "resignificar", values: "clarificar valores",
  regulate: "regular", no_intervention: "no intervenir",
};

export function ClinicalWorkspace() {
  const queryClient = useQueryClient();
  const { authenticatedRequest } = useSession();
  const abortRef = useRef<AbortController | null>(null);
	const transcriptionAbortRef = useRef<AbortController | null>(null);
	const recorderRef = useRef<MediaRecorder | null>(null);
	const streamRef = useRef<MediaStream | null>(null);
	const chunksRef = useRef<Blob[]>([]);
  const audioFileRef = useRef<HTMLInputElement | null>(null);
  const [audioFileName, setAudioFileName] = useState("");
  const [audioFileError, setAudioFileError] = useState<string | null>(null);
  const [range] = useState(clinicalRange);
  const [clientId, setClientId] = useState("");
  const [appointmentId, setAppointmentId] = useState("");
  const [fragment, setFragment] = useState("");
  const [output, setOutput] = useState<ClinicalAnalysisOutput | null>(null);
  const [progress, setProgress] = useState("");
  const [decision, setDecision] = useState<Decision>(null);
  const [correction, setCorrection] = useState("");
  const [streamError, setStreamError] = useState<string | null>(null);
  const [isAnalyzing, setIsAnalyzing] = useState(false);
	const [audioConsent, setAudioConsent] = useState(false);
	const [recordingState, setRecordingState] = useState<RecordingState>("idle");
	const [recordedAudio, setRecordedAudio] = useState<Blob | null>(null);
	const [recordingSeconds, setRecordingSeconds] = useState(0);

  const clientsQuery = useQuery({
    queryKey: ["clients", "clinical-workspace"],
    queryFn: () => authenticatedRequest((session) => listClients(session)),
  });
  const appointmentsQuery = useQuery({
    queryKey: ["appointments", "clinical-workspace", range],
    queryFn: () => authenticatedRequest((session) => listAppointments(session, range)),
  });
  const statusQuery = useQuery({
    queryKey: ["clinical-ai", "status"],
    queryFn: () => authenticatedRequest((session) => getLocalModelStatus(session)),
    retry: false,
    refetchInterval: isAnalyzing ? false : 10_000,
  });
	const transcriptionStatusQuery = useQuery({
		queryKey: ["clinical-transcription", "status"],
		queryFn: () => authenticatedRequest((session) => getLocalTranscriptionStatus(session)),
		retry: false,
		refetchInterval: recordingState === "transcribing" ? false : 15_000,
	});
  const warmMutation = useMutation({
    mutationFn: () => authenticatedRequest((session) => warmLocalModel(session)),
    onSuccess: async () => {
      toast.success("Modelo local precargado.");
      await queryClient.invalidateQueries({ queryKey: ["clinical-ai", "status"] });
    },
  });
	const decisionMutation = useMutation({
		mutationFn: ({ disposition, correctionText = "" }: { disposition: "accepted" | "corrected" | "discarded" | "postponed"; correctionText?: string }) => {
			if (!output?.suggestion_id) throw new Error("La sugerencia no tiene un identificador persistido.");
			return authenticatedRequest((session) => decideClinicalSuggestion(session, output.suggestion_id!, {
				disposition,
				correction_text: correctionText,
			}));
		},
		onSuccess: (suggestion) => {
			setDecision(suggestion.disposition === "corrected" ? "correcting" : suggestion.disposition as Decision);
			toast.success("Decisión registrada por separado; la formulación clínica no fue modificada.");
		},
		onError: (error) => toast.error(error instanceof Error ? error.message : "No se pudo registrar la decisión."),
	});

  const clients = clientsQuery.data?.items ?? [];
  const appointments = useMemo(
    () => (appointmentsQuery.data?.items ?? []).filter((item) => !clientId || item.client_id === clientId),
    [appointmentsQuery.data?.items, clientId],
  );
  const selectedAppointment = appointments.find((item) => item.id === appointmentId);
  const modelLoaded = statusQuery.data?.models?.some((model) => model.configured && model.loaded) ?? false;
	const transcriptionAvailable = Boolean(transcriptionStatusQuery.data?.enabled && transcriptionStatusQuery.data?.available);

	useEffect(() => {
		if (recordingState !== "recording") return;
		const timer = window.setInterval(() => setRecordingSeconds((seconds) => {
			const next = seconds + 1;
			if (next >= 30 && recorderRef.current?.state === "recording") {
				window.setTimeout(() => recorderRef.current?.stop(), 0);
			}
			return next;
		}), 1000);
		return () => window.clearInterval(timer);
	}, [recordingState]);

	useEffect(() => () => {
		recorderRef.current?.stop();
		streamRef.current?.getTracks().forEach((track) => track.stop());
		transcriptionAbortRef.current?.abort();
	}, []);

	const startRecording = async () => {
		if (!audioConsent || !appointmentId || !transcriptionAvailable) return;
		try {
			const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
			streamRef.current = stream;
			chunksRef.current = [];
			setRecordedAudio(null);
			setRecordingSeconds(0);
			const preferred = ["audio/webm;codecs=opus", "audio/webm", "audio/ogg;codecs=opus"].find((type) => MediaRecorder.isTypeSupported(type));
			const recorder = new MediaRecorder(stream, preferred ? { mimeType: preferred } : undefined);
			recorderRef.current = recorder;
			recorder.ondataavailable = (event) => { if (event.data.size > 0) chunksRef.current.push(event.data); };
			recorder.onstop = () => {
				const blob = new Blob(chunksRef.current, { type: recorder.mimeType || "audio/webm" });
				setRecordedAudio(blob.size > 0 ? blob : null);
				setRecordingState(blob.size > 0 ? "ready" : "idle");
				stream.getTracks().forEach((track) => track.stop());
				streamRef.current = null;
				recorderRef.current = null;
				chunksRef.current = [];
			};
			recorder.start(500);
			setRecordingState("recording");
		} catch {
			toast.error("No se pudo acceder al micrófono. Revisa el permiso del navegador.");
		}
	};

	const pauseRecording = () => {
		if (recorderRef.current?.state === "recording") {
			recorderRef.current.pause();
			setRecordingState("paused");
		}
	};
	const resumeRecording = () => {
		if (recorderRef.current?.state === "paused") {
			recorderRef.current.resume();
			setRecordingState("recording");
		}
	};
	const stopRecording = () => {
		if (recorderRef.current && recorderRef.current.state !== "inactive") recorderRef.current.stop();
	};
	const discardRecording = () => {
		transcriptionAbortRef.current?.abort();
		setAudioFileName("");
		setAudioFileError(null);
		setRecordedAudio(null);
		setRecordingState("idle");
		setRecordingSeconds(0);
	};
	const transcribeRecording = async () => {
		if (!recordedAudio || !appointmentId || !audioConsent || !transcriptionAvailable || isAnalyzing) return;
		const controller = new AbortController();
		transcriptionAbortRef.current = controller;
		setRecordingState("transcribing");
		try {
			const result = await authenticatedRequest((session) => transcribeLocalAudio(session, appointmentId, recordedAudio, controller.signal));
			setFragment(result.text);
			toast.success(`Transcripción local lista en ${result.transcription_seconds.toFixed(1)} s. Revísala antes de analizar.`);
			setRecordedAudio(null);
			setRecordingState("idle");
			setRecordingSeconds(0);
		} catch (error) {
			if (controller.signal.aborted) toast.info("Transcripción cancelada; el audio temporal fue descartado.");
			else toast.error(error instanceof Error ? error.message : "Falló la transcripción local.");
			setRecordedAudio(null);
			setRecordingState("idle");
		} finally {
			setAudioFileName("");
			transcriptionAbortRef.current = null;
		}
	};

  const startAnalysis = async () => {
    if (!appointmentId || !fragment.trim()) return;
    const controller = new AbortController();
    abortRef.current = controller;
    setOutput(null);
    setDecision(null);
    setCorrection("");
    setStreamError(null);
    setProgress("Validando acceso clínico…");
    setIsAnalyzing(true);
    try {
      await authenticatedRequest((session) => analyzeLiveStream(
        session,
        { appointment_id: appointmentId, fragment: fragment.trim() },
        controller.signal,
        (event) => {
          if (event.type === "status") setProgress(event.data.message ?? "Preparando análisis local…");
          if (event.type === "progress") setProgress(`Generando y validando localmente · ${Math.round(event.data.elapsed_ms / 100) / 10}s`);
          if (event.type === "result") setOutput(event.data);
          if (event.type === "error") setStreamError(event.data.message);
          if (event.type === "done") setProgress("Análisis validado.");
        },
      ));
    } catch (error) {
      if (controller.signal.aborted) setProgress("Generación cancelada.");
      else setStreamError(error instanceof Error ? error.message : "Falló el análisis local.");
    } finally {
      abortRef.current = null;
      setIsAnalyzing(false);
      void queryClient.invalidateQueries({ queryKey: ["clinical-ai", "status"] });
    }
  };

	const markDecision = (next: Decision) => {
		if (next === "correcting") {
			setDecision(next);
			return;
		}
		if (next) decisionMutation.mutate({ disposition: next });
  };

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <Badge variant="outline">Supervisor local</Badge>
        <h2 className="text-3xl font-semibold">Bloc clínico de microprocesos</h2>
        <p className="max-w-3xl text-muted-foreground">
          Segunda capa de análisis local. El profesional conserva el juicio clínico; ninguna sugerencia se incorpora automáticamente al registro.
        </p>
      </header>

      <ClinicalJourneyNav current="/clinical-workspace" clientId={clientId} />

      <Card>
        <CardContent className="grid gap-4 pt-6 md:grid-cols-[1fr_1fr_auto] md:items-end">
          <div className="space-y-2">
            <Label htmlFor="clinical-client">Paciente</Label>
            <select id="clinical-client" className="h-11 w-full rounded-xl border bg-background px-3" value={clientId} disabled={recordingState !== "idle" || isAnalyzing} onChange={(event) => { setClientId(event.target.value); setAppointmentId(""); }}>
              <option value="">Selecciona un paciente</option>
              {clients.map((client) => <option key={client.id} value={client.id}>{client.fullname}</option>)}
            </select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="clinical-appointment">Sesión</Label>
            <select id="clinical-appointment" className="h-11 w-full rounded-xl border bg-background px-3" value={appointmentId} onChange={(event) => setAppointmentId(event.target.value)} disabled={!clientId || recordingState !== "idle" || isAnalyzing}>
              <option value="">Selecciona una sesión</option>
              {appointments.map((appointment) => <option key={appointment.id} value={appointment.id}>{new Date(appointment.starts_at).toLocaleString()} · {appointment.status}</option>)}
            </select>
          </div>
          <div className="flex items-center gap-2 rounded-xl border px-3 py-2">
            <span className={`size-2 rounded-full ${statusQuery.data?.available ? (statusQuery.data.busy ? "bg-amber-500" : "bg-emerald-500") : "bg-red-500"}`} />
            <div className="min-w-32 text-sm">
              <p className="font-medium">{statusQuery.data?.configured_model ?? "Ollama local"}</p>
              <p className="text-xs text-muted-foreground">{statusQuery.data?.busy ? "ocupado" : modelLoaded ? "cargado" : statusQuery.data?.available ? "disponible" : "sin conexión"}</p>
            </div>
            {statusQuery.data?.available && !modelLoaded ? <Button size="sm" variant="outline" disabled={warmMutation.isPending} onClick={() => warmMutation.mutate()}>Precargar</Button> : null}
          </div>
        </CardContent>
      </Card>

      <section className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]">
        <Card>
          <CardHeader>
            <CardTitle>Fragmento actual</CardTitle>
            <CardDescription>{selectedAppointment ? "Escribe o pega material de la sesión seleccionada." : "Selecciona paciente y sesión antes de analizar."}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <Textarea aria-label="Fragmento clínico" className="min-h-[420px] resize-y text-base leading-7" value={fragment} onChange={(event) => setFragment(event.target.value)} placeholder="Paciente: …\nTerapeuta: …" disabled={isAnalyzing} />
			<div className="space-y-3 rounded-2xl border bg-muted/20 p-4">
				<div className="flex flex-wrap items-center justify-between gap-3"><div><p className="font-medium">Transcripción efímera local</p><p className="text-xs text-muted-foreground">{transcriptionAvailable ? `${transcriptionStatusQuery.data?.engine} · ${transcriptionStatusQuery.data?.model} · ${transcriptionStatusQuery.data?.device}/${transcriptionStatusQuery.data?.compute_type}` : transcriptionStatusQuery.data?.enabled ? "Transcriptor local sin conexión" : "Audio deshabilitado por configuración"}</p></div>{recordingState === "recording" || recordingState === "paused" ? <Badge className="animate-pulse bg-red-600 text-white">● GRABANDO · {recordingSeconds}s</Badge> : null}</div>
				<label className="flex items-start gap-2 text-sm"><input className="mt-1" type="checkbox" checked={audioConsent} onChange={(event) => setAudioConsent(event.target.checked)} disabled={recordingState !== "idle"} /><span>Confirmo que cuento con el consentimiento aplicable para grabar o cargar este audio y transcribirlo localmente. Esta confirmación no sustituye los consentimientos registrados del paciente. El archivo original de tu dispositivo no se modifica.</span></label>
                <input ref={audioFileRef} type="file" className="sr-only" aria-label="Archivo de grabación" accept=".m4a,.mp4,.wav,.webm,.ogg,audio/mp4,audio/x-m4a,audio/wav,audio/webm,audio/ogg" disabled={!audioConsent || !appointmentId || recordingState !== "idle" || isAnalyzing} onChange={(event) => {
                  const file = event.target.files?.[0];
                  event.target.value = "";
                  if (!file || !audioConsent || !appointmentId || recordingState !== "idle" || isAnalyzing) return;
                  setAudioFileError(null);
                  try {
                    const mime = validateAudioFile(file);
                    setRecordedAudio(file.slice(0, file.size, mime));
                    setAudioFileName(file.name);
                    setRecordingSeconds(0);
                    setRecordingState("ready");
                  } catch (error) {
                    setAudioFileError(error instanceof Error ? error.message : "No se pudo cargar el archivo.");
                  }
                }} />
                {audioFileError ? <p role="alert" className="text-sm text-destructive">{audioFileError}</p> : null}
                {audioFileName && recordedAudio ? <p role="status" className="break-all text-sm">{audioFileName} · {(recordedAudio.size / 1024 / 1024).toFixed(2)} MiB · Solo en memoria del navegador</p> : null}
                <p className="text-xs text-muted-foreground">M4A, MP4, WAV, WebM u OGG · Máximo 100 MiB. Seleccionar el archivo no lo envía ni inicia la transcripción.</p>
				<div className="flex flex-wrap gap-2">
                    <Button size="sm" variant="outline" disabled={!audioConsent || !appointmentId || recordingState !== "idle" || isAnalyzing} onClick={() => audioFileRef.current?.click()}>Agregar archivo</Button>
					{recordingState === "idle" ? <Button size="sm" variant="outline" disabled={!audioConsent || !appointmentId || !transcriptionAvailable} onClick={() => void startRecording()}><Mic className="size-4" /> Iniciar grabación</Button> : null}
					{recordingState === "recording" ? <><Button size="sm" variant="outline" onClick={pauseRecording}><Pause className="size-4" /> Pausar</Button><Button size="sm" variant="destructive" onClick={stopRecording}><Square className="size-4" /> Finalizar fragmento</Button></> : null}
					{recordingState === "paused" ? <><Button size="sm" variant="outline" onClick={resumeRecording}><Play className="size-4" /> Continuar</Button><Button size="sm" variant="destructive" onClick={stopRecording}><Square className="size-4" /> Finalizar fragmento</Button></> : null}
					{recordingState === "ready" ? <><Button size="sm" disabled={!audioConsent || !transcriptionAvailable || isAnalyzing} onClick={() => void transcribeRecording()}><Mic className="size-4" /> Transcribir localmente</Button><Button size="sm" variant="outline" onClick={discardRecording}><Trash2 className="size-4" /> Descartar audio</Button></> : null}
					{recordingState === "transcribing" ? <Button size="sm" variant="destructive" onClick={discardRecording}><Square className="size-4" /> Cancelar y eliminar audio</Button> : null}
				</div>
				{recordingState === "ready" ? <p className="text-xs text-amber-700">Audio en memoria pendiente de transcripción local. El corte de 30 segundos solo aplica al micrófono, no al archivo cargado. Una consulta larga puede exceder el tiempo del servicio efímero; no se garantiza su transcripción completa por esta vía. No se enviará a Qwen. La transcripción reemplazará el texto del fragmento actual.</p> : null}
			</div>
            <div className="flex flex-wrap items-center gap-3">
              <Button disabled={!appointmentId || !fragment.trim() || isAnalyzing || !statusQuery.data?.available} onClick={() => void startAnalysis()}>
                <BrainCircuit className="size-4" /> Analizar localmente
              </Button>
              {isAnalyzing ? <Button variant="destructive" onClick={() => abortRef.current?.abort()}><Square className="size-4" /> Cancelar</Button> : null}
              <span className="text-sm text-muted-foreground">{progress}</span>
            </div>
            <p className="text-xs text-muted-foreground">El texto se procesa mediante el modelo local configurado para esta instalación. No se envía a una API externa.</p>
          </CardContent>
        </Card>

        <div className="space-y-4">
          {streamError ? <Card className="border-destructive/50"><CardContent className="flex gap-3 pt-6 text-sm"><AlertTriangle className="size-5 text-destructive" /><p>{streamError}</p></CardContent></Card> : null}
          {!output && !streamError ? <Card><CardContent className="flex min-h-[300px] flex-col items-center justify-center gap-3 text-center text-muted-foreground"><BrainCircuit className="size-10" /><p>La supervisión validada aparecerá aquí.</p><p className="max-w-sm text-xs">Durante la generación se muestra actividad, pero no interpretaciones parciales sin validar.</p></CardContent></Card> : null}
          {output ? <ClinicalResultCard output={output} decision={decision} correction={correction} isDeciding={decisionMutation.isPending} onDecision={markDecision} onCorrection={setCorrection} onSaveCorrection={() => decisionMutation.mutate({ disposition: "corrected", correctionText: correction.trim() })} /> : null}
        </div>
      </section>
    </div>
  );
}

function ClinicalResultCard({ output, decision, correction, isDeciding, onDecision, onCorrection, onSaveCorrection }: { output: ClinicalAnalysisOutput; decision: Decision; correction: string; isDeciding: boolean; onDecision: (decision: Decision) => void; onCorrection: (value: string) => void; onSaveCorrection: () => void }) {
  const result = output.result;
  const lightClass = result.hypothesis.traffic_light === "green" ? "bg-emerald-100 text-emerald-900" : result.hypothesis.traffic_light === "yellow" ? "bg-amber-100 text-amber-900" : "bg-red-100 text-red-900";
  return <Card className={result.risk.detected ? "border-red-400" : ""}>
    <CardHeader>
      <div className="flex flex-wrap items-center justify-between gap-2"><CardTitle>Supervisión validada</CardTitle><Badge className={lightClass}>{trafficLabels[result.hypothesis.traffic_light]}</Badge></div>
      <CardDescription>{output.prompt_version} · {(output.metrics.total / 1_000_000_000).toFixed(1)}s · {output.metrics.eval_tokens_per_second.toFixed(1)} tok/s{output.metrics.repaired ? " · salida reparada" : ""}</CardDescription>
    </CardHeader>
    <CardContent className="space-y-5 text-sm">
      {result.risk.detected ? <div className="rounded-2xl border border-red-300 bg-red-50 p-4 text-red-950"><p className="font-semibold">Requiere evaluación clínica humana</p><p className="mt-1">El indicador no está confirmado ni descartado.</p>{output.risk_protocol ? <p className="mt-2 rounded-lg bg-white/70 p-2"><strong>Protocolo configurado:</strong> {output.risk_protocol}</p> : null}</div> : null}
      <ResultField label="NODO" value={result.node} />
      <div><p className="text-xs font-semibold tracking-wider text-muted-foreground">HIPÓTESIS · {result.hypothesis.epistemic_level.toUpperCase()}</p><p className="mt-1 leading-6">{result.hypothesis.text}</p></div>
      <div><p className="text-xs font-semibold tracking-wider text-muted-foreground">EVIDENCIA</p><ul className="mt-2 space-y-2">{result.evidence.map((item, index) => <li key={`${item.source_id}-${index}`} className="rounded-xl bg-muted/50 p-3"><Badge variant="outline">{item.kind}</Badge><span className="ml-2">{item.summary}</span></li>)}</ul></div>
      <ResultField label="AHORA" value={nowLabels[result.now] ?? result.now} />
      <ResultField label="CUIDADO CON" value={result.caution} />
      {result.suggested_interventions.length ? <div><p className="text-xs font-semibold tracking-wider text-muted-foreground">PODRÍAS PROBAR</p><ol className="mt-2 list-decimal space-y-2 pl-5">{result.suggested_interventions.map((item) => <li key={item}>{item}</li>)}</ol></div> : null}
      {result.therapist_meta ? <ResultField label="META-TERAPEUTA" value={result.therapist_meta} /> : null}
      <div className="border-t pt-4"><p className="mb-3 text-xs text-muted-foreground">Decisión local: no modifica notas ni formulación.</p><div className="flex flex-wrap gap-2">
		<Button size="sm" disabled={isDeciding} variant={decision === "accepted" ? "default" : "outline"} onClick={() => onDecision("accepted")}><Check className="size-4" /> Aceptar</Button>
		<Button size="sm" disabled={isDeciding} variant={decision === "correcting" ? "default" : "outline"} onClick={() => onDecision("correcting")}><Pencil className="size-4" /> Corregir</Button>
		<Button size="sm" disabled={isDeciding} variant={decision === "discarded" ? "destructive" : "outline"} onClick={() => onDecision("discarded")}><Trash2 className="size-4" /> Descartar</Button>
		<Button size="sm" disabled={isDeciding} variant={decision === "postponed" ? "secondary" : "outline"} onClick={() => onDecision("postponed")}><Clock3 className="size-4" /> Posponer</Button>
		</div>{decision === "correcting" ? <div className="mt-3 space-y-2"><Textarea className="min-h-24" value={correction} onChange={(event) => onCorrection(event.target.value)} placeholder="Escribe tu corrección clínica. Se guardará separada de la formulación." /><Button size="sm" disabled={!correction.trim() || isDeciding} onClick={onSaveCorrection}>Guardar corrección separada</Button></div> : null}</div>
    </CardContent>
  </Card>;
}

function ResultField({ label, value }: { label: string; value: string }) {
  return <div><p className="text-xs font-semibold tracking-wider text-muted-foreground">{label}</p><p className="mt-1 leading-6">{value}</p></div>;
}
