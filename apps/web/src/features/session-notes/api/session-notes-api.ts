import { AuthSession } from "@/features/auth/lib/auth-types";
import { env } from "@/lib/config/env";
import { apiFetch } from "@/lib/http/api-client";
import { ListEnvelope, SessionNote, SessionNoteUpsertInput } from "@/types/api";

export function listSessionNotes(session: AuthSession, appointmentId: string) {
  return apiFetch<ListEnvelope<SessionNote>>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: `/appointments/${appointmentId}/notes`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function createSessionNote(
  session: AuthSession,
  appointmentId: string,
  input: SessionNoteUpsertInput,
) {
  return apiFetch<SessionNote>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: `/appointments/${appointmentId}/notes`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
    body: input,
  });
}

export function updateSessionNote(session: AuthSession, noteId: string, input: SessionNoteUpsertInput) {
  return apiFetch<SessionNote>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "PUT",
    path: `/notes/${noteId}`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
    body: input,
  });
}

export type SessionNoteVersion = {
  id: string; note_id: string; version: number; body: string; is_private: boolean;
  change_kind: string; change_reason: string; actor_user_id: string; created_at: string;
};

export function signSessionNote(session: AuthSession, noteId: string) {
  return apiFetch<SessionNote>({ baseUrl: env.NEXT_PUBLIC_API_URL, method: "POST", path: `/notes/${noteId}/sign`, tenantId: session.tenantId, accessToken: session.accessToken });
}

export function addSessionNoteAddendum(session: AuthSession, noteId: string, body: string, reason: string) {
  return apiFetch<SessionNote>({ baseUrl: env.NEXT_PUBLIC_API_URL, method: "POST", path: `/notes/${noteId}/addenda`, tenantId: session.tenantId, accessToken: session.accessToken, body: { body, reason } });
}

export function listSessionNoteVersions(session: AuthSession, noteId: string) {
  return apiFetch<ListEnvelope<SessionNoteVersion>>({ baseUrl: env.NEXT_PUBLIC_API_URL, method: "GET", path: `/notes/${noteId}/versions`, tenantId: session.tenantId, accessToken: session.accessToken });
}
