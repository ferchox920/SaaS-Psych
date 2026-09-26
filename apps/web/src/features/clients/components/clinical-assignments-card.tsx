"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useSession } from "@/features/auth/hooks/use-session";
import { apiFetch, ApiError } from "@/lib/http/api-client";
import { env } from "@/lib/config/env";

type Assignment = { id: string; user_id: string; relationship: string; starts_at: string; ends_at?: string };
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function ClinicalAssignmentsCard({ clientId }: { clientId: string }) {
  const { session, authenticatedRequest } = useSession();
  const cache = useQueryClient();
  const [userId, setUserId] = useState("");
  const [relationship, setRelationship] = useState("treating");
  const admin = session?.role === "owner" || session?.role === "admin";
  const path = `/clients/${clientId}/assignments`;
  const request = <T,>(method: "GET" | "POST" | "DELETE", suffix = "", body?: unknown) =>
    authenticatedRequest((current) => apiFetch<T>({
      baseUrl: env.NEXT_PUBLIC_API_URL, path: path + suffix, method, body,
      tenantId: current.tenantId, accessToken: current.accessToken,
    }));
  const key = ["clinical-assignments", session?.tenantId, clientId];
  const assignments = useQuery({
    queryKey: key,
    enabled: admin,
    queryFn: () => request<{ items: Assignment[] }>("GET"),
    retry: false,
  });
  const grant = useMutation({
    mutationFn: () => request<Assignment>("POST", "", { user_id: userId.trim(), relationship }),
    onSuccess: async () => {
      setUserId("");
      await cache.invalidateQueries({ queryKey: key });
    },
  });
  const end = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      request<void>("DELETE", `/${id}`, { reason }),
    onSuccess: async () => {
      await cache.invalidateQueries({ queryKey: key });
    },
  });
  if (!admin) return null;
  const error = assignments.error || grant.error || end.error;
  const errorText = error instanceof ApiError && error.status === 409
    ? "La asignación ya está activa o cambió. Actualiza la lista."
    : error instanceof ApiError && error.status === 403
      ? "No tienes permiso para gestionar esta asignación."
      : "No se pudo completar la operación. Consulta el estado e inténtalo de nuevo.";
  return (
    <Card>
      <CardHeader><CardTitle>Asignaciones clínicas</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">Solo propietarios y administradores. El rol por sí solo no concede acceso clínico.</p>
        <div className="grid gap-3 sm:grid-cols-[1fr_auto_auto] sm:items-end">
          <label className="space-y-1 text-sm">ID del profesional
            <input className="block w-full rounded border p-2" value={userId} onChange={(event) => setUserId(event.target.value)} placeholder="UUID de usuario del mismo tenant" />
          </label>
          <label className="space-y-1 text-sm">Relación clínica
            <select className="block w-full rounded border p-2" value={relationship} onChange={(event) => setRelationship(event.target.value)}>
              <option value="treating">Tratante</option><option value="supervisor">Supervisor</option>
            </select>
          </label>
          <Button disabled={!uuidPattern.test(userId.trim()) || grant.isPending} onClick={() => grant.mutate()}>Asignar profesional</Button>
        </div>
        {error && <p role="alert">{errorText}</p>}
        {assignments.isPending ? <p role="status">Consultando asignaciones…</p> : null}
        {assignments.data?.items.length === 0 ? <p>No hay asignaciones registradas.</p> : null}
        <ul className="space-y-2">
          {assignments.data?.items.map((item) => <li className="flex flex-wrap items-center justify-between gap-3 rounded border p-3" key={item.id}>
            <span className="text-sm break-all">{item.user_id} · {item.relationship} · {item.ends_at ? "finalizada" : "activa"}</span>
            {!item.ends_at && <Button variant="outline" disabled={end.isPending} onClick={() => {
              const reason = window.prompt("Motivo obligatorio para finalizar la asignación:")?.trim();
              if (reason) end.mutate({ id: item.id, reason });
            }}>Finalizar asignación</Button>}
          </li>)}
        </ul>
      </CardContent>
    </Card>
  );
}
