import type { AuthSession } from "@/features/auth/lib/auth-types";
import { apiFetch } from "@/lib/http/api-client";
import { env } from "@/lib/config/env";
import type { ClinicalSession } from "./model";
export function workspaceRequest<T>(
  auth: AuthSession,
  path: string,
  method: "GET" | "POST" = "GET",
  body?: unknown,
  signal?: AbortSignal,
) {
  return apiFetch<T>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    path,
    method,
    body,
    signal,
    tenantId: auth.tenantId,
    accessToken: auth.accessToken,
  });
}
export async function listClinicalSessions(auth: AuthSession, clientId: string, signal?: AbortSignal): Promise<{ items: ClinicalSession[]; can_write: boolean }> {
  const items: ClinicalSession[] = [];
  let offset = 0;
  let canWrite = false;
  for (;;) {
    const page = await workspaceRequest<{ items: ClinicalSession[]; can_write: boolean; next_offset?: number | null }>(
      auth, `/clients/${clientId}/clinical-sessions?limit=100&offset=${offset}`, "GET", undefined, signal,
    );
    if (offset === 0) canWrite = page.can_write;
    items.push(...page.items);
    if (page.next_offset == null) return { items, can_write: canWrite };
    if (page.next_offset <= offset || page.next_offset > 1_000_000) throw new Error("Invalid session pagination response");
    offset = page.next_offset;
  }
}
export function uploadAudio<T>(
  auth: AuthSession,
  sessionId: string,
  audio: Blob,
  mime: string,
  signal?: AbortSignal,
) {
  return apiFetch<T>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    path: `/clinical-sessions/${sessionId}/audio`,
    method: "POST",
    rawBody: audio,
    headers: { "Content-Type": mime },
    signal,
    tenantId: auth.tenantId,
    accessToken: auth.accessToken,
  });
}
