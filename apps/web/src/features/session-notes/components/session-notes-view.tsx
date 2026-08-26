"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { toast } from "sonner";

import { EmptyState } from "@/components/shared/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { listAppointments } from "@/features/appointments/api/appointments-api";
import { useSession } from "@/features/auth/hooks/use-session";
import { listClients } from "@/features/clients/api/clients-api";
import {
  createSessionNote,
  listSessionNotes,
  updateSessionNote,
} from "@/features/session-notes/api/session-notes-api";
import { SessionNoteFormCard } from "@/features/session-notes/components/session-note-form-card";
import { getSessionNoteErrorMessage } from "@/features/session-notes/lib/session-note-error-messages";
import { SessionNoteFormValues } from "@/features/session-notes/schemas/session-note-schema";

type SessionNotesViewProps = {
  initialAppointmentId?: string;
};

function getDefaultRange() {
  const now = new Date();
  const from = new Date(now);
  from.setDate(now.getDate() - 14);
  from.setHours(0, 0, 0, 0);
  const to = new Date(now);
  to.setDate(now.getDate() + 14);
  to.setHours(23, 59, 0, 0);

  return {
    from: from.toISOString(),
    to: to.toISOString(),
  };
}

export function SessionNotesView({ initialAppointmentId }: SessionNotesViewProps) {
  const queryClient = useQueryClient();
  const router = useRouter();
  const { authenticatedRequest, session } = useSession();
  const [range] = useState(getDefaultRange);
  const [selectedAppointmentId, setSelectedAppointmentId] = useState<string | null>(initialAppointmentId ?? null);
  const [activeNoteId, setActiveNoteId] = useState<string | null>(null);

  const appointmentsQuery = useQuery({
    queryKey: ["appointments", "session-notes-source", range],
    queryFn: () => authenticatedRequest((currentSession) => listAppointments(currentSession, range)),
  });

  const clientsQuery = useQuery({
    queryKey: ["clients", "session-notes-options"],
    queryFn: () => authenticatedRequest((currentSession) => listClients(currentSession)),
  });

  const appointments = useMemo(() => appointmentsQuery.data?.items ?? [], [appointmentsQuery.data?.items]);
  const clients = useMemo(() => clientsQuery.data?.items ?? [], [clientsQuery.data?.items]);
  const clientsById = useMemo(
    () => new Map(clients.map((client) => [client.id, client])),
    [clients],
  );
  const resolvedSelectedAppointmentId =
    selectedAppointmentId && appointments.some((appointment) => appointment.id === selectedAppointmentId)
      ? selectedAppointmentId
      : appointments[0]?.id ?? null;
  const selectedAppointment =
    appointments.find((appointment) => appointment.id === resolvedSelectedAppointmentId) ?? null;

  const notesQuery = useQuery({
    queryKey: ["session-notes", resolvedSelectedAppointmentId],
    enabled: Boolean(resolvedSelectedAppointmentId),
    queryFn: () =>
      authenticatedRequest((currentSession) =>
        listSessionNotes(currentSession, resolvedSelectedAppointmentId!),
      ),
  });

  const notes = useMemo(() => notesQuery.data?.items ?? [], [notesQuery.data?.items]);
  const activeNote = notes.find((note) => note.id === activeNoteId) ?? null;
  const canEditActiveNote =
    !activeNote || activeNote.author_user_id === session?.userId || ["owner", "admin"].includes(session?.role ?? "");

  const refreshNotes = async (nextNoteId?: string | null) => {
    await queryClient.invalidateQueries({
      queryKey: ["session-notes", resolvedSelectedAppointmentId],
    });

    if (typeof nextNoteId !== "undefined") {
      setActiveNoteId(nextNoteId);
    }
  };

  const createMutation = useMutation({
    mutationFn: (values: SessionNoteFormValues) =>
      authenticatedRequest((currentSession) =>
        createSessionNote(currentSession, resolvedSelectedAppointmentId!, {
          body: values.body,
          is_private: values.is_private,
        }),
      ),
    onSuccess: async (note) => {
      toast.success("Nota creada.");
      await refreshNotes(note.id);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ noteId, values }: { noteId: string; values: SessionNoteFormValues }) =>
      authenticatedRequest((currentSession) =>
        updateSessionNote(currentSession, noteId, {
          body: values.body,
          is_private: values.is_private,
        }),
      ),
    onSuccess: async (note) => {
      toast.success("Nota actualizada.");
      await refreshNotes(note.id);
    },
  });

  const mutationError = createMutation.error || updateMutation.error;
  const mutationErrorMessage = mutationError ? getSessionNoteErrorMessage(mutationError) : null;

  const handleSubmit = async (values: SessionNoteFormValues) => {
    if (!resolvedSelectedAppointmentId) {
      return;
    }

    if (activeNote) {
      await updateMutation.mutateAsync({
        noteId: activeNote.id,
        values,
      });
      return;
    }

    await createMutation.mutateAsync(values);
  };

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <Badge variant="outline">Session notes</Badge>
        <h2 className="text-3xl font-semibold">Notas de sesion</h2>
        <p className="text-muted-foreground">
          Flujo alineado con el backend: primero eliges una cita y luego trabajas sus notas, respetando privacidad y permisos por autor/rol.
        </p>
      </header>

      <section className="grid gap-6 xl:grid-cols-[1.4fr_0.95fr]">
        <div className="space-y-4">
          <Card>
            <CardHeader className="gap-4">
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div>
                  <CardTitle>Seleccion de cita</CardTitle>
                  <CardDescription>
                    El endpoint de notas depende de `appointment_id`, por eso la cita seleccionada gobierna todo el modulo.
                  </CardDescription>
                </div>
                {resolvedSelectedAppointmentId ? (
                  <Button
                    onClick={() => router.push(`/appointments`)}
                    type="button"
                    variant="outline"
                  >
                    Volver a agenda
                  </Button>
                ) : null}
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              {appointmentsQuery.isLoading ? (
                <div className="grid gap-3">
                  {Array.from({ length: 3 }).map((_, index) => (
                    <div
                      key={index}
                      className="h-24 animate-pulse rounded-[28px] border border-border/60 bg-muted/40"
                    />
                  ))}
                </div>
              ) : null}

              {appointmentsQuery.isError ? (
                <EmptyState
                  title="No se pudo cargar la agenda fuente"
                  description={getSessionNoteErrorMessage(appointmentsQuery.error)}
                />
              ) : null}

              {!appointmentsQuery.isLoading && !appointmentsQuery.isError && appointments.length > 0 ? (
                <div className="grid gap-4">
                  {appointments.map((appointment) => {
                    const client = clientsById.get(appointment.client_id);
                    const selected = appointment.id === selectedAppointmentId;

                    return (
                      <button
                        key={appointment.id}
                        className="text-left"
                        onClick={() => {
                          setSelectedAppointmentId(appointment.id);
                          setActiveNoteId(null);
                          void queryClient.invalidateQueries({
                            queryKey: ["session-notes", appointment.id],
                          });
                        }}
                        type="button"
                      >
                        <Card
                          className={
                            selected
                              ? "border-primary shadow-[0_18px_45px_-30px_rgba(59,90,158,0.5)]"
                              : "transition hover:-translate-y-0.5 hover:border-primary/40"
                          }
                        >
                          <CardHeader className="gap-3 md:flex-row md:items-start md:justify-between">
                            <div>
                              <CardTitle>{client?.fullname ?? "Cliente sin resolver"}</CardTitle>
                              <CardDescription>{appointment.location || "Sin ubicacion definida"}</CardDescription>
                            </div>
                            <Badge variant={appointment.status === "canceled" ? "outline" : "secondary"}>
                              {appointment.status}
                            </Badge>
                          </CardHeader>
                          <CardContent className="grid gap-2 text-sm text-muted-foreground md:grid-cols-2">
                            <span>Inicio: {new Date(appointment.starts_at).toLocaleString()}</span>
                            <span>Fin: {new Date(appointment.ends_at).toLocaleString()}</span>
                          </CardContent>
                        </Card>
                      </button>
                    );
                  })}
                </div>
              ) : null}

              {!appointmentsQuery.isLoading && !appointmentsQuery.isError && appointments.length === 0 ? (
                <EmptyState
                  title="Todavia no hay citas para inspeccionar"
                  description="Primero crea una cita desde agenda. Luego podras registrar y editar notas clinicas desde este modulo."
                />
              ) : null}
            </CardContent>
          </Card>

          {selectedAppointment ? (
            <Card>
              <CardHeader>
                <CardTitle>Notas de la cita seleccionada</CardTitle>
                <CardDescription>
                  Cliente {clientsById.get(selectedAppointment.client_id)?.fullname ?? selectedAppointment.client_id}
                </CardDescription>
              </CardHeader>
              <CardContent className="grid gap-3">
                {notesQuery.isLoading ? (
                  <div className="grid gap-3">
                    {Array.from({ length: 2 }).map((_, index) => (
                      <div
                        key={index}
                        className="h-28 animate-pulse rounded-[24px] border border-border/60 bg-muted/40"
                      />
                    ))}
                  </div>
                ) : null}

                {notesQuery.isError ? (
                  <EmptyState
                    title="No se pudieron cargar las notas"
                    description={getSessionNoteErrorMessage(notesQuery.error)}
                  />
                ) : null}

                {!notesQuery.isLoading && !notesQuery.isError && notes.length > 0 ? (
                  notes.map((note) => {
                    const isOwner = note.author_user_id === session?.userId;
                    const canEdit = isOwner || ["owner", "admin"].includes(session?.role ?? "");
                    const selected = note.id === activeNoteId;

                    return (
                      <button
                        key={note.id}
                        className="text-left"
                        onClick={() => setActiveNoteId(note.id)}
                        type="button"
                      >
                        <div
                          className={`rounded-[24px] border p-4 ${
                            selected
                              ? "border-primary bg-secondary/20 shadow-[0_18px_45px_-30px_rgba(59,90,158,0.5)]"
                              : "border-border/70 bg-muted/30"
                          }`}
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <Badge variant={note.is_private ? "secondary" : "outline"}>
                              {note.is_private ? "private" : "shared"}
                            </Badge>
                            {isOwner ? <Badge>autor</Badge> : null}
                            {!canEdit ? <Badge variant="outline">solo lectura</Badge> : null}
                            <span className="text-xs text-muted-foreground">
                              {new Date(note.updated_at).toLocaleString()}
                            </span>
                          </div>
                          <p className="mt-3 whitespace-pre-wrap text-sm text-foreground">{note.body}</p>
                        </div>
                      </button>
                    );
                  })
                ) : null}

                {!notesQuery.isLoading && !notesQuery.isError && notes.length === 0 ? (
                  <EmptyState
                    title="Sin notas para esta cita"
                    description="Puedes crear la primera nota desde el panel lateral."
                  />
                ) : null}
              </CardContent>
            </Card>
          ) : null}
        </div>

        <SessionNoteFormCard
          activeNote={activeNote}
          canEditActiveNote={canEditActiveNote}
          errorMessage={mutationErrorMessage}
          hasSelectedAppointment={Boolean(resolvedSelectedAppointmentId)}
          isSubmitting={createMutation.isPending || updateMutation.isPending}
          onCancelEdit={() => {
            createMutation.reset();
            updateMutation.reset();
            setActiveNoteId(null);
          }}
          onSubmit={handleSubmit}
        />
      </section>
    </div>
  );
}
