"use client";

import { useDeferredValue, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CalendarClock, Search } from "lucide-react";
import Link from "next/link";
import { toast } from "sonner";

import { EmptyState } from "@/components/shared/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  cancelAppointment,
  createAppointment,
  listAppointments,
  updateAppointment,
} from "@/features/appointments/api/appointments-api";
import { AppointmentFormCard } from "@/features/appointments/components/appointment-form-card";
import { GoogleCalendarCard } from "@/features/appointments/components/google-calendar-card";
import { getAppointmentErrorMessage } from "@/features/appointments/lib/appointment-error-messages";
import { AppointmentFormValues } from "@/features/appointments/schemas/appointment-schema";
import { useSession } from "@/features/auth/hooks/use-session";
import { listClients } from "@/features/clients/api/clients-api";

function getDefaultRange() {
  const now = new Date();
  const from = new Date(now);
  from.setHours(0, 0, 0, 0);
  const to = new Date(from);
  to.setDate(to.getDate() + 14);
  to.setHours(23, 59, 0, 0);

  return {
    from: from.toISOString(),
    to: to.toISOString(),
  };
}

export function AppointmentsView() {
  const queryClient = useQueryClient();
  const { authenticatedRequest } = useSession();
  const [range, setRange] = useState(getDefaultRange);
  const [activeAppointmentId, setActiveAppointmentId] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);

  const clientsQuery = useQuery({
    queryKey: ["clients", "options"],
    queryFn: () => authenticatedRequest((session) => listClients(session)),
  });

  const appointmentsQuery = useQuery({
    queryKey: ["appointments", "list", range],
    queryFn: () => authenticatedRequest((session) => listAppointments(session, range)),
  });

  const clients = useMemo(() => clientsQuery.data?.items ?? [], [clientsQuery.data?.items]);
  const clientsById = useMemo(
    () =>
      new Map(
        clients.map((client) => [client.id, client]),
      ),
    [clients],
  );
  const appointments = useMemo(() => appointmentsQuery.data?.items ?? [], [appointmentsQuery.data?.items]);
  const activeAppointment = appointments.find((appointment) => appointment.id === activeAppointmentId) ?? null;

  const filteredAppointments = useMemo(() => {
    const term = deferredSearch.trim().toLowerCase();

    if (!term) {
      return appointments;
    }

    return appointments.filter((appointment) => {
      const clientName = clientsById.get(appointment.client_id)?.fullname ?? "";

      return [appointment.location, appointment.status, clientName]
        .join(" ")
        .toLowerCase()
        .includes(term);
    });
  }, [appointments, clientsById, deferredSearch]);

  const refreshAppointments = async (nextAppointmentId?: string | null) => {
    await queryClient.invalidateQueries({
      queryKey: ["appointments", "list"],
    });

    if (typeof nextAppointmentId !== "undefined") {
      setActiveAppointmentId(nextAppointmentId);
    }
  };

  const createMutation = useMutation({
    mutationFn: (values: AppointmentFormValues) =>
      authenticatedRequest((session) =>
        createAppointment(session, {
          client_id: values.client_id,
          starts_at: values.starts_at,
          ends_at: values.ends_at,
          location: values.location ?? "",
        }),
      ),
    onSuccess: async (appointment) => {
      toast.success("Cita creada.");
      await refreshAppointments(appointment.id);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ appointmentId, values }: { appointmentId: string; values: AppointmentFormValues }) =>
      authenticatedRequest((session) =>
        updateAppointment(session, appointmentId, {
          starts_at: values.starts_at,
          ends_at: values.ends_at,
          location: values.location ?? "",
        }),
      ),
    onSuccess: async (appointment) => {
      toast.success("Cita actualizada.");
      await refreshAppointments(appointment.id);
    },
  });

  const cancelMutation = useMutation({
    mutationFn: (appointmentId: string) =>
      authenticatedRequest((session) => cancelAppointment(session, appointmentId)),
    onSuccess: async (appointment) => {
      toast.success("Cita cancelada.");
      await refreshAppointments(appointment.id);
    },
  });

  const mutationError = createMutation.error || updateMutation.error || cancelMutation.error;
  const mutationErrorMessage = mutationError ? getAppointmentErrorMessage(mutationError) : null;

  const handleSubmit = async (values: AppointmentFormValues) => {
    if (activeAppointment) {
      await updateMutation.mutateAsync({
        appointmentId: activeAppointment.id,
        values,
      });
      return;
    }

    await createMutation.mutateAsync(values);
  };

  const handleCancelAppointment = async () => {
    if (!activeAppointment) {
      return;
    }

    await cancelMutation.mutateAsync(activeAppointment.id);
  };

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <Badge variant="outline">Appointments</Badge>
        <h2 className="text-3xl font-semibold">Agenda</h2>
        <p className="text-muted-foreground">
          Agenda end-to-end del MVP: filtro por rango, alta, edicion y cancelacion sobre el contrato actual del backend.
        </p>
      </header>

      <GoogleCalendarCard
        activeAppointmentId={activeAppointmentId}
        clients={clients}
        onImported={refreshAppointments}
        range={range}
      />

      <section className="grid gap-6 xl:grid-cols-[1.4fr_0.95fr]">
        <div className="space-y-4">
          <Card>
            <CardHeader className="gap-4">
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div>
                  <CardTitle>Turnos del rango activo</CardTitle>
                  <CardDescription>
                    El backend exige `from` y `to` en RFC3339 para listar. Esta vista trabaja sobre esa restriccion.
                  </CardDescription>
                </div>
                <Button onClick={() => setActiveAppointmentId(null)} type="button" variant="secondary">
                  <CalendarClock className="size-4" />
                  Nueva cita
                </Button>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 lg:grid-cols-[1fr_1fr_1.2fr]">
                <Input
                  type="datetime-local"
                  value={toLocalInput(range.from)}
                  onChange={(event) =>
                    setRange((current) => ({
                      ...current,
                      from: new Date(event.target.value).toISOString(),
                    }))
                  }
                />
                <Input
                  type="datetime-local"
                  value={toLocalInput(range.to)}
                  onChange={(event) =>
                    setRange((current) => ({
                      ...current,
                      to: new Date(event.target.value).toISOString(),
                    }))
                  }
                />
                <div className="relative">
                  <Search className="pointer-events-none absolute left-4 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    className="pl-10"
                    onChange={(event) => setSearch(event.target.value)}
                    placeholder="Buscar por cliente, estado o ubicacion"
                    value={search}
                  />
                </div>
              </div>

              {appointmentsQuery.isLoading ? (
                <div className="grid gap-3">
                  {Array.from({ length: 3 }).map((_, index) => (
                    <div
                      key={index}
                      className="h-28 animate-pulse rounded-[28px] border border-border/60 bg-muted/40"
                    />
                  ))}
                </div>
              ) : null}

              {appointmentsQuery.isError ? (
                <EmptyState
                  title="No se pudo cargar la agenda"
                  description={getAppointmentErrorMessage(appointmentsQuery.error)}
                />
              ) : null}

              {!appointmentsQuery.isLoading && !appointmentsQuery.isError && filteredAppointments.length > 0 ? (
                <div className="grid gap-4">
                  {filteredAppointments.map((appointment) => {
                    const active = appointment.id === activeAppointmentId;
                    const client = clientsById.get(appointment.client_id);
                    const isCanceled = appointment.status === "canceled";

                    return (
                      <button
                        key={appointment.id}
                        className={`text-left ${active ? "outline-none" : ""}`}
                        onClick={() => setActiveAppointmentId(appointment.id)}
                        type="button"
                      >
                        <Card
                          className={
                            active
                              ? "border-primary shadow-[0_18px_45px_-30px_rgba(59,90,158,0.5)]"
                              : "transition hover:-translate-y-0.5 hover:border-primary/40"
                          }
                        >
                          <CardHeader className="gap-3 md:flex-row md:items-start md:justify-between">
                            <div>
                              <CardTitle>{client?.fullname ?? "Cliente sin resolver"}</CardTitle>
                              <CardDescription>{appointment.location || "Sin ubicacion definida"}</CardDescription>
                            </div>
                            <div className="flex flex-wrap gap-2">
                              <Badge variant={isCanceled ? "outline" : "secondary"}>{appointment.status}</Badge>
                              {active ? <Badge>editando</Badge> : null}
                            </div>
                          </CardHeader>
                          <CardContent className="grid gap-2 text-sm text-muted-foreground md:grid-cols-2">
                            <span>Inicio: {new Date(appointment.starts_at).toLocaleString()}</span>
                            <span>Fin: {new Date(appointment.ends_at).toLocaleString()}</span>
                            <span className="md:col-span-2">
                              <Link
                                className="text-sm font-medium text-primary hover:underline"
                                href={`/session-notes?appointmentId=${appointment.id}`}
                                onClick={(event) => event.stopPropagation()}
                              >
                                Abrir notas de esta cita
                              </Link>
                            </span>
                          </CardContent>
                        </Card>
                      </button>
                    );
                  })}
                </div>
              ) : null}

              {!appointmentsQuery.isLoading && !appointmentsQuery.isError && filteredAppointments.length === 0 ? (
                <EmptyState
                  title={appointments.length ? "Sin resultados para ese filtro" : "Sin citas en este rango"}
                  description={
                    appointments.length
                      ? "Ajusta el rango o el texto de busqueda."
                      : "Puedes crear la primera cita desde el panel lateral usando un cliente existente."
                  }
                />
              ) : null}
            </CardContent>
          </Card>
        </div>

        <AppointmentFormCard
          activeAppointment={activeAppointment}
          clients={clients}
          errorMessage={mutationErrorMessage}
          isCanceling={cancelMutation.isPending}
          isSubmitting={createMutation.isPending || updateMutation.isPending}
          onCancelAppointment={handleCancelAppointment}
          onCancelEdit={() => {
            createMutation.reset();
            updateMutation.reset();
            cancelMutation.reset();
            setActiveAppointmentId(null);
          }}
          onSubmit={handleSubmit}
        />
      </section>
    </div>
  );
}

function toLocalInput(value: string) {
  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {
    return "";
  }

  const offset = date.getTimezoneOffset();
  return new Date(date.getTime() - offset * 60_000).toISOString().slice(0, 16);
}
