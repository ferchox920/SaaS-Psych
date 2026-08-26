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
