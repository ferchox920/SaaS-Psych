"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { CalendarPlus2, LoaderCircle, MapPin, PencilLine, RotateCcw, XCircle } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { fromDateTimeLocalValue, toDateTimeLocalValue } from "@/features/appointments/lib/appointment-datetime";
import { AppointmentFormValues, appointmentSchema } from "@/features/appointments/schemas/appointment-schema";
import { Client, Appointment } from "@/types/api";

const createDefaultValues = (): AppointmentFormValues => {
  const now = new Date();
  const rounded = new Date(now);
  rounded.setMinutes(Math.ceil(now.getMinutes() / 30) * 30, 0, 0);
  const ends = new Date(rounded.getTime() + 60 * 60 * 1000);

  return {
    client_id: "",
    starts_at: toDateTimeLocalValue(rounded.toISOString()),
    ends_at: toDateTimeLocalValue(ends.toISOString()),
    location: "",
  };
};

export function AppointmentFormCard({
  activeAppointment,
  clients,
  errorMessage,
  isCanceling,
  isSubmitting,
  onCancelEdit,
  onCancelAppointment,
  onSubmit,
}: {
  activeAppointment: Appointment | null;
  clients: Client[];
  errorMessage: string | null;
  isCanceling: boolean;
  isSubmitting: boolean;
  onCancelEdit: () => void;
  onCancelAppointment: () => void;
  onSubmit: (values: AppointmentFormValues) => Promise<void>;
}) {
  const form = useForm<AppointmentFormValues>({
    resolver: zodResolver(appointmentSchema),
    defaultValues: createDefaultValues(),
  });

  useEffect(() => {
    if (!activeAppointment) {
      form.reset(createDefaultValues());
      return;
    }

    form.reset({
      client_id: activeAppointment.client_id,
      starts_at: toDateTimeLocalValue(activeAppointment.starts_at),
      ends_at: toDateTimeLocalValue(activeAppointment.ends_at),
      location: activeAppointment.location,
    });
  }, [activeAppointment, form]);

  const isCanceled = activeAppointment?.status === "canceled";
  const mode = activeAppointment ? "edit" : "create";

  return (
    <Card className="sticky top-4">
      <CardHeader>
        <div className="flex items-center gap-3">
          <div className="rounded-2xl bg-secondary p-3 text-secondary-foreground">
            {mode === "edit" ? <PencilLine className="size-5" /> : <CalendarPlus2 className="size-5" />}
          </div>
          <div>
            <CardTitle>{mode === "edit" ? "Editar cita" : "Nueva cita"}</CardTitle>
            <CardDescription>
              {mode === "edit"
                ? "Ajusta el horario o la ubicación de la cita."
                : "Crea una cita para un paciente existente."}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <form
          className="space-y-4"
          onSubmit={form.handleSubmit(async (values) => {
            await onSubmit({
              ...values,
              starts_at: fromDateTimeLocalValue(values.starts_at),
              ends_at: fromDateTimeLocalValue(values.ends_at),
            });
          })}
        >
          <div className="space-y-2">
            <Label htmlFor="client_id">Cliente</Label>
            <select
              id="client_id"
              className="flex h-11 w-full rounded-2xl border border-input bg-white px-4 py-2 text-sm shadow-sm outline-none transition focus-visible:ring-4 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
              disabled={mode === "edit"}
              {...form.register("client_id")}
            >
              <option value="">Selecciona un cliente</option>
              {clients.map((client) => (
                <option key={client.id} value={client.id}>
                  {client.fullname}
                </option>
              ))}
            </select>
            <FieldError message={form.formState.errors.client_id?.message} />
          </div>

          <div className="grid gap-4">
            <div className="space-y-2">
              <Label htmlFor="starts_at">Inicio</Label>
              <Input id="starts_at" type="datetime-local" {...form.register("starts_at")} />
              <FieldError message={form.formState.errors.starts_at?.message} />
            </div>

            <div className="space-y-2">
              <Label htmlFor="ends_at">Fin</Label>
              <Input id="ends_at" type="datetime-local" {...form.register("ends_at")} />
              <FieldError message={form.formState.errors.ends_at?.message} />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="location">Ubicacion</Label>
            <Input id="location" placeholder="Consultorio, virtual o referencia" {...form.register("location")} />
            <FieldError message={form.formState.errors.location?.message} />
          </div>

          {errorMessage ? (
            <div className="rounded-2xl border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {errorMessage}
            </div>
          ) : null}

          {isCanceled ? (
            <div className="rounded-2xl border border-border/70 bg-muted/40 px-4 py-3 text-sm text-muted-foreground">
              Esta cita ya fue cancelada y no puede editarse.
            </div>
          ) : null}

          <div className="flex flex-wrap gap-3">
            <Button disabled={isSubmitting || isCanceling || isCanceled} type="submit">
              {isSubmitting ? <LoaderCircle className="size-4 animate-spin" /> : mode === "edit" ? <PencilLine className="size-4" /> : <CalendarPlus2 className="size-4" />}
              {mode === "edit" ? "Guardar cambios" : "Crear cita"}
            </Button>

            <Button
              disabled={isSubmitting || isCanceling}
              onClick={() => {
                form.reset(createDefaultValues());
                onCancelEdit();
              }}
              type="button"
              variant="outline"
            >
              <RotateCcw className="size-4" />
              {mode === "edit" ? "Nueva cita" : "Limpiar"}
            </Button>

            {mode === "edit" ? (
              <Button
                className="ml-auto"
                disabled={isSubmitting || isCanceling || isCanceled}
                onClick={onCancelAppointment}
                type="button"
                variant="destructive"
              >
                {isCanceling ? <LoaderCircle className="size-4 animate-spin" /> : <XCircle className="size-4" />}
                Cancelar cita
              </Button>
            ) : null}
          </div>

          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <MapPin className="size-4" />
            Las citas canceladas permanecen en el historial.
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

function FieldError({ message }: { message?: string }) {
  if (!message) {
    return null;
  }

  return <p className="text-sm text-destructive">{message}</p>;
}
