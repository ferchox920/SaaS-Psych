"use client";

import { useQuery } from "@tanstack/react-query";
import { CalendarRange, Shield, Users } from "lucide-react";
import Link from "next/link";

import { MetricCard } from "@/components/shared/metric-card";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { listAppointmentPage } from "@/features/appointments/api/appointments-api";
import { useSession } from "@/features/auth/hooks/use-session";
import { listClientPage } from "@/features/clients/api/clients-api";

function pageCount(page: { items: unknown[]; next_offset?: number | null } | undefined) {
  if (!page) return "0";
  return `${page.items.length}${page.next_offset == null ? "" : "+"}`;
}

export function DashboardOverview() {
  const { session, authenticatedRequest } = useSession();

  const clientsQuery = useQuery({
    queryKey: ["clients", "summary"],
    queryFn: () => authenticatedRequest((currentSession) => listClientPage(currentSession)),
  });

  const appointmentsQuery = useQuery({
    queryKey: ["appointments", "summary"],
    queryFn: () =>
      authenticatedRequest((currentSession) =>
        listAppointmentPage(currentSession, {
          from: new Date().toISOString(),
          to: new Date(Date.now() + 1000 * 60 * 60 * 24 * 14).toISOString(),
        }),
      ),
  });

  const metrics = [
    {
      title: "Pacientes",
      value: clientsQuery.isError ? "No disponible" : clientsQuery.isPending ? "Cargando" : pageCount(clientsQuery.data),
      description: clientsQuery.isError ? "No se pudo consultar la base de pacientes." : "Pacientes visibles; + indica que hay más páginas.",
      icon: <Users className="size-5" />,
    },
    {
      title: "Próximas sesiones",
      value: appointmentsQuery.isError ? "No disponible" : appointmentsQuery.isPending ? "Cargando" : pageCount(appointmentsQuery.data),
      description: appointmentsQuery.isError ? "No se pudo consultar la agenda." : "Citas en los próximos 14 días; + indica más páginas.",
      icon: <CalendarRange className="size-5" />,
    },
    {
      title: "Auditoría",
      value: session?.role === "member" ? "Restringido" : "Disponible",
      description: "Disponible para los roles de administración.",
      icon: <Shield className="size-5" />,
    },
  ];

  return (
    <div className="space-y-8">
      <section className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <Badge variant="outline" className="mb-3">
            Panel
          </Badge>
          <h2 className="text-3xl font-semibold tracking-tight">Resumen de actividad</h2>
          <p className="mt-2 max-w-3xl text-muted-foreground">
            Consulta pacientes y citas para continuar el trabajo clínico.
          </p>
        </div>
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {metrics.map((metric) => (
          <MetricCard key={metric.title} {...metric} />
        ))}
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Continuar el recorrido</CardTitle>
          <CardDescription>Abre un paciente o revisa la agenda para continuar con una sesión.</CardDescription>
        </CardHeader>
        <CardContent className="flex gap-4 text-sm font-medium text-primary">
          <Link href="/clients">Ver pacientes</Link>
          <Link href="/appointments">Ver agenda</Link>
        </CardContent>
      </Card>
    </div>
  );
}
