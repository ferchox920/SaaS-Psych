"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { useSession } from "@/features/auth/hooks/use-session";
import {
  approveClinicalFormulation,
  createClinicalFormulation,
  listClinicalFormulations,
} from "@/features/clinical-analysis/api/clinical-analysis-api";
import { listClients } from "@/features/clients/api/clients-api";
import { ClinicalFormulationAnchor } from "@/types/api";
import { ClinicalJourneyNav } from "./clinical-journey-nav";

const initialAnchors = (): ClinicalFormulationAnchor[] => [
  { source_id: "manual_fact_1", kind: "fact", summary: "", traffic_light: null },
  { source_id: "manual_hypothesis_1", kind: "hypothesis", summary: "", traffic_light: "yellow" },
];

export function ClinicalFormulationView() {
  const queryClient = useQueryClient();
  const { authenticatedRequest } = useSession();
  const [clientId, setClientId] = useState("");
  const [summary, setSummary] = useState("");
  const [anchors, setAnchors] = useState<ClinicalFormulationAnchor[]>(initialAnchors);

  const clientsQuery = useQuery({
    queryKey: ["clients", "clinical-formulation"],
    queryFn: () => authenticatedRequest((session) => listClients(session)),
  });
  const formulationsQuery = useQuery({
    queryKey: ["clinical-formulations", clientId],
    enabled: Boolean(clientId),
    queryFn: () => authenticatedRequest((session) => listClinicalFormulations(session, clientId)),
  });
  const createMutation = useMutation({
    mutationFn: () => authenticatedRequest((session) => createClinicalFormulation(session, clientId, { approved_summary: summary.trim(), anchors })),
    onSuccess: async () => {
      toast.success("Borrador longitudinal creado; todavía no alimenta al modelo.");
      setSummary("");
      setAnchors(initialAnchors());
      await queryClient.invalidateQueries({ queryKey: ["clinical-formulations", clientId] });
    },
    onError: (error) => toast.error(error instanceof Error ? error.message : "No se pudo crear el borrador."),
  });
  const approveMutation = useMutation({
    mutationFn: (snapshotId: string) => authenticatedRequest((session) => approveClinicalFormulation(session, clientId, snapshotId)),
    onSuccess: async () => {
      toast.success("Formulación aprobada. Reemplazará el contexto aprobado anterior.");
      await queryClient.invalidateQueries({ queryKey: ["clinical-formulations", clientId] });
    },
  });

  const canCreate = Boolean(clientId) && anchors.length >= 2 && anchors.every((item) => item.source_id.trim() && item.summary.trim());
  const snapshots = useMemo(() => formulationsQuery.data?.items ?? [], [formulationsQuery.data?.items]);

  const updateAnchor = (index: number, patch: Partial<ClinicalFormulationAnchor>) => {
    setAnchors((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, ...patch } : item));
  };

  return <div className="space-y-6">
    <header className="space-y-2"><Badge variant="outline">Memoria longitudinal</Badge><h2 className="text-3xl font-semibold">Formulación clínica aprobada</h2><p className="max-w-3xl text-muted-foreground">Solo una versión aprobada alimenta el contexto local. Los borradores, sugerencias pendientes y sugerencias descartadas quedan fuera.</p></header>
    <ClinicalJourneyNav current="/clinical-formulation" clientId={clientId} />
    <section className="grid gap-6 xl:grid-cols-[1fr_1fr]">
      <Card><CardHeader><CardTitle>Nuevo borrador</CardTitle><CardDescription>Requiere entre dos y cuatro antecedentes concretos. Aprobar es una acción posterior y explícita.</CardDescription></CardHeader><CardContent className="space-y-5">
        <div className="space-y-2"><Label htmlFor="formulation-client">Paciente</Label><select id="formulation-client" className="h-11 w-full rounded-xl border bg-background px-3" value={clientId} onChange={(event) => setClientId(event.target.value)}><option value="">Selecciona un paciente</option>{(clientsQuery.data?.items ?? []).map((client) => <option key={client.id} value={client.id}>{client.fullname}</option>)}</select></div>
        <div className="space-y-2"><Label htmlFor="approved-summary">Resumen clínico propuesto</Label><Textarea id="approved-summary" className="min-h-32" value={summary} onChange={(event) => setSummary(event.target.value)} placeholder="Resumen profesional breve, revisado por el profesional tratante." /></div>
        <div className="space-y-3"><div className="flex items-center justify-between"><Label>Antecedentes concretos</Label><Button size="sm" variant="outline" disabled={anchors.length >= 4} onClick={() => setAnchors((current) => [...current, { source_id: `manual_anchor_${current.length + 1}`, kind: "fact", summary: "", traffic_light: null }])}><Plus className="size-4" /> Agregar</Button></div>
          {anchors.map((anchor, index) => <div key={index} className="space-y-3 rounded-2xl border p-4">
            <div className="grid gap-3 md:grid-cols-[1fr_0.7fr_auto]"><Input aria-label={`Fuente ${index + 1}`} value={anchor.source_id} onChange={(event) => updateAnchor(index, { source_id: event.target.value })} /><select className="h-10 rounded-xl border bg-background px-3" value={anchor.kind} onChange={(event) => updateAnchor(index, { kind: event.target.value as "fact" | "hypothesis", traffic_light: event.target.value === "fact" ? null : "yellow" })}><option value="fact">HECHO</option><option value="hypothesis">HIPÓTESIS</option></select><Button aria-label="Eliminar antecedente" size="icon" variant="ghost" disabled={anchors.length <= 2} onClick={() => setAnchors((current) => current.filter((_, itemIndex) => itemIndex !== index))}><Trash2 className="size-4" /></Button></div>
            <Textarea aria-label={`Resumen de antecedente ${index + 1}`} className="min-h-20" value={anchor.summary} onChange={(event) => updateAnchor(index, { summary: event.target.value })} placeholder="Contenido concreto, sin inferencias no etiquetadas." />
            {anchor.kind === "hypothesis" ? <select className="h-10 rounded-xl border bg-background px-3" value={anchor.traffic_light ?? "yellow"} onChange={(event) => updateAnchor(index, { traffic_light: event.target.value as "green" | "yellow" | "red" })}><option value="green">VERDE</option><option value="yellow">AMARILLO</option><option value="red">ROJO</option></select> : null}
          </div>)}
        </div>
        <Button disabled={!canCreate || createMutation.isPending} onClick={() => createMutation.mutate()}>Crear borrador versionado</Button>
      </CardContent></Card>

      <Card><CardHeader><CardTitle>Historial de formulación</CardTitle><CardDescription>La aprobación sustituye la versión activa, pero preserva todas las anteriores.</CardDescription></CardHeader><CardContent className="space-y-4">
        {!clientId ? <p className="text-sm text-muted-foreground">Selecciona un paciente para consultar su historial.</p> : null}
        {clientId && snapshots.length === 0 && !formulationsQuery.isLoading ? <p className="text-sm text-muted-foreground">No hay formulaciones para este paciente.</p> : null}
        {snapshots.map((snapshot) => <div key={snapshot.id} className={`rounded-2xl border p-4 ${snapshot.status === "approved" ? "border-emerald-400 bg-emerald-50/40" : ""}`}><div className="flex items-center justify-between gap-2"><div className="flex items-center gap-2"><Badge>v{snapshot.version}</Badge><Badge variant="outline">{snapshot.status}</Badge></div>{snapshot.status === "draft" ? <Button size="sm" disabled={approveMutation.isPending} onClick={() => approveMutation.mutate(snapshot.id)}><CheckCircle2 className="size-4" /> Aprobar</Button> : null}</div>{snapshot.approved_summary ? <p className="mt-3 text-sm leading-6">{snapshot.approved_summary}</p> : null}<ul className="mt-3 space-y-2">{snapshot.anchors.map((anchor) => <li key={anchor.source_id} className="text-sm"><span className="font-medium">{anchor.kind === "fact" ? "HECHO" : `HIPÓTESIS ${anchor.traffic_light?.toUpperCase()}`}:</span> {anchor.summary}</li>)}</ul></div>)}
      </CardContent></Card>
    </section>
  </div>;
}
