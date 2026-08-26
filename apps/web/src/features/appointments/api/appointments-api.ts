import { AuthSession } from "@/features/auth/lib/auth-types";
import { env } from "@/lib/config/env";
import { apiFetch } from "@/lib/http/api-client";
import {
  Appointment,
  AppointmentCreateInput,
  AppointmentUpdateInput,
  ListEnvelope,
} from "@/types/api";

export function listAppointments(
  session: AuthSession,
  params: {
    from: string;
    to: string;
  },
) {
  return apiFetch<ListEnvelope<Appointment>>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: `/appointments?from=${encodeURIComponent(params.from)}&to=${encodeURIComponent(params.to)}`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function createAppointment(session: AuthSession, input: AppointmentCreateInput) {
  return apiFetch<Appointment>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: "/appointments",
    tenantId: session.tenantId,
    accessToken: session.accessToken,
    body: input,
  });
}

export function updateAppointment(session: AuthSession, appointmentId: string, input: AppointmentUpdateInput) {
  return apiFetch<Appointment>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "PUT",
    path: `/appointments/${appointmentId}`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
    body: input,
  });
}

export function cancelAppointment(session: AuthSession, appointmentId: string) {
  return apiFetch<Appointment>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: `/appointments/${appointmentId}/cancel`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}
