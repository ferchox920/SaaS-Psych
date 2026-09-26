"use client";

import { useDeferredValue, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Search, UserPlus } from "lucide-react";
import { toast } from "sonner";
import Link from "next/link";

import { EmptyState } from "@/components/shared/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useSession } from "@/features/auth/hooks/use-session";
import { archiveClient, createClient, listArchivedClients, listClients, restoreClient, updateClient } from "@/features/clients/api/clients-api";
import { ClientFormCard } from "@/features/clients/components/client-form-card";
import { ClinicalAssignmentsCard } from "@/features/clients/components/clinical-assignments-card";
import { getClientErrorMessage } from "@/features/clients/lib/client-error-messages";
import { ClientFormValues } from "@/features/clients/schemas/client-schema";
export function ClientsView() {
  const queryClient = useQueryClient();
  const { authenticatedRequest } = useSession();
  const [activeClientId, setActiveClientId] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);

  const clientsQuery = useQuery({
    queryKey: ["clients", "list"],
    queryFn: () => authenticatedRequest((session) => listClients(session)),
  });
	const archivedClientsQuery = useQuery({
		queryKey: ["clients", "archived"],
		queryFn: () => authenticatedRequest((session) => listArchivedClients(session)),
	});

  const clients = useMemo(() => clientsQuery.data?.items ?? [], [clientsQuery.data?.items]);
  const activeClient = clients.find((client) => client.id === activeClientId) ?? null;

  const filteredClients = useMemo(() => {
    const term = deferredSearch.trim().toLowerCase();

    if (!term) {
      return clients;
    }

    return clients.filter((client) => {
      return [client.fullname, client.contact, client.notes_public].some((value) =>
        value.toLowerCase().includes(term),
      );
    });
  }, [clients, deferredSearch]);

  const refreshClients = async (nextClientId?: string | null) => {
    await queryClient.invalidateQueries({
      queryKey: ["clients", "list"],
    });

    if (typeof nextClientId !== "undefined") {
      setActiveClientId(nextClientId);
    }
  };

  const createMutation = useMutation({
    mutationFn: (values: ClientFormValues) =>
      authenticatedRequest((session) =>
        createClient(session, {
          fullname: values.fullname,
          contact: values.contact ?? "",
          notes_public: values.notes_public ?? "",
        }),
      ),
    onSuccess: async (client) => {
      toast.success("Cliente creado.");
      await refreshClients(client.id);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ clientId, values }: { clientId: string; values: ClientFormValues }) =>
      authenticatedRequest((session) =>
        updateClient(session, clientId, {
          fullname: values.fullname,
          contact: values.contact ?? "",
          notes_public: values.notes_public ?? "",
        }),
      ),
    onSuccess: async (client) => {
      toast.success("Cliente actualizado.");
      await refreshClients(client.id);
    },
  });

  const deleteMutation = useMutation({
	mutationFn: ({ clientId, reason }: { clientId: string; reason: string }) =>
		authenticatedRequest((session) => archiveClient(session, clientId, reason)),
    onSuccess: async () => {
	  toast.success("Cliente archivado sin eliminar su historia.");
	  await queryClient.invalidateQueries({ queryKey: ["clients", "archived"] });
      await refreshClients(null);
    },
  });

	const restoreMutation = useMutation({
		mutationFn: (clientId: string) => authenticatedRequest((session) => restoreClient(session, clientId)),
		onSuccess: async () => {
			toast.success("Cliente restaurado.");
			await queryClient.invalidateQueries({ queryKey: ["clients"] });
		},
	});

  const mutationError = createMutation.error || updateMutation.error || deleteMutation.error;
  const errorMessage = mutationError ? getClientErrorMessage(mutationError) : null;

  const handleSubmit = async (values: ClientFormValues) => {
    if (activeClient) {
      await updateMutation.mutateAsync({
        clientId: activeClient.id,
        values,
      });
      return;
    }

    await createMutation.mutateAsync(values);
  };

  const handleDelete = async () => {
    if (!activeClient) {
      return;
    }

	const reason = window.prompt("Motivo obligatorio del archivo clínico:")?.trim();
	if (!reason) {
		return;
	}
	await deleteMutation.mutateAsync({ clientId: activeClient.id, reason });
  };

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <Badge variant="outline">Pacientes</Badge>
        <h2 className="text-3xl font-semibold">Pacientes</h2>
        <p className="text-muted-foreground">
		  Los pacientes se muestran según asignación clínica. Archivar preserva citas, notas e identificadores.
        </p>
      </header>

      {activeClient ? <Link className="inline-flex rounded-xl border border-primary px-4 py-3 font-semibold text-primary" href={`/clients/${activeClient.id}/session`}>Abrir sesión clínica de {activeClient.fullname}</Link> : <p className="text-sm text-muted-foreground">Selecciona un paciente para abrir su sesión clínica.</p>}

      <section className="grid gap-6 xl:grid-cols-[1.35fr_0.95fr]">
        <div className="space-y-4">
          <Card>
            <CardHeader className="gap-4 md:flex-row md:items-center md:justify-between">
              <div>
                <CardTitle>Pacientes registrados</CardTitle>
                <CardDescription>
                  Selecciona un paciente para consultar o actualizar su ficha.
                </CardDescription>
              </div>
              <Button
                onClick={() => setActiveClientId(null)}
                type="button"
                variant="secondary"
              >
                <UserPlus className="size-4" />
                Nuevo paciente
              </Button>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="relative">
                <Search className="pointer-events-none absolute left-4 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="pl-10"
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="Buscar por nombre, contacto o notas"
                  value={search}
                />
              </div>

              {clientsQuery.isLoading ? (
                <div className="grid gap-3">
                  {Array.from({ length: 3 }).map((_, index) => (
                    <div
                      key={index}
                      className="h-28 animate-pulse rounded-[28px] border border-border/60 bg-muted/40"
                    />
                  ))}
                </div>
              ) : null}

              {clientsQuery.isError ? (
                <EmptyState
                  title="No se pudo cargar el listado"
                  description={getClientErrorMessage(clientsQuery.error)}
                />
              ) : null}

              {!clientsQuery.isLoading && !clientsQuery.isError && filteredClients.length > 0 ? (
                <div className="grid gap-4 lg:grid-cols-2">
                  {filteredClients.map((client) => {
                    const active = client.id === activeClientId;

                    return (
                      <button
                        key={client.id}
                        className={`text-left ${active ? "outline-none" : ""}`}
                        onClick={() => setActiveClientId(client.id)}
                        type="button"
                      >
                        <Card
                          className={
                            active
                              ? "border-primary shadow-[0_18px_45px_-30px_rgba(59,90,158,0.5)]"
                              : "transition hover:-translate-y-0.5 hover:border-primary/40"
                          }
                        >
                          <CardHeader>
                            <div className="flex items-start justify-between gap-3">
                              <div>
                                <CardTitle>{client.fullname}</CardTitle>
                                <CardDescription>
                                  {client.contact || "Sin contacto registrado"}
                                </CardDescription>
                              </div>
                              {active ? <Badge>editando</Badge> : <Badge variant="secondary">activo</Badge>}
                            </div>
                          </CardHeader>
                          <CardContent className="space-y-3 text-sm text-muted-foreground">
                            <p className="line-clamp-3">
                              {client.notes_public || "Sin notas publicas"}
                            </p>
                            <div className="flex flex-wrap gap-2">
                              <Badge variant="outline">
                                Actualizado {new Date(client.updated_at).toLocaleString()}
                              </Badge>
                            </div>
                          </CardContent>
                        </Card>
                      </button>
                    );
                  })}
                </div>
              ) : null}

              {!clientsQuery.isLoading && !clientsQuery.isError && filteredClients.length === 0 ? (
                <EmptyState
                  title={clients.length ? "Sin resultados para esa busqueda" : "Todavia no hay clientes"}
                  description={
                    clients.length
                      ? "Ajusta el filtro o crea un nuevo cliente."
					  : "Puedes cargar el primer paciente desde el panel derecho; el creador queda asignado como tratante."
                  }
                />
              ) : null}
            </CardContent>
          </Card>
        </div>

        <ClientFormCard
          activeClient={activeClient}
          errorMessage={errorMessage}
          isDeleting={deleteMutation.isPending}
          isSubmitting={createMutation.isPending || updateMutation.isPending}
          onCancel={() => {
            createMutation.reset();
            updateMutation.reset();
            deleteMutation.reset();
            setActiveClientId(null);
          }}
          onDelete={handleDelete}
          onSubmit={handleSubmit}
        />
      </section>

      {activeClient ? <ClinicalAssignmentsCard key={activeClient.id} clientId={activeClient.id} /> : null}

	  <Card>
		<CardHeader>
		  <CardTitle>Historia archivada</CardTitle>
		  <CardDescription>El archivo no borra citas ni notas. Solo aparecen pacientes para los que conservas autorización clínica.</CardDescription>
		</CardHeader>
		<CardContent className="space-y-3">
		  {(archivedClientsQuery.data?.items ?? []).map((client) => (
			<div className="flex items-center justify-between gap-4 rounded-2xl border p-4" key={client.id}>
			  <div>
				<p className="font-medium">{client.fullname}</p>
				<p className="text-sm text-muted-foreground">Motivo: {client.archive_reason || "No informado"}</p>
			  </div>
			  <Button disabled={restoreMutation.isPending} onClick={() => restoreMutation.mutate(client.id)} type="button" variant="outline">
				Restaurar
			  </Button>
			</div>
		  ))}
		  {!archivedClientsQuery.isLoading && !(archivedClientsQuery.data?.items.length) ? (
			<p className="text-sm text-muted-foreground">No hay pacientes archivados autorizados.</p>
		  ) : null}
		</CardContent>
	  </Card>
    </div>
  );
}
