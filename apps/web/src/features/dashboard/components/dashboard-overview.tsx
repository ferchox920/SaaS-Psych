"use client";

import { useQuery } from "@tanstack/react-query";
import { CalendarRange, FileText, Shield, Users } from "lucide-react";

import { EmptyState } from "@/components/shared/empty-state";
import { MetricCard } from "@/components/shared/metric-card";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { listAppointments } from "@/features/appointments/api/appointments-api";
import { useSession } from "@/features/auth/hooks/use-session";
import { listClients } from "@/features/clients/api/clients-api";

export function DashboardOverview() {
  const { session, authenticatedRequest } = useSession();

  const clientsQuery = useQuery({
    queryKey: ["clients", "summary"],
    queryFn: () => authenticatedRequest((currentSession) => listClients(currentSession)),
  });

  const appointmentsQuery = useQuery({
    queryKey: ["appointments", "summary"],
    queryFn: () =>
      authenticatedRequest((currentSession) =>
        listAppointments(currentSession, {
          from: new Date().toISOString(),
          to: new Date(Date.now() + 1000 * 60 * 60 * 24 * 14).toISOString(),
        }),
      ),
  });

  const metrics = [
    {
      title: "Clientes",
      value: String(clientsQuery.data?.items.length ?? 0),
      description: "Padron disponible para el profesional.",
      icon: <Users className="size-5" />,
    },
    {
      title: "Proximas sesiones",
      value: String(appointmentsQuery.data?.items.length ?? 0),
      description: "Ventana de agenda de los proximos 14 dias.",
      icon: <CalendarRange className="size-5" />,
    },
    {
      title: "Notas clinicas",
      value: "Preparado",
      description: "La capa de session notes ya tiene endpoint y carpeta de feature.",
      icon: <FileText className="size-5" />,
    },
    {
      title: "Audit",
      value: session?.role === "member" ? "Restringido" : "Disponible",
      description: "Visibilidad condicionada por rol owner/admin.",
      icon: <Shield className="size-5" />,
    },
  ];

  return (
    <div className="space-y-8">
      <section className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <Badge variant="outline" className="mb-3">
            Dashboard
          </Badge>
          <h2 className="text-3xl font-semibold tracking-tight">Base operativa del MVP</h2>
          <p className="mt-2 max-w-3xl text-muted-foreground">
            El dashboard inicial valida la integracion con `clients` y `appointments` reales del backend sin mezclar
            estado global innecesario.
          </p>
        </div>
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {metrics.map((metric) => (
          <MetricCard key={metric.title} {...metric} />
        ))}
      </section>

      <section className="grid gap-4 xl:grid-cols-[1.4fr_1fr]">
        <Card>
          <CardHeader>
            <CardTitle>Capas listas para crecer</CardTitle>
            <CardDescription>La arquitectura esta preparada para avanzar por feature sin reescribir la base.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 md:grid-cols-2">
            <ScopeCard title="Auth" description="Login por tenant + refresh centralizado en provider." />
            <ScopeCard title="Clients" description="Listados reales con TanStack Query y DTOs tipados." />
            <ScopeCard title="Appointments" description="Query por rango respetando el contrato actual del backend." />
            <ScopeCard title="Audit" description="Feature separada y visible solo para owner/admin." />
          </CardContent>
        </Card>

        <EmptyState
          title="Siguiente iteracion sugerida"
          description="Conectar mutations de create/update para clients y appointments, y luego llevar auth a cookies httpOnly si queres endurecer seguridad antes de crecer en SSR."
        />
      </section>
    </div>
  );
}

function ScopeCard({ title, description }: { title: string; description: string }) {
  return (
    <div className="rounded-[24px] border border-border/70 bg-muted/40 p-4">
      <h3 className="font-medium">{title}</h3>
      <p className="mt-2 text-sm text-muted-foreground">{description}</p>
    </div>
  );
}
