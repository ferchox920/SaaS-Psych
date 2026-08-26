import { AuthSession } from "@/features/auth/lib/auth-types";
import { env } from "@/lib/config/env";
import { apiFetch } from "@/lib/http/api-client";
import { AuditEnvelope, AuditFilters, AuditPageParam } from "@/types/api";

export function listAudit(
  session: AuthSession,
  filters: AuditFilters,
  pageParam?: AuditPageParam,
) {
  const params = new URLSearchParams();
  params.set("limit", String(filters.limit ?? 20));
  params.set("order", filters.order ?? "desc");

  if (filters.actionPrefix) {
    params.set("action_prefix", filters.actionPrefix);
  }
  if (filters.entity) {
    params.set("entity", filters.entity);
  }
  if (filters.from) {
    params.set("from", filters.from);
  }
  if (filters.to) {
    params.set("to", filters.to);
  }
  if (pageParam?.cursor) {
    params.set("cursor", pageParam.cursor);
  }
  if (pageParam?.cursor_id) {
    params.set("cursor_id", pageParam.cursor_id);
  }

  return apiFetch<AuditEnvelope>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: `/audit?${params.toString()}`,
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}
