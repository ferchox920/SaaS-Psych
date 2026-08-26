import { AuthSession } from "@/features/auth/lib/auth-types";
import { env } from "@/lib/config/env";
import { apiFetch } from "@/lib/http/api-client";
import { Appointment, GoogleCalendarCandidate, GoogleCalendarStatus, ListEnvelope } from "@/types/api";

export function getGoogleCalendarStatus(session: AuthSession) {
  return apiFetch<GoogleCalendarStatus>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: "/integrations/google-calendar/status",
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function beginGoogleCalendarAuthorization(session: AuthSession) {
  return apiFetch<{ authorization_url: string }>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: "/integrations/google-calendar/authorize",
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function listGoogleCalendarCandidates(session: AuthSession, range: { from: string; to: string }) {
  return apiFetch<ListEnvelope<GoogleCalendarCandidate>>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: `/integrations/google-calendar/events?from=${encodeURIComponent(range.from)}&to=${encodeURIComponent(range.to)}`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function importGoogleCalendarEvent(session: AuthSession, eventId: string, clientId: string) {
  return apiFetch<Appointment>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: `/integrations/google-calendar/events/${encodeURIComponent(eventId)}/import`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
    body: { client_id: clientId },
  });
}

export function pushAppointmentToGoogleCalendar(session: AuthSession, appointmentId: string) {
  return apiFetch<{ appointment_id: string; sync_status: string; last_synced_at: string }>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: `/integrations/google-calendar/appointments/${appointmentId}/push`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function disconnectGoogleCalendar(session: AuthSession) {
  return apiFetch<void>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "DELETE",
    path: "/integrations/google-calendar/connection",
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}
