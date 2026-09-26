export type Document = Record<string, unknown>;
export type Entity = {
  id: string;
  title?: string;
  statement?: string;
  description?: string;
  approval_status?: string;
  clinical_status?: string;
  status?: string;
  version?: number;
  process_id?: string;
  created_at?: string;
  updated_at?: string;
  created_from_ai_run_id?: string;
};
export type Evidence = Entity & {
  epistemic_type: string;
  source_type: string;
  source_id: string;
  source_version: number;
  source_item_id: string;
};
export type Event = Entity & {
  event_type: string;
  occurred_at?: string;
  observed_at: string;
  evidence: Evidence[];
};
export type Hypothesis = Entity & {
  confidence_level: string;
  supporting_evidence: Evidence[];
  contradicting_evidence: Evidence[];
};
export type Target = Entity & {
  target_type: string;
  evidence_ids: string[];
  hypothesis_ids: string[];
  event_ids: string[];
};
export type Indicator = Entity & {
  indicator_type: string;
  measurement_method?: string;
  baseline?: string;
  target_value?: string;
  links: { source_id: string; source_type: string; relation_type: string }[];
};
export type Goal = Entity & {
  goal_type: string;
  priority: string;
  target_ids: string[];
  indicators: Indicator[];
};
export type Rationale = Entity & {
  target_id: string;
  goal_id: string;
  approach_slug: string;
  approach_version: number;
  technique_slug?: string;
  technique_version?: number;
  rationale: string;
  expected_effect: string;
  grounding_status: string;
  evidence_ids: string[];
  hypothesis_ids: string[];
};
export type Phase = Entity & {
  position: number;
  entry_criteria?: string;
  exit_criteria?: string;
  goal_ids: string[];
  indicator_ids: string[];
  rationale_ids: string[];
};
export type GIRA = Entity & {
  gira_version: number;
  entity_version: number;
  summary: string;
  supersedes_gira_id?: string;
  target_ids: string[];
  goal_ids: string[];
  rationales: Rationale[];
  phases: Phase[];
};
export type Strategy = {
  targets: Target[];
  goals: Goal[];
  rationales: Rationale[];
  giras: GIRA[];
};
export type Process = Entity & {
  events: Event[];
  hypotheses: Hypothesis[];
  therapeutic_strategy?: Strategy;
};
export type Operation = {
  id: string;
  sequence: number;
  operation_type: string;
  target_entity_id: string;
  expected_entity_version?: number;
  original_proposal: Document;
  human_modification?: Document;
  review_status: string;
  reviewed_at?: string;
  reviewed_by_user_id?: string;
};
export type Diff = {
  id: string;
  client_id: string;
  status: string;
  revision: number;
  base_state_version: number;
  is_stale: boolean;
  created_at: string;
  clinical_session_id?: string;
  source_session_report_id?: string;
  source_ai_run_id?: string;
  source_external_proposal_id?: string;
  merged_state_version?: number;
  merged_by_user_id?: string;
  merged_at?: string;
  operations: Operation[];
  uncertainties: { type: string; question: string; evidence_ids: string[] }[];
};
export type State = {
  client_id: string;
  state_version: number;
  processes: Process[];
  unassigned_hypotheses: Hypothesis[];
  recent_events: Event[];
  active_evidence: Evidence[];
  open_proposals: Diff[];
};
export type Report = {
  id: string;
  clinical_session_id: string;
  version: number;
  revision: number;
  status: string;
  schema_version: string;
  report_json: Document;
  source_ai_run_id?: string;
  approved_at?: string;
  created_at: string;
};
export type Registry = {
  slug: string;
  version: number;
  name: string;
  status: string;
  approach_slug?: string;
  approach_version?: number;
};
export type Transition = {
  id: string;
  entity_type: string;
  entity_id: string;
  diff_id: string;
  action: string;
  from_version?: number;
  to_version: number;
  from_status?: string;
  to_status?: string;
  actor_user_id: string;
  merged_by_user_id?: string;
  created_at: string;
  original_proposal: Document;
  human_modification?: Document;
};
export const epistemicLabels: Record<string, string> = {
  patient_report: "Relato del paciente",
  therapist_observation: "Observación del terapeuta",
  measurement: "Medición",
  documented_fact: "Hecho documentado",
  inference: "Inferencia de IA",
};
export function counts(d: Diff) {
  const ops = d.operations ?? [];
  return {
    total: ops.length,
    reviewed: ops.filter((o) => o.review_status !== "pending").length,
    approved: ops.filter((o) => o.review_status === "approved").length,
    modified: ops.filter((o) => o.review_status === "modified").length,
    rejected: ops.filter((o) => o.review_status === "rejected").length,
  };
}
export function mergeAllowed(d: Diff, write: boolean) {
  const c = counts(d);
  return (
    write &&
    !d.is_stale &&
    d.status === "approved" &&
    c.total > 0 &&
    c.reviewed === c.total &&
    c.approved + c.modified > 0
  );
}
export function decisionAllowed(d: Diff, o: Operation, write: boolean) {
  return (
    write &&
    !d.is_stale &&
    ["pending_review", "partially_reviewed"].includes(d.status) &&
    o.review_status === "pending"
  );
}
export function sourceLabel(d: Diff) {
  return d.source_external_proposal_id
    ? "Propuesta externa manual"
    : d.source_ai_run_id
      ? "Ejecución de IA local"
      : "Fuente no disponible";
}
export function clinicalError(e: unknown) {
  const status = (e as { status?: number })?.status;
  return status === 409
    ? "El estado clínico cambió o la revisión está incompleta. Consulta la versión actual antes de decidir o fusionar. No se reintentó automáticamente."
    : status === 403
      ? "No autorizado para esta acción clínica. Revisa asignación y consentimiento."
      : status === 404
        ? "Recurso clínico no disponible para este paciente."
        : [400, 422].includes(status ?? 0)
          ? "La modificación no cumple el contrato. Revisa los campos; no se guardaron cambios."
          : status === 401
            ? "La sesión de acceso venció."
            : "No se confirmó la operación. Consulta el estado antes de volver a intentarlo.";
}
