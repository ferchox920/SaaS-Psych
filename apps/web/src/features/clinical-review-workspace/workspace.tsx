"use client";
import Link from "next/link";
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/features/auth/hooks/use-session";
import { ClinicalJourneyNav } from "@/features/clinical-analysis/components/clinical-journey-nav";
import { Button } from "@/components/ui/button";
import { DiffInbox } from "./diffs";
import { Reports } from "./reports";
import { ProjectBridge } from "../project-bridge/workspace";
import {
  EvidenceCard,
  EventCard,
  HypothesisCard,
  ProcessCard,
  TargetCard,
  GoalCard,
  GIRACard,
} from "./knowledge";
import {
  Document,
  Evidence,
  Event,
  Hypothesis,
  Process,
  Target,
  Goal,
  GIRA,
  State,
} from "./model";
import { useClinicalRead, useClinicalSessions, ReadError, Fields, Paged } from "./shared";
import type { Client } from "@/types/api";
const tabs = [
  "Resumen",
  "Reportes",
  "Propuestas",
  "Procesos",
  "Evidencia y eventos",
  "Hipótesis",
  "Targets y Goals",
  "GIRA",
  "Contexto aprobado",
  "GPT Project",
  "Historia",
];
export function ClinicalReviewWorkspace({ client }: { client: string }) {
  const { session } = useSession();
  return session ? (
    <Workspace
      key={`${session.tenantId}/${session.userId}/${client}`}
      client={client}
      tenantId={session.tenantId}
    />
  ) : (
    <p role="status">Verificando acceso…</p>
  );
}
function Workspace({ client, tenantId }: { client: string; tenantId: string }) {
  const [tab, setTab] = useState("Resumen");
  const [diff, setDiff] = useState("");
  const cache = useQueryClient();
  const access = useClinicalSessions(client);
  const patient = useClinicalRead<Client>(client, `/clients/${client}`, !!access.data && !access.error);
  const patientName = patient.data?.id === client && patient.data.tenant_id === tenantId ? patient.data.fullname : null;
  const write = !!access.data?.can_write && !access.error;
  const selectDiff = (id: string) => {
    setDiff(id);
    setTab("Propuestas");
  };
  const refresh = async () => {
    await cache.invalidateQueries({ queryKey: ["clinical-review"] });
  };
  if (access.error)
    return (
      <div>
        <Link href={`/clients/${client}/session`}>Volver a sesión</Link>
        <ReadError error={access.error} />
        <Button onClick={() => void access.refetch()}>Consultar acceso</Button>
      </div>
    );
  if (!access.data) return <p role="status">Consultando asignación clínica…</p>;
  return (
    <main className="max-w-6xl mx-auto space-y-5 min-w-0">
      <header className="space-y-2">
        <Link className="underline" href={`/clients/${client}/session`}>
          Volver a sesión y transcripción
        </Link>
        <h2 className="text-3xl font-semibold">
          Revisión clínica y estado longitudinal
        </h2>
        <p className="break-words">
          Paciente: {patientName ?? (patient.isPending ? "Consultando identidad…" : "Identidad no disponible")} ·{" "}
          {write ? "CA-W: terapeuta tratante" : "Solo lectura clínica"}
        </p>
        {patient.error ? <p role="alert" className="text-sm text-destructive">No se pudo cargar el nombre del paciente.</p> : null}
        <Button variant="outline" onClick={() => void refresh()}>
          Actualizar estado clínico
        </Button>
      </header>
      <ClinicalJourneyNav current="/clients" clientId={client} />
      <nav aria-label="Secciones clínicas" className="flex flex-wrap gap-2">
        {tabs.map((t) => (
          <Button
            key={t}
            variant={tab === t ? "default" : "outline"}
            aria-pressed={tab === t}
            onClick={() => setTab(t)}
          >
            {t}
          </Button>
        ))}
      </nav>
      {tab === "GPT Project" ? (
        <ProjectBridge client={client} write={write} onDiff={selectDiff} />
      ) : tab === "Reportes" ? (
        <Reports
          client={client}
          sessions={access.data.items ?? []}
          write={write}
          onDiff={selectDiff}
        />
      ) : tab === "Propuestas" ? (
        <DiffInbox
          client={client}
          write={write}
          selected={diff}
          onSelect={selectDiff}
          onMerged={refresh}
        />
      ) : (
        <KnowledgeTab
          key={tab}
          client={client}
          tab={tab}
          onDiff={selectDiff}
          onProposals={() => setTab("Propuestas")}
        />
      )}
    </main>
  );
}
function KnowledgeTab({
  client,
  tab,
  onDiff,
  onProposals,
}: {
  client: string;
  tab: string;
  onDiff: (id: string) => void;
  onProposals: () => void;
}) {
  const base = `/clients/${client}`;
  const [evidenceOffset, setEvidenceOffset] = useState(0);
  const [eventOffset, setEventOffset] = useState(0);
  const [hypothesisOffset, setHypothesisOffset] = useState(0);
  const [targetOffset, setTargetOffset] = useState(0);
  const [goalOffset, setGoalOffset] = useState(0);
  const [giraOffset, setGiraOffset] = useState(0);
  const [processOffset, setProcessOffset] = useState(0);
  const state = useClinicalRead<State>(
    client,
    `${base}/longitudinal-state`,
    tab === "Resumen",
  );
  const processes = useClinicalRead<{ items: Process[]; next_offset: number | null }>(
    client,
    `${base}/processes?limit=25&offset=${processOffset}`,
    tab === "Procesos" || tab === "Historia",
  );
  const evidence = useClinicalRead<{ items: Evidence[]; has_more: boolean }>(
    client,
    `${base}/evidence-page?offset=${evidenceOffset}`,
    tab === "Evidencia y eventos",
  );
  const events = useClinicalRead<{ items: Event[]; next_offset: number | null }>(
    client,
    `${base}/events?limit=25&offset=${eventOffset}`,
    tab === "Evidencia y eventos",
  );
  const hypotheses = useClinicalRead<{ items: Hypothesis[]; next_offset: number | null }>(
    client,
    `${base}/hypotheses?limit=25&offset=${hypothesisOffset}`,
    tab === "Hipótesis",
  );
  const targets = useClinicalRead<{ items: Target[]; next_offset: number | null }>(
    client,
    `${base}/targets?limit=25&offset=${targetOffset}`,
    tab === "Targets y Goals",
  );
  const goals = useClinicalRead<{ items: Goal[]; next_offset: number | null }>(
    client,
    `${base}/goals?limit=25&offset=${goalOffset}`,
    tab === "Targets y Goals",
  );
  const giras = useClinicalRead<{ items: GIRA[]; next_offset: number | null }>(
    client,
    `${base}/giras?limit=25&offset=${giraOffset}`,
    tab === "GIRA",
  );
  const context = useClinicalRead<Document>(
    client,
    `${base}/approved-clinical-context`,
    tab === "Contexto aprobado",
  );
  const err = [
    state,
    processes,
    evidence,
    events,
    hypotheses,
    targets,
    goals,
    giras,
    context,
  ].find((q) => q.error)?.error;
  if (err) return <ReadError error={err} />;
  const activeQueries =
    tab === "Evidencia y eventos"
      ? [evidence, events]
      : tab === "Hipótesis"
        ? [hypotheses]
        : tab === "Targets y Goals"
          ? [targets, goals]
          : tab === "GIRA"
            ? [giras]
            : [];
  if (activeQueries.some((q) => q.isPending))
    return <p role="status">Consultando {tab}…</p>;
  return (
    <section className="space-y-4">
      <h3 className="text-xl font-semibold">{tab}</h3>
      {tab === "Resumen" &&
        (state.data ? (
          <>
            <p>Estado longitudinal fusionado · v{state.data.state_version}</p>
            <p>
              Procesos activos aprobados:{" "}
              {state.data.processes?.filter(
                (p) =>
                  p.clinical_status === "active" &&
                  p.approval_status === "approved",
              ).length ?? 0}
            </p>
            <p>
              Propuestas abiertas (separadas del conocimiento aprobado):{" "}
              {state.data.open_proposals?.length ?? 0}
            </p>
            <Button variant="outline" onClick={onProposals}>
              Revisar propuestas pendientes
            </Button>
            <h4>Procesos aprobados, hipótesis y estrategia actual</h4>
            <Paged
              items={state.data.processes ?? []}
              empty="Sin procesos aprobados todavía."
              render={(p) => (
                <ProcessCard
                  key={p.id}
                  item={p}
                  client={client}
                  onDiff={onDiff}
                  onProposals={onProposals}
                />
              )}
            />
            <h4>Hipótesis aprobadas sin proceso asignado</h4>
            <Paged
              items={state.data.unassigned_hypotheses ?? []}
              empty="No hay hipótesis aprobadas sin proceso."
              render={(h) => (
                <HypothesisCard
                  key={h.id}
                  item={h}
                  client={client}
                  onDiff={onDiff}
                />
              )}
            />
            <h4>Eventos recientes</h4>
            <Paged
              items={state.data.recent_events ?? []}
              render={(e) => <EventCard key={e.id} item={e} />}
            />
          </>
        ) : (
          <p role="status">Cargando resumen…</p>
        ))}
      {(tab === "Procesos" || tab === "Historia") &&
        (processes.data ? (
          <>
            <p>
              Se muestran estados de aprobación y clínicos por separado,
              incluidos históricos. La historia se consulta por entidad.
            </p>
            <Paged
              items={processes.data.items ?? []}
              render={(p) => (
                <ProcessCard
                  key={p.id}
                  item={p}
                  client={client}
                  onDiff={onDiff}
                  onProposals={onProposals}
                />
              )}
            />
            <div className="flex gap-2">
              <Button
                disabled={processOffset === 0}
                onClick={() => setProcessOffset(Math.max(0, processOffset - 25))}
              >
                Procesos anteriores
              </Button>
              <Button
                disabled={processes.data.next_offset == null}
                onClick={() => setProcessOffset(processes.data.next_offset ?? processOffset)}
              >
                Más procesos
              </Button>
            </div>
          </>
        ) : (
          <p role="status">Cargando procesos…</p>
        ))}
      {tab === "Evidencia y eventos" && (
        <>
          <h4>Evidencia — tipo epistémico explícito</h4>
          <Paged
            items={evidence.data?.items ?? []}
            render={(e) => <EvidenceCard key={e.id} item={e} />}
          />
          <h4>Eventos clínicos</h4>
          <div className="flex gap-2">
            <Button
              disabled={!evidenceOffset}
              onClick={() =>
                setEvidenceOffset(Math.max(0, evidenceOffset - 25))
              }
            >
              Evidencia anterior
            </Button>
            <Button
              disabled={!evidence.data?.has_more}
              onClick={() => setEvidenceOffset(evidenceOffset + 25)}
            >
              Más evidencia
            </Button>
          </div>
          <Paged
            items={events.data?.items ?? []}
            render={(e) => <EventCard key={e.id} item={e} />}
          />
          <div className="flex gap-2">
            <Button
              disabled={eventOffset === 0 || !events.data}
              onClick={() => setEventOffset(Math.max(0, eventOffset - 25))}
            >
              Eventos anteriores
            </Button>
            <Button
              disabled={events.data?.next_offset == null}
              onClick={() => setEventOffset(events.data?.next_offset ?? eventOffset)}
            >
              Más eventos
            </Button>
          </div>
        </>
      )}
      {tab === "Hipótesis" && (
        <>
          <p>
            Las hipótesis competidoras coexisten. No se calcula un ganador ni
            probabilidades. Incluye retiradas/rechazadas si el servidor las
            devuelve.
          </p>
          <Paged
            items={hypotheses.data?.items ?? []}
            empty="No hay hipótesis clínicas todavía."
            render={(h) => (
              <HypothesisCard
                key={h.id}
                item={h}
                client={client}
                onDiff={onDiff}
              />
            )}
          />
          <div className="flex gap-2">
            <Button
              disabled={hypothesisOffset === 0 || !hypotheses.data}
              onClick={() => setHypothesisOffset(Math.max(0, hypothesisOffset - 25))}
            >
              Hipótesis anteriores
            </Button>
            <Button
              disabled={hypotheses.data?.next_offset == null}
              onClick={() => setHypothesisOffset(hypotheses.data?.next_offset ?? hypothesisOffset)}
            >
              Más hipótesis
            </Button>
          </div>
        </>
      )}
      {tab === "Targets y Goals" && (
        <>
          <h4>Targets (no son Goals)</h4>
          <Paged
            items={targets.data?.items ?? []}
            render={(t) => <TargetCard key={t.id} item={t} />}
          />
          <div className="flex gap-2">
            <Button
              disabled={targetOffset === 0 || !targets.data}
              onClick={() => setTargetOffset(Math.max(0, targetOffset - 25))}
            >
              Targets anteriores
            </Button>
            <Button
              disabled={targets.data?.next_offset == null}
              onClick={() => setTargetOffset(targets.data?.next_offset ?? targetOffset)}
            >
              Más targets
            </Button>
          </div>
          <h4>Goals e indicadores</h4>
          <Paged
            items={goals.data?.items ?? []}
            render={(g) => (
              <GoalCard key={g.id} item={g} client={client} onDiff={onDiff} />
            )}
          />
          <div className="flex gap-2">
            <Button
              disabled={goalOffset === 0 || !goals.data}
              onClick={() => setGoalOffset(Math.max(0, goalOffset - 25))}
            >
              Goals anteriores
            </Button>
            <Button
              disabled={goals.data?.next_offset == null}
              onClick={() => setGoalOffset(goals.data?.next_offset ?? goalOffset)}
            >
              Más goals
            </Button>
          </div>
        </>
      )}
      {tab === "GIRA" && (
        <>
          <p>
            Versiones aprobadas y propuestas según servidor. Superseded
            permanece visible. No se activa GIRA remoto.
          </p>
          <Paged
            items={giras.data?.items ?? []}
            empty="No hay GIRA actual ni versiones históricas."
            render={(g) => (
              <GIRACard
                key={g.id}
                item={g}
                client={client}
                onDiff={onDiff}
                onProposals={onProposals}
              />
            )}
          />
          <div className="flex gap-2">
            <Button
              disabled={giraOffset === 0 || !giras.data}
              onClick={() => setGiraOffset(Math.max(0, giraOffset - 25))}
            >
              GIRA anteriores
            </Button>
            <Button
              disabled={giras.data?.next_offset == null}
              onClick={() => setGiraOffset(giras.data?.next_offset ?? giraOffset)}
            >
              Más GIRA
            </Button>
          </div>
        </>
      )}
      {tab === "Contexto aprobado" &&
        (context.data ? (
          <>
            <p>
              Formulación y reportes aprobados disponibles para este paciente.
              Los candidatos del reporte no son hipótesis
              longitudinales aprobadas.
            </p>
            <Fields value={context.data} />
          </>
        ) : (
          <p role="status">Consultando contexto aprobado…</p>
        ))}
    </section>
  );
}
