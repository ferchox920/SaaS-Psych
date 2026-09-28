"use client";

import { useMemo, useState } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";

import { EmptyState } from "@/components/shared/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { listAudit } from "@/features/audit/api/audit-api";
import { getAuditErrorMessage } from "@/features/audit/lib/audit-error-messages";
import { auditActionFilters, auditEntityFilters, auditEntityLabel, auditSummary, auditTitle } from "@/features/audit/lib/audit-summary";
import { useSession } from "@/features/auth/hooks/use-session";
import { AuditFilters } from "@/types/api";

function getDefaultFilters(): AuditFilters {
  return {
    actionPrefix: "",
    entity: "",
    from: "",
    to: "",
    order: "desc",
    limit: 20,
  };
}

export function AuditView() {
  const { session, authenticatedRequest } = useSession();
  const allowed = ["owner", "admin"].includes(session?.role ?? "");
  const [draftFilters, setDraftFilters] = useState(getDefaultFilters);
  const [filters, setFilters] = useState(getDefaultFilters);
  const [customAction, setCustomAction] = useState(false);
  const [customEntity, setCustomEntity] = useState(false);

  const auditQuery = useInfiniteQuery({
    queryKey: ["audit", "list", filters],
    enabled: allowed,
    initialPageParam: undefined as { cursor?: string; cursor_id?: string } | undefined,
    queryFn: ({ pageParam }) =>
      authenticatedRequest((currentSession) => listAudit(currentSession, filters, pageParam)),
    getNextPageParam: (lastPage) => {
      if (!lastPage.pagination.next_cursor || !lastPage.pagination.next_cursor_id) {
        return undefined;
      }

      return {
        cursor: lastPage.pagination.next_cursor,
        cursor_id: lastPage.pagination.next_cursor_id,
      };
    },
  });

  const entries = useMemo(
    () => auditQuery.data?.pages.flatMap((page) => page.items) ?? [],
    [auditQuery.data?.pages],
  );

  if (!allowed) {
    return (
      <EmptyState
        title="Acceso restringido"
        description="La auditoría solo está disponible para las personas con permisos de administración."
      />
    );
  }

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <Badge variant="outline">Trazabilidad</Badge>
        <h2 className="text-3xl font-semibold">Auditoría</h2>
        <p className="text-muted-foreground">
          Consulta quién realizó cada cambio y cuándo ocurrió. Filtra los eventos para encontrar una decisión concreta.
        </p>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Filtros</CardTitle>
          <CardDescription>
            Busca cambios por tipo, entidad, fechas y orden de registro.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 lg:grid-cols-3">
          <div className="space-y-2">
            <Label htmlFor="actionPrefix">Tipo de cambio</Label>
            <select className="flex h-11 w-full rounded-2xl border border-input bg-white px-4 text-sm"
              id="actionPrefix"
              value={customAction ? "__custom__" : draftFilters.actionPrefix}
              onChange={(event) => {
                setCustomAction(event.target.value === "__custom__");
                setDraftFilters((current) => ({ ...current, actionPrefix: event.target.value === "__custom__" ? "" : event.target.value }));
              }}
            >
              {auditActionFilters.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
              <option value="__custom__">Código personalizado…</option>
            </select>
            {customAction && <div className="space-y-2 pt-2">
              <Label htmlFor="customActionPrefix">Código de acción</Label>
              <Input id="customActionPrefix" value={draftFilters.actionPrefix} onChange={(event) => setDraftFilters((current) => ({ ...current, actionPrefix: event.target.value }))} placeholder="Código o prefijo exacto" />
            </div>}
          </div>

          <div className="space-y-2">
            <Label htmlFor="entity">Entidad</Label>
            <select className="flex h-11 w-full rounded-2xl border border-input bg-white px-4 text-sm"
              id="entity"
              value={customEntity ? "__custom__" : draftFilters.entity}
              onChange={(event) => {
                setCustomEntity(event.target.value === "__custom__");
                setDraftFilters((current) => ({ ...current, entity: event.target.value === "__custom__" ? "" : event.target.value }));
              }}
            >
              {auditEntityFilters.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
              <option value="__custom__">Código personalizado…</option>
            </select>
            {customEntity && <div className="space-y-2 pt-2">
              <Label htmlFor="customEntity">Código de entidad</Label>
              <Input id="customEntity" value={draftFilters.entity} onChange={(event) => setDraftFilters((current) => ({ ...current, entity: event.target.value }))} placeholder="Código exacto" />
            </div>}
          </div>

          <div className="space-y-2">
            <Label htmlFor="limit">Resultados por página</Label>
            <select
              id="limit"
              className="flex h-11 w-full rounded-2xl border border-input bg-white px-4 py-2 text-sm shadow-sm outline-none transition focus-visible:ring-4 focus-visible:ring-ring"
              value={String(draftFilters.limit)}
              onChange={(event) =>
                setDraftFilters((current) => ({
                  ...current,
                  limit: Number(event.target.value),
                }))
              }
            >
              <option value="20">20</option>
              <option value="50">50</option>
              <option value="100">100</option>
            </select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="from">Desde</Label>
            <Input
              id="from"
              type="datetime-local"
              value={toLocalInput(draftFilters.from)}
              onChange={(event) =>
                setDraftFilters((current) => ({
                  ...current,
                  from: event.target.value ? new Date(event.target.value).toISOString() : "",
                }))
              }
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="to">Hasta</Label>
            <Input
              id="to"
              type="datetime-local"
              value={toLocalInput(draftFilters.to)}
              onChange={(event) =>
                setDraftFilters((current) => ({
                  ...current,
                  to: event.target.value ? new Date(event.target.value).toISOString() : "",
                }))
              }
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="order">Orden</Label>
            <select
              id="order"
              className="flex h-11 w-full rounded-2xl border border-input bg-white px-4 py-2 text-sm shadow-sm outline-none transition focus-visible:ring-4 focus-visible:ring-ring"
              value={draftFilters.order}
              onChange={(event) =>
                setDraftFilters((current) => ({
                  ...current,
                  order: event.target.value as "asc" | "desc",
                }))
              }
            >
              <option value="desc">Más recientes primero</option>
              <option value="asc">Más antiguos primero</option>
            </select>
          </div>

          <div className="flex flex-wrap gap-3 lg:col-span-3">
            <Button onClick={() => setFilters(draftFilters)} type="button">
              Aplicar filtros
            </Button>
            <Button
              onClick={() => {
                const next = getDefaultFilters();
                setDraftFilters(next);
                setFilters(next);
                setCustomAction(false);
                setCustomEntity(false);
              }}
              type="button"
              variant="outline"
            >
              Limpiar
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-4">
        {auditQuery.isLoading ? (
          Array.from({ length: 3 }).map((_, index) => (
            <div
              key={index}
              className="h-32 animate-pulse rounded-[28px] border border-border/60 bg-muted/40"
            />
          ))
        ) : null}

        {auditQuery.isError ? (
          <EmptyState
            title="No se pudo cargar la auditoría"
            description={getAuditErrorMessage(auditQuery.error)}
          />
        ) : null}

        {!auditQuery.isLoading && !auditQuery.isError && entries.length > 0 ? (
          entries.map((entry) => (
            <Card key={entry.id} className="min-w-0">
              <CardHeader className="gap-3 md:flex-row md:items-start md:justify-between">
                <div>
                  <div className="flex flex-wrap items-center gap-2">
                    <CardTitle className="text-base">{auditTitle(entry)}</CardTitle>
                    <Badge variant="secondary">{auditEntityLabel(entry.entity)}</Badge>
                  </div>
                  <CardDescription>
                    {entry.actor_user_id ? "Acción de una persona autorizada" : "Acción del sistema"}
                  </CardDescription>
                </div>
                <Badge variant="outline">{new Date(entry.created_at).toLocaleString()}</Badge>
              </CardHeader>
              <CardContent className="space-y-3 text-sm text-muted-foreground">
                <p>{auditSummary(entry)}</p>
                <details className="min-w-0 rounded-[20px] border border-border/60 bg-muted/30 p-3">
                  <summary className="cursor-pointer font-medium text-foreground focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2">Detalle técnico</summary>
                  <dl className="mt-3 grid gap-2 break-all text-xs sm:grid-cols-[auto_1fr]">
                    <dt>Acción</dt><dd>{entry.action}</dd>
                    <dt>Actor</dt><dd>{entry.actor_user_id ?? "system"}</dd>
                    <dt>Tenant</dt><dd>{entry.tenant_id}</dd>
                    <dt>Entidad</dt><dd>{entry.entity} · {entry.entity_id ?? "sin ID"}</dd>
                    <dt>Evento</dt><dd>{entry.id}</dd>
                  </dl>
                  <pre className="mt-3 max-w-full overflow-x-auto rounded-[16px] bg-muted/40 p-3 text-xs text-foreground">{JSON.stringify(entry.metadata, null, 2)}</pre>
                </details>
              </CardContent>
            </Card>
          ))
        ) : null}

        {!auditQuery.isLoading && !auditQuery.isError && entries.length === 0 ? (
          <EmptyState
            title="Sin eventos para esos filtros"
            description="Ajusta el tipo de cambio, la entidad o el rango temporal para encontrar eventos."
          />
        ) : null}
      </div>

      {auditQuery.hasNextPage ? (
        <div className="flex justify-center">
          <Button
            disabled={auditQuery.isFetchingNextPage}
            onClick={() => void auditQuery.fetchNextPage()}
            type="button"
            variant="outline"
          >
            {auditQuery.isFetchingNextPage ? "Cargando…" : "Cargar más"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

function toLocalInput(value?: string) {
  if (!value) {
    return "";
  }

  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {
    return "";
  }

  const offset = date.getTimezoneOffset();
  return new Date(date.getTime() - offset * 60_000).toISOString().slice(0, 16);
}
