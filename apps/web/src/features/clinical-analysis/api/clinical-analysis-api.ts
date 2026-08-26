import { AuthSession } from "@/features/auth/lib/auth-types";
import { env } from "@/lib/config/env";
import { ApiError, apiFetch } from "@/lib/http/api-client";
import {
  ClinicalAnalysisOutput,
	ClinicalReviewOutput,
	ClinicalFormulationAnchor,
	ClinicalFormulationSnapshot,
  ClinicalNowAction,
	ClinicalSuggestion,
	ClinicalSuggestionDisposition,
	ListEnvelope,
  LocalModelStatus,
	LocalTranscriptionResult,
	LocalTranscriptionStatus,
} from "@/types/api";

export type AnalyzeLiveInput = {
  appointment_id: string;
  fragment: string;
  previous_intervention?: string;
  previous_action?: ClinicalNowAction;
  patient_response?: string;
};

export type ClinicalStreamEvent =
  | { type: "status"; data: { state: string; message?: string } }
  | { type: "progress"; data: { state: string; generated_characters: number; elapsed_ms: number } }
  | { type: "result"; data: ClinicalAnalysisOutput }
  | { type: "done"; data: { state: string } }
  | { type: "error"; data: { code: string; message: string } };

export type ClinicalReviewStreamEvent =
  | { type: "status"; data: { state: string; message?: string } }
  | { type: "progress"; data: { state: string; generated_characters: number; elapsed_ms: number } }
  | { type: "result"; data: ClinicalReviewOutput }
  | { type: "done"; data: { state: string } }
  | { type: "error"; data: { code: string; message: string } };

export function getLocalModelStatus(session: AuthSession) {
  return apiFetch<LocalModelStatus>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: "/clinical-ai/status",
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function decideClinicalSuggestion(
  session: AuthSession,
  suggestionId: string,
  input: { disposition: Exclude<ClinicalSuggestionDisposition, "pending">; correction_text?: string; reason?: string },
) {
  return apiFetch<ClinicalSuggestion>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "PUT",
    path: `/clinical-ai/suggestions/${suggestionId}/decision`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
    body: input,
  });
}

export function listClinicalFormulations(session: AuthSession, clientId: string) {
  return apiFetch<ListEnvelope<ClinicalFormulationSnapshot>>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: `/clients/${clientId}/formulations`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function createClinicalFormulation(
  session: AuthSession,
  clientId: string,
  input: { approved_summary: string; anchors: ClinicalFormulationAnchor[] },
) {
  return apiFetch<ClinicalFormulationSnapshot>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: `/clients/${clientId}/formulations`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
    body: input,
  });
}

export function approveClinicalFormulation(session: AuthSession, clientId: string, snapshotId: string) {
  return apiFetch<ClinicalFormulationSnapshot>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: `/clients/${clientId}/formulations/${snapshotId}/approve`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function warmLocalModel(session: AuthSession) {
  return apiFetch<{ status: string }>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: "/clinical-ai/warm",
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function getLocalTranscriptionStatus(session: AuthSession) {
  return apiFetch<LocalTranscriptionStatus>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: "/clinical-transcription/status",
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export async function transcribeLocalAudio(
  session: AuthSession,
  appointmentId: string,
  audio: Blob,
  signal: AbortSignal,
) {
  const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/appointments/${appointmentId}/transcription`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": audio.type || "audio/webm",
      "X-Tenant-ID": session.tenantId,
      "X-Clinical-Audio-Consent": "true",
      Authorization: `Bearer ${session.accessToken}`,
    },
    body: audio,
    signal,
  });
  if (!response.ok) {
    let message = "No fue posible transcribir el audio localmente.";
    try {
      const payload = await response.json() as { error?: { message?: string } };
      message = payload.error?.message ?? message;
    } catch {}
    throw new ApiError(response.status, "transcription_failed", message);
  }
  return response.json() as Promise<LocalTranscriptionResult>;
}

export async function analyzeLiveStream(
  session: AuthSession,
  input: AnalyzeLiveInput,
  signal: AbortSignal,
  onEvent: (event: ClinicalStreamEvent) => void,
) {
  const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/clinical-ai/analyze-live`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "X-Tenant-ID": session.tenantId,
      Authorization: `Bearer ${session.accessToken}`,
    },
    body: JSON.stringify(input),
    signal,
  });

  if (!response.ok || !response.body) {
    throw new ApiError(response.status, "analysis_failed", "No fue posible iniciar el análisis local.");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  const dispatchBlocks = (flush = false) => {
    const blocks = buffer.split(/\r?\n\r?\n/);
    buffer = flush ? "" : blocks.pop() ?? "";
    for (const block of blocks) {
      const eventLine = block.split(/\r?\n/).find((line) => line.startsWith("event:"));
      const dataLines = block.split(/\r?\n/).filter((line) => line.startsWith("data:"));
      if (!eventLine || dataLines.length === 0) continue;
      const type = eventLine.slice(6).trim() as ClinicalStreamEvent["type"];
      const data = JSON.parse(dataLines.map((line) => line.slice(5).trimStart()).join("\n"));
      onEvent({ type, data } as ClinicalStreamEvent);
    }
  };

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    dispatchBlocks();
  }
  buffer += decoder.decode();
  if (buffer.trim()) {
    buffer += "\n\n";
    dispatchBlocks(true);
  }
}

export async function reviewSessionStream(
  session: AuthSession,
  input: { appointment_id: string; session_text: string },
  signal: AbortSignal,
  onEvent: (event: ClinicalReviewStreamEvent) => void,
) {
  const response = await fetch(`${env.NEXT_PUBLIC_API_URL}/clinical-ai/review-session`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", "X-Tenant-ID": session.tenantId, Authorization: `Bearer ${session.accessToken}` },
    body: JSON.stringify(input),
    signal,
  });
  if (!response.ok || !response.body) throw new ApiError(response.status, "review_failed", "No fue posible iniciar la revisión local.");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  const dispatch = (flush = false) => {
    const blocks = buffer.split(/\r?\n\r?\n/);
    buffer = flush ? "" : blocks.pop() ?? "";
    for (const block of blocks) {
      const eventLine = block.split(/\r?\n/).find((line) => line.startsWith("event:"));
      const dataLines = block.split(/\r?\n/).filter((line) => line.startsWith("data:"));
      if (!eventLine || dataLines.length === 0) continue;
      const type = eventLine.slice(6).trim() as ClinicalReviewStreamEvent["type"];
      const data = JSON.parse(dataLines.map((line) => line.slice(5).trimStart()).join("\n"));
      onEvent({ type, data } as ClinicalReviewStreamEvent);
    }
  };
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    dispatch();
  }
  buffer += decoder.decode();
  if (buffer.trim()) { buffer += "\n\n"; dispatch(true); }
}
