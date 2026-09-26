"use client";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Entity,
  Evidence,
  Event,
  Hypothesis,
  Process,
  Target,
  Goal,
  GIRA,
  Rationale,
  Registry,
  Transition,
  epistemicLabels,
} from "./model";
import {
  Status,
  Reference,
  Fields,
  Paged,
  useClinicalRead,
  ReadError,
} from "./shared";
export function EvidenceCard({ item: e }: { item: Evidence }) {
  return (
    <article id={`entity-${e.id}`} className="border rounded p-3 space-y-2">
      <h4 className="font-semibold">
        Evidencia · {epistemicLabels[e.epistemic_type] ?? e.epistemic_type}
      </h4>
      <p>{e.statement}</p>
      <p>
        Estado: {e.status} · v{e.version}
      </p>
      <details>
        <summary>Fuente de evidencia</summary>
        <Reference label={e.source_type} id={e.source_id} />
        <p>
          Versión fuente {e.source_version} · Ítem {e.source_item_id}
        </p>
        <Reference label="Evidencia" id={e.id} />
      </details>
    </article>
  );
}
export function EventCard({ item: e }: { item: Event }) {
  return (
    <article className="border rounded p-3 space-y-2">
      <h4>
        Evento · {e.event_type} · {e.title}
      </h4>
      <Status item={e} />
      <p>{e.description}</p>
      <p>Fecha {e.occurred_at ?? e.observed_at}</p>
      <Paged
        items={e.evidence ?? []}
        render={(v) => <EvidenceCard key={v.id} item={v} />}
      />
    </article>
  );
}
export function HypothesisCard({
  item: h,
  client,
  onDiff,
}: {
  item: Hypothesis;
  client: string;
  onDiff: (id: string) => void;
}) {
  return (
    <article id={`entity-${h.id}`} className="border rounded p-4 space-y-3">
      <h4 className="font-semibold">Hipótesis clínica · {h.statement}</h4>
      <Status item={h} />
      <p>
        Nivel de respaldo: {h.confidence_level} (no es probabilidad calculada)
      </p>
      <Reference label="Proceso" id={h.process_id} />
      <div className="grid md:grid-cols-2 gap-4">
        <section>
          <h5 className="font-semibold">
            APOYA · {h.supporting_evidence?.length ?? 0}
          </h5>
          <Paged
            items={h.supporting_evidence ?? []}
            render={(e) => <EvidenceCard key={e.id} item={e} />}
          />
        </section>
        <section>
          <h5 className="font-semibold">
            CONTRADICE · {h.contradicting_evidence?.length ?? 0}
          </h5>
          <Paged
            items={h.contradicting_evidence ?? []}
            render={(e) => <EvidenceCard key={e.id} item={e} />}
          />
        </section>
      </div>
      <History client={client} kind="hypothesis" item={h} onDiff={onDiff} />
    </article>
  );
}
export function TargetCard({ item: t }: { item: Target }) {
  return (
    <article id={`entity-${t.id}`} className="border rounded p-3">
      <h4 className="font-semibold">Target · {t.title}</h4>
      <p>Tipo: {t.target_type}</p>
      <Status item={t} />
      <p>{t.description}</p>
      <details>
        <summary>Grounding del Target</summary>
        <Fields
          value={{
            evidence_ids: t.evidence_ids,
            hypothesis_ids: t.hypothesis_ids,
            event_ids: t.event_ids,
          }}
        />
      </details>
    </article>
  );
}
export function GoalCard({
  item: g,
  client,
  onDiff,
}: {
  item: Goal;
  client: string;
  onDiff: (id: string) => void;
}) {
  return (
    <article id={`entity-${g.id}`} className="border rounded p-4 space-y-3">
      <h4 className="font-semibold">Goal · {g.title}</h4>
      <Status item={g} />
      <p>
        {g.description} · Tipo {g.goal_type} · Prioridad {g.priority}
      </p>
      <Fields value={{ targets: g.target_ids }} />
      <h5>Indicadores — no cambian el estado del Goal automáticamente</h5>
      <Paged
        items={g.indicators ?? []}
        empty="Sin indicadores vinculados"
        render={(i) => (
          <section
            key={i.id}
            id={`entity-${i.id}`}
            className="border rounded p-3"
          >
            <h6>Indicador · {i.description}</h6>
            <p>
              Tipo {i.indicator_type} · Método{" "}
              {i.measurement_method ?? "No informado"}
            </p>
            <p>
              Baseline: {i.baseline ?? "No informado"} · Objetivo:{" "}
              {i.target_value ?? "No informado"}
            </p>
            <p>Estado: {i.status}</p>
            {["supports_progress", "supports_regression", "neutral"].map(
              (relation) => (
                <div key={relation}>
                  <strong>{relation}</strong>
                  <Fields
                    value={(i.links ?? []).filter(
                      (l) => l.relation_type === relation,
                    )}
                  />
                </div>
              ),
            )}
            {(i.links ?? [])
              .filter(
                (l) =>
                  ![
                    "supports_progress",
                    "supports_regression",
                    "neutral",
                  ].includes(l.relation_type),
              )
              .map((l) => (
                <Fields key={`${l.source_id}/${l.relation_type}`} value={l} />
              ))}
          </section>
        )}
      />
      <History client={client} kind="goal" item={g} onDiff={onDiff} />
    </article>
  );
}
function RationaleCard({
  item: r,
  approaches,
  techniques,
}: {
  item: Rationale;
  approaches: Registry[];
  techniques: Registry[];
}) {
  const a = approaches.find(
    (a) => a.slug === r.approach_slug && a.version === r.approach_version,
  );
  const t = techniques.find(
    (t) => t.slug === r.technique_slug && t.version === r.technique_version,
  );
  return (
    <section id={`entity-${r.id}`} className="border rounded p-3 space-y-2">
      <h5 className="font-semibold">Rationale terapéutico (no es hipótesis)</h5>
      <p className="break-all">
        Proceso {r.process_id} → Target {r.target_id} → Goal {r.goal_id}
      </p>
      <p>{r.rationale}</p>
      <p>
        Approach: {a?.name ?? r.approach_slug} v{r.approach_version} ·{" "}
        {a?.status ?? "Registry no disponible"}
      </p>
      <p>
        Técnica: {t?.name ?? r.technique_slug ?? "No seleccionada"}{" "}
        {r.technique_version ? `v${r.technique_version}` : ""} · Approach
        compatible: {t?.approach_slug ?? r.approach_slug} v
        {t?.approach_version ?? r.approach_version}
      </p>
      <p>Señal/efecto esperado: {r.expected_effect}</p>
      <p>
        Grounding: {r.grounding_status} · Aprobación: {r.approval_status}
      </p>
      <details>
        <summary>Fuentes del rationale</summary>
        <Fields
          value={{
            evidence_ids: r.evidence_ids,
            hypothesis_ids: r.hypothesis_ids,
          }}
        />
      </details>
    </section>
  );
}
export function GIRACard({
  item: g,
  client,
  onDiff,
  onProposals,
}: {
  item: GIRA;
  client: string;
  onDiff: (id: string) => void;
  onProposals: () => void;
}) {
  const approaches = useClinicalRead<{ items: Registry[] }>(
    client,
    "/therapeutic-approaches",
  );
  const techniques = useClinicalRead<{ items: Registry[] }>(
    client,
    "/therapeutic-techniques",
  );
  return (
    <article className="border rounded p-4 space-y-4">
      <h4 className="font-semibold">
        GIRA v{g.gira_version} · {g.title} ·{" "}
        {g.clinical_status === "superseded"
          ? "Histórica / superseded"
          : g.approval_status === "approved" && g.clinical_status === "active"
            ? "Actual / active"
            : `Estado ${g.clinical_status}`}
      </h4>
      <Status item={{ ...g, version: g.entity_version }} />
      <p>{g.summary}</p>
      <Reference label="Proceso" id={g.process_id} />
      <Reference label="Sustituye GIRA" id={g.supersedes_gira_id} />
      <Fields value={{ targets: g.target_ids, goals: g.goal_ids }} />
      {(approaches.error || techniques.error) && (
        <ReadError error={approaches.error || techniques.error} />
      )}
      <Paged
        items={g.rationales ?? []}
        render={(r) => (
          <RationaleCard
            key={r.id}
            item={r}
            approaches={approaches.data?.items ?? []}
            techniques={techniques.data?.items ?? []}
          />
        )}
      />
      <h5>Fases — no son un wizard irreversible</h5>
      <Paged
        items={[...(g.phases ?? [])].sort((a, b) => a.position - b.position)}
        render={(p) => (
          <section key={p.id} className="border p-3">
            <h6>
              Fase {p.position}: {p.title} · {p.clinical_status}
            </h6>
            <p>{p.description}</p>
            <p>Entrada: {p.entry_criteria ?? "No informada"}</p>
            <p>Salida: {p.exit_criteria ?? "No informada"}</p>
            <Fields
              value={{
                goal_ids: p.goal_ids,
                indicator_ids: p.indicator_ids,
                rationale_ids: p.rationale_ids,
              }}
            />
          </section>
        )}
      />
      <History client={client} kind="gira" item={g} onDiff={onDiff} />
      <Button variant="outline" onClick={onProposals}>
        Revisar propuestas de cambio de GIRA
      </Button>
      <p>
        No hay guardado directo ni generación remota. Los cambios se revisan por
        operación en ClinicalDiff y requieren merge humano.
      </p>
    </article>
  );
}
export function ProcessCard({
  item: p,
  client,
  onDiff,
  onProposals,
}: {
  item: Process;
  client: string;
  onDiff: (id: string) => void;
  onProposals: () => void;
}) {
  const s = p.therapeutic_strategy;
  return (
    <article className="border rounded p-4 space-y-4">
      <h4 className="font-semibold">Proceso · {p.title}</h4>
      <Status item={p} />
      <p>{p.description}</p>
      <p>
        Hipótesis {p.hypotheses?.length ?? 0} · Targets{" "}
        {s?.targets?.length ?? 0} · Goals {s?.goals?.length ?? 0}
      </p>
      <details>
        <summary>Eventos y evidencia vinculada al proceso</summary>
        <Paged
          items={p.events ?? []}
          render={(e) => <EventCard key={e.id} item={e} />}
        />
      </details>
      <details>
        <summary>Hipótesis competidoras — sin ganador automático</summary>
        <Paged
          items={p.hypotheses ?? []}
          render={(h) => (
            <HypothesisCard
              key={h.id}
              item={h}
              client={client}
              onDiff={onDiff}
            />
          )}
        />
      </details>
      <details>
        <summary>Targets, Goals y GIRA</summary>
        <Paged
          items={s?.targets ?? []}
          render={(t) => <TargetCard key={t.id} item={t} />}
        />
        <Paged
          items={s?.goals ?? []}
          render={(g) => (
            <GoalCard key={g.id} item={g} client={client} onDiff={onDiff} />
          )}
        />
        <Paged
          items={s?.giras ?? []}
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
      </details>
      <History client={client} kind="process" item={p} onDiff={onDiff} />
    </article>
  );
}
export function History({
  client,
  kind,
  item,
  onDiff,
}: {
  client: string;
  kind: string;
  item: Entity;
  onDiff: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [offset, setOffset] = useState(0);
  const q = useClinicalRead<{
    items: Transition[];
    has_more: boolean;
    offset: number;
  }>(
    client,
    `/clients/${client}/clinical-history/${kind}/${item.id}?offset=${offset}`,
    open,
  );
  return (
    <div>
      <Button variant="outline" onClick={() => setOpen(!open)}>
        Historial de {kind} {item.id.slice(0, 8)}
      </Button>
      {open &&
        (q.error ? (
          <ReadError error={q.error} />
        ) : q.data ? (
          <div className="space-y-3">
            {q.data.items?.map((t) => (
              <article key={t.id} className="border p-2">
                <p>
                  {t.created_at} · {t.action} · {t.from_status ?? "Nueva"} →{" "}
                  {t.to_status} · v{t.from_version ?? 0} → v{t.to_version}
                </p>
                <Reference
                  label="Persona que fusionó"
                  id={t.merged_by_user_id ?? t.actor_user_id}
                />
                <Button variant="outline" onClick={() => onDiff(t.diff_id)}>
                  Ver ClinicalDiff fuente
                </Button>
                <details>
                  <summary>Original y modificación histórica</summary>
                  <h6>Original</h6>
                  <Fields value={t.original_proposal} />
                  <h6>Modificación humana</h6>
                  <Fields value={t.human_modification} />
                </details>
              </article>
            ))}
            <p>Transiciones desde {offset + 1}, hasta 25 por página</p>
            <Button
              disabled={!offset}
              onClick={() => setOffset(Math.max(0, offset - 25))}
            >
              Anterior historial
            </Button>
            <Button
              disabled={!q.data.has_more}
              onClick={() => setOffset(offset + 25)}
            >
              Siguiente historial
            </Button>
          </div>
        ) : (
          <p role="status">Consultando historial…</p>
        ))}
    </div>
  );
}
