import { AuthSession } from "@/features/auth/lib/auth-types";
import { env } from "@/lib/config/env";
import { apiFetch } from "@/lib/http/api-client";
import { fetchAllPages } from "@/lib/http/fetch-all-pages";
import { Client, ClientUpsertInput, ListEnvelope } from "@/types/api";

export function listClientPage(session: AuthSession, offset = 0) {
  return apiFetch<ListEnvelope<Client>>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: `/clients?limit=100&offset=${offset}`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}

export function listClients(session: AuthSession) {
  return fetchAllPages((offset) => listClientPage(session, offset));
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
	return fetchAllPages((offset) => apiFetch<ListEnvelope<Client>>({
		baseUrl: env.NEXT_PUBLIC_API_URL,
		method: "GET",
		path: `/clients/archived?limit=100&offset=${offset}`,
		tenantId: session.tenantId,
		accessToken: session.accessToken,
	}));
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
