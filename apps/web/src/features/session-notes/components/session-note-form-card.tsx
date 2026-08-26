"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { FilePlus2, LoaderCircle, Lock, PencilLine, RotateCcw } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { SessionNoteFormValues, sessionNoteSchema } from "@/features/session-notes/schemas/session-note-schema";
import { SessionNote } from "@/types/api";

const emptyValues: SessionNoteFormValues = {
  body: "",
  is_private: true,
};

export function SessionNoteFormCard({
  activeNote,
  canEditActiveNote,
  errorMessage,
  hasSelectedAppointment,
  isSubmitting,
  onCancelEdit,
  onSubmit,
}: {
  activeNote: SessionNote | null;
  canEditActiveNote: boolean;
  errorMessage: string | null;
  hasSelectedAppointment: boolean;
  isSubmitting: boolean;
  onCancelEdit: () => void;
  onSubmit: (values: SessionNoteFormValues) => Promise<void>;
}) {
  const form = useForm<SessionNoteFormValues>({
    resolver: zodResolver(sessionNoteSchema),
    defaultValues: emptyValues,
  });

  useEffect(() => {
    if (!activeNote) {
      form.reset(emptyValues);
      return;
    }

    form.reset({
      body: activeNote.body,
      is_private: activeNote.is_private,
    });
  }, [activeNote, form]);

  const mode = activeNote ? "edit" : "create";

  return (
    <Card className="sticky top-4">
      <CardHeader>
        <div className="flex items-center gap-3">
          <div className="rounded-2xl bg-secondary p-3 text-secondary-foreground">
            {mode === "edit" ? <PencilLine className="size-5" /> : <FilePlus2 className="size-5" />}
          </div>
          <div>
            <CardTitle>{mode === "edit" ? "Editar nota" : "Nueva nota"}</CardTitle>
            <CardDescription>
              {mode === "edit"
                ? "Solo el autor o un owner/admin pueden modificar una nota existente."
                : "La nota se crea para la cita seleccionada en el panel izquierdo."}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)}>
          <div className="space-y-2">
            <Label htmlFor="body">Contenido</Label>
            <Textarea
              id="body"
              className="min-h-44"
              disabled={!hasSelectedAppointment || (mode === "edit" && !canEditActiveNote)}
              placeholder="Observaciones, progreso, acuerdos o recordatorios clinicos"
              {...form.register("body")}
            />
            <FieldError message={form.formState.errors.body?.message} />
          </div>

          <label className="flex items-center gap-3 rounded-2xl border border-border/70 bg-muted/30 px-4 py-3 text-sm text-foreground">
            <input
              className="size-4"
              disabled={!hasSelectedAppointment || (mode === "edit" && !canEditActiveNote)}
              type="checkbox"
              {...form.register("is_private")}
            />
            <span className="inline-flex items-center gap-2">
              <Lock className="size-4" />
              Marcar como nota privada
            </span>
          </label>

          {errorMessage ? (
            <div className="rounded-2xl border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {errorMessage}
            </div>
          ) : null}

          {!hasSelectedAppointment ? (
            <div className="rounded-2xl border border-border/70 bg-muted/30 px-4 py-3 text-sm text-muted-foreground">
              Selecciona una cita para cargar o editar notas.
            </div>
          ) : null}

          {mode === "edit" && !canEditActiveNote ? (
            <div className="rounded-2xl border border-border/70 bg-muted/30 px-4 py-3 text-sm text-muted-foreground">
              Esta nota es visible para ti, pero solo el autor o un owner/admin pueden editarla.
            </div>
          ) : null}

          <div className="flex flex-wrap gap-3">
            <Button
              disabled={!hasSelectedAppointment || isSubmitting || (mode === "edit" && !canEditActiveNote)}
              type="submit"
            >
              {isSubmitting ? <LoaderCircle className="size-4 animate-spin" /> : mode === "edit" ? <PencilLine className="size-4" /> : <FilePlus2 className="size-4" />}
              {mode === "edit" ? "Guardar nota" : "Crear nota"}
            </Button>

            <Button
              disabled={isSubmitting}
              onClick={() => {
                form.reset(emptyValues);
                onCancelEdit();
              }}
              type="button"
              variant="outline"
            >
              <RotateCcw className="size-4" />
              {mode === "edit" ? "Nueva nota" : "Limpiar"}
            </Button>
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
