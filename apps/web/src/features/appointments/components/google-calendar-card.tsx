"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CalendarSync, ExternalLink, Link2, RefreshCw, Unplug } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  beginGoogleCalendarAuthorization,
  disconnectGoogleCalendar,
  getGoogleCalendarStatus,
  importGoogleCalendarEvent,
  listGoogleCalendarCandidates,
  pushAppointmentToGoogleCalendar,
} from "@/features/appointments/api/google-calendar-api";
import { useSession } from "@/features/auth/hooks/use-session";
import { ApiError } from "@/lib/http/api-client";
import { Client } from "@/types/api";

type Props = {
  clients: Client[];
  range: { from: string; to: string };
  onImported: (appointmentId: string) => Promise<void>;
  activeAppointmentId: string | null;
};

export function GoogleCalendarCard({ clients, range, onImported, activeAppointmentId }: Props) {
  const queryClient = useQueryClient();
  const { authenticatedRequest } = useSession();
  const [clientByEvent, setClientByEvent] = useState<Record<string, string>>({});

  useEffect(() => {
    const current = new URL(window.location.href);
    const result = current.searchParams.get("google_calendar");
    if (!result) return;
    if (result === "connected") toast.success("Google Calendar conectado.");
    else if (result === "denied") toast.info("La autorización de Google Calendar fue cancelada.");
    else toast.error("No se pudo completar la conexión con Google Calendar.");
    current.searchParams.delete("google_calendar");
    window.history.replaceState({}, "", current.pathname + current.search + current.hash);
    void queryClient.invalidateQueries({ queryKey: ["google-calendar"] });
  }, [queryClient]);

  const statusQuery = useQuery({
    queryKey: ["google-calendar", "status"],
    queryFn: () => authenticatedRequest((session) => getGoogleCalendarStatus(session)),
  });
  const connected = statusQuery.data?.connected ?? false;
  const candidatesQuery = useQuery({
    queryKey: ["google-calendar", "candidates", range],
    queryFn: () => authenticatedRequest((session) => listGoogleCalendarCandidates(session, range)),
    enabled: connected,
  });

  const connectMutation = useMutation({
    mutationFn: () => authenticatedRequest((session) => beginGoogleCalendarAuthorization(session)),
    onSuccess: ({ authorization_url }) => window.location.assign(authorization_url),
  });
  const disconnectMutation = useMutation({
    mutationFn: () => authenticatedRequest((session) => disconnectGoogleCalendar(session)),
    onSuccess: async () => {
      toast.success("Google Calendar desconectado.");
      await queryClient.invalidateQueries({ queryKey: ["google-calendar"] });
    },
  });
  const importMutation = useMutation({
    mutationFn: ({ eventId, clientId }: { eventId: string; clientId: string }) =>
      authenticatedRequest((session) => importGoogleCalendarEvent(session, eventId, clientId)),
    onSuccess: async (appointment) => {
      toast.success("Evento asociado y turno creado.");
      await queryClient.invalidateQueries({ queryKey: ["google-calendar", "candidates"] });
      await onImported(appointment.id);
    },
  });
  const pushMutation = useMutation({
    mutationFn: (appointmentId: string) => authenticatedRequest((session) => pushAppointmentToGoogleCalendar(session, appointmentId)),
    onSuccess: () => toast.success("Turno sincronizado con Google Calendar."),
  });

  const error = statusQuery.error || candidatesQuery.error || connectMutation.error || disconnectMutation.error || importMutation.error || pushMutation.error;
  const errorMessage = error instanceof ApiError ? error.message : error ? "No se pudo completar la operación con Google Calendar." : null;
  const events = candidatesQuery.data?.items ?? [];

  return (
    <Card>
      <CardHeader className="gap-3 md:flex-row md:items-start md:justify-between">
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2"><CalendarSync className="size-5" /> Google Calendar</CardTitle>
          <CardDescription>
            Importa eventos y asócialos manualmente a un paciente. La identidad del paciente permanece solo en SessionFlow.
          </CardDescription>
        </div>
        <Badge variant={connected ? "secondary" : "outline"}>
          {!statusQuery.data?.enabled ? "no configurado" : connected ? "conectado" : statusQuery.data?.reauthorization_required ? "reautorizar" : "desconectado"}
        </Badge>
      </CardHeader>
      <CardContent className="space-y-4">
        {statusQuery.data?.enabled && !connected ? (
          <Button disabled={connectMutation.isPending} onClick={() => connectMutation.mutate()} type="button">
            <Link2 className="size-4" /> {statusQuery.data.reauthorization_required ? "Reautorizar Google" : "Conectar Google Calendar"}
          </Button>
        ) : null}
        {!statusQuery.data?.enabled && !statusQuery.isLoading ? (
          <p className="text-sm text-muted-foreground">La integración está instalada pero requiere las credenciales OAuth de Google Cloud.</p>
        ) : null}
        {connected ? (
          <div className="flex flex-wrap gap-2">
            <Button disabled={candidatesQuery.isFetching} onClick={() => void candidatesQuery.refetch()} type="button" variant="outline">
              <RefreshCw className={candidatesQuery.isFetching ? "size-4 animate-spin" : "size-4"} /> Actualizar eventos
            </Button>
            <Button disabled={disconnectMutation.isPending} onClick={() => disconnectMutation.mutate()} type="button" variant="outline">
              <Unplug className="size-4" /> Desconectar
            </Button>
            {activeAppointmentId ? (
              <Button disabled={pushMutation.isPending} onClick={() => pushMutation.mutate(activeAppointmentId)} type="button">
                <CalendarSync className="size-4" /> Sincronizar turno seleccionado
              </Button>
            ) : null}
          </div>
        ) : null}
        {errorMessage ? <p className="text-sm text-destructive">{errorMessage}</p> : null}
        {connected && !candidatesQuery.isLoading && events.length === 0 ? (
          <p className="text-sm text-muted-foreground">No hay eventos de Google pendientes de asociación en el rango activo.</p>
        ) : null}
        {events.length > 0 ? (
          <div className="grid gap-3">
            {events.map((event) => (
              <div className="grid gap-3 rounded-2xl border p-4 lg:grid-cols-[1fr_240px_auto] lg:items-center" key={event.event_id}>
                <div>
                  <p className="font-medium">{event.summary || "Evento sin título"}</p>
                  <p className="text-sm text-muted-foreground">
                    {new Date(event.starts_at).toLocaleString()} – {new Date(event.ends_at).toLocaleTimeString()}
                    {event.location ? ` · ${event.location}` : ""}
                  </p>
                  {event.html_link ? <a className="inline-flex items-center gap-1 text-xs text-primary hover:underline" href={event.html_link} rel="noreferrer" target="_blank">Abrir en Google <ExternalLink className="size-3" /></a> : null}
                </div>
                <select
                  aria-label="Paciente para asociar"
                  className="h-10 rounded-xl border border-input bg-background px-3 text-sm"
                  onChange={(e) => setClientByEvent((current) => ({ ...current, [event.event_id]: e.target.value }))}
                  value={clientByEvent[event.event_id] ?? ""}
                >
                  <option value="">Seleccionar paciente</option>
                  {clients.map((client) => <option key={client.id} value={client.id}>{client.fullname}</option>)}
                </select>
                <Button
                  disabled={!clientByEvent[event.event_id] || importMutation.isPending}
                  onClick={() => importMutation.mutate({ eventId: event.event_id, clientId: clientByEvent[event.event_id] })}
                  type="button"
                >
                  Asociar y crear turno
                </Button>
              </div>
            ))}
          </div>
        ) : null}
        <p className="text-xs text-muted-foreground">
          Al enviar un turno desde SessionFlow, Google recibe únicamente “Sesión”, horario, ubicación y un identificador técnico privado; nunca notas clínicas.
        </p>
      </CardContent>
    </Card>
  );
}
