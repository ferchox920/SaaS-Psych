"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle, PencilLine, Plus, Trash2, UserRound } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Client } from "@/types/api";
import { ClientFormValues, clientSchema } from "@/features/clients/schemas/client-schema";

const emptyValues: ClientFormValues = {
  fullname: "",
  contact: "",
  notes_public: "",
};

export function ClientFormCard({
  activeClient,
  errorMessage,
  isDeleting,
  isSubmitting,
  onCancel,
  onDelete,
  onSubmit,
}: {
  activeClient: Client | null;
  errorMessage: string | null;
  isDeleting: boolean;
  isSubmitting: boolean;
  onCancel: () => void;
  onDelete: () => void;
  onSubmit: (values: ClientFormValues) => Promise<void>;
}) {
  const form = useForm<ClientFormValues>({
    resolver: zodResolver(clientSchema),
    defaultValues: emptyValues,
  });

  useEffect(() => {
    if (!activeClient) {
      form.reset(emptyValues);
      return;
    }

    form.reset({
      fullname: activeClient.fullname,
      contact: activeClient.contact,
      notes_public: activeClient.notes_public,
    });
  }, [activeClient, form]);

  const mode = activeClient ? "edit" : "create";

  return (
    <Card className="sticky top-4">
      <CardHeader>
        <div className="flex items-center gap-3">
          <div className="rounded-2xl bg-secondary p-3 text-secondary-foreground">
            {mode === "edit" ? <PencilLine className="size-5" /> : <Plus className="size-5" />}
          </div>
          <div>
            <CardTitle>{mode === "edit" ? "Editar cliente" : "Nuevo cliente"}</CardTitle>
            <CardDescription>
              {mode === "edit"
                ? "Actualiza el cliente seleccionado sin salir del listado."
                : "Alta rapida para avanzar el flujo profesional/admin."}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)}>
          <div className="space-y-2">
            <Label htmlFor="fullname">Nombre completo</Label>
            <Input
              id="fullname"
              placeholder="Nombre y apellido"
              {...form.register("fullname")}
            />
            <FieldError message={form.formState.errors.fullname?.message} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="contact">Contacto</Label>
            <Input
              id="contact"
              placeholder="Email, telefono o referencia"
              {...form.register("contact")}
            />
            <FieldError message={form.formState.errors.contact?.message} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="notes_public">Notas publicas</Label>
            <Textarea
              id="notes_public"
              placeholder="Resumen visible para operacion interna"
              {...form.register("notes_public")}
            />
            <FieldError message={form.formState.errors.notes_public?.message} />
          </div>

          {errorMessage ? (
            <div className="rounded-2xl border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {errorMessage}
            </div>
          ) : null}

          <div className="flex flex-wrap gap-3">
            <Button disabled={isSubmitting || isDeleting} type="submit">
              {isSubmitting ? <LoaderCircle className="size-4 animate-spin" /> : mode === "edit" ? <PencilLine className="size-4" /> : <Plus className="size-4" />}
              {mode === "edit" ? "Guardar cambios" : "Crear cliente"}
            </Button>

            <Button
              disabled={isSubmitting || isDeleting}
              onClick={() => {
                form.reset(emptyValues);
                onCancel();
              }}
              type="button"
              variant="outline"
            >
              <UserRound className="size-4" />
              {mode === "edit" ? "Nuevo cliente" : "Limpiar"}
            </Button>

            {mode === "edit" ? (
              <Button
                className="ml-auto"
                disabled={isSubmitting || isDeleting}
                onClick={onDelete}
                type="button"
                variant="destructive"
              >
                {isDeleting ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
				Archivar
              </Button>
            ) : null}
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
