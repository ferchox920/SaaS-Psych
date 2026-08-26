import { AuthSession } from "@/features/auth/lib/auth-types";
import { env } from "@/lib/config/env";
import { apiFetch } from "@/lib/http/api-client";
import { Client, ClientUpsertInput, ListEnvelope } from "@/types/api";

export function listClients(session: AuthSession) {
  return apiFetch<ListEnvelope<Client>>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: "/clients",
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function createClient(session: AuthSession, input: ClientUpsertInput) {
  return apiFetch<Client>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: "/clients",
    tenantId: session.tenantId,
    accessToken: session.accessToken,
    body: input,
  });
}

export function updateClient(session: AuthSession, clientId: string, input: ClientUpsertInput) {
  return apiFetch<Client>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "PUT",
    path: `/clients/${clientId}`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
    body: input,
  });
}

export function archiveClient(session: AuthSession, clientId: string, reason: string) {
  return apiFetch<void>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
	method: "POST",
	path: `/clients/${clientId}/archive`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
	body: { reason },
  });
}

export function listArchivedClients(session: AuthSession) {
	return apiFetch<ListEnvelope<Client>>({
		baseUrl: env.NEXT_PUBLIC_API_URL,
		method: "GET",
		path: "/clients/archived",
		tenantId: session.tenantId,
		accessToken: session.accessToken,
	});
}

export function restoreClient(session: AuthSession, clientId: string) {
	return apiFetch<void>({
		baseUrl: env.NEXT_PUBLIC_API_URL,
		method: "POST",
		path: `/clients/${clientId}/restore`,
		tenantId: session.tenantId,
		accessToken: session.accessToken,
	});
}
