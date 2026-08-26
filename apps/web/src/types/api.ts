export type ListEnvelope<T> = {
  items: T[];
};

export type Client = {
  id: string;
  tenant_id: string;
  fullname: string;
  contact: string;
  notes_public: string;
  created_at: string;
  updated_at: string;
	archived_at?: string;
	archived_by_user_id?: string;
	archive_reason?: string;
};

export type ClientUpsertInput = {
  fullname: string;
  contact: string;
  notes_public: string;
};

export type Appointment = {
  id: string;
  tenant_id: string;
  client_id: string;
  starts_at: string;
  ends_at: string;
  status: string;
  location: string;
  created_at: string;
  updated_at: string;
};

export type AppointmentCreateInput = {
  client_id: string;
  starts_at: string;
  ends_at: string;
  location: string;
};

export type AppointmentUpdateInput = {
  starts_at: string;
  ends_at: string;
  location: string;
};

export type GoogleCalendarStatus = {
  enabled: boolean;
  connected: boolean;
  reauthorization_required: boolean;
  calendar_id?: string;
  last_sync_at?: string;
};

export type GoogleCalendarCandidate = {
  event_id: string;
  summary: string;
  location: string;
  starts_at: string;
  ends_at: string;
  html_link?: string;
};

export type SessionNote = {
  id: string;
  tenant_id: string;
  appointment_id: string;
  author_user_id: string;
  body: string;
  is_private: boolean;
  status: "draft" | "signed";
  current_version: number;
  signed_at?: string;
  created_at: string;
  updated_at: string;
};

export type SessionNoteUpsertInput = {
  body: string;
  is_private: boolean;
};

export type AuditEntry = {
  id: string;
  tenant_id: string;
  actor_user_id?: string;
  action: string;
  entity: string;
  entity_id?: string;
  metadata: Record<string, unknown>;
  created_at: string;
};

export type AuditFilters = {
  actionPrefix?: string;
  entity?: string;
  from?: string;
  to?: string;
  order?: "asc" | "desc";
  limit?: number;
};

export type AuditPageParam = {
  cursor?: string;
  cursor_id?: string;
};

export type AuditEnvelope = {
  items: AuditEntry[];
  pagination: {
    limit: number;
    offset: number;
    count: number;
    total_count: number;
    next_cursor?: string;
    next_cursor_id?: string;
  };
};

export type ClinicalTrafficLight = "green" | "yellow" | "red";
export type ClinicalEpistemicLevel = "fact" | "inference" | "hypothesis";
export type ClinicalNowAction =
  | "listen"
  | "clarify"
  | "reflect"
  | "explore"
  | "confront"
  | "restructure"
  | "resignify"
  | "values"
  | "regulate"
  | "no_intervention";

export type ClinicalAnalysisResult = {
  mode: "live";
  node: string;
  hypothesis: {
    text: string;
    epistemic_level: ClinicalEpistemicLevel;
    traffic_light: ClinicalTrafficLight;
  };
  evidence: Array<{
    source_id: string;
    kind: "fact" | "inference";
    summary: string;
  }>;
  now: ClinicalNowAction;
  caution: string;
  suggested_interventions: string[];
  therapist_meta: string | null;
  risk: {
    detected: boolean;
    category: string | null;
    requires_human_assessment: boolean;
  };
};

export type ClinicalAnalysisOutput = {
  result: ClinicalAnalysisResult;
  metrics: {
    first_token: number;
    total: number;
    eval_count: number;
    eval_tokens_per_second: number;
    repaired: boolean;
  };
  prompt_version: string;
  suggestion_id?: string;
  risk_protocol?: string;
};

export type ClinicalReviewResult = {
  mode: "review";
  emerging_formulation: string;
  effective_interventions: string[];
  weak_or_risky_interventions: string[];
  hypothesis_calibration: string;
  patient_responses: string;
  alliance: string;
  therapist_patterns: string[];
  next_focus: string[];
  evidence: Array<{ source_id: string; kind: "fact" | "inference"; summary: string }>;
  caution: string;
  risk: { detected: boolean; category: string | null; requires_human_assessment: boolean };
};

export type ClinicalReviewOutput = {
  result: ClinicalReviewResult;
  metrics: ClinicalAnalysisOutput["metrics"];
  prompt_version: string;
  suggestion_id?: string;
  risk_protocol?: string;
};

export type ClinicalSuggestionDisposition = "pending" | "accepted" | "corrected" | "discarded" | "postponed";

export type ClinicalSuggestion = {
  id: string;
  tenant_id: string;
  client_id: string;
  appointment_id: string;
  created_by_user_id: string;
  mode: "live" | "review";
  prompt_version: string;
  result: ClinicalAnalysisResult | ClinicalReviewResult;
  disposition: ClinicalSuggestionDisposition;
  correction_text: string;
  decision_reason: string;
  decided_by_user_id?: string;
  decided_at?: string;
  created_at: string;
};

export type ClinicalFormulationAnchor = {
  id?: string;
  source_id: string;
  kind: "fact" | "hypothesis";
  summary: string;
  traffic_light?: ClinicalTrafficLight | null;
  source_suggestion_id?: string | null;
};

export type ClinicalFormulationSnapshot = {
  id: string;
  tenant_id: string;
  client_id: string;
  version: number;
  approved_summary: string;
  status: "draft" | "approved" | "superseded";
  created_by_user_id: string;
  approved_by_user_id?: string;
  approved_at?: string;
  created_at: string;
  anchors: ClinicalFormulationAnchor[];
};

export type LocalModelStatus = {
  available: boolean;
  busy: boolean;
  configured_model: string;
  message?: string;
  models?: Array<{
    name: string;
    size?: number;
    loaded: boolean;
    configured: boolean;
  }>;
};

export type LocalTranscriptionStatus = {
  enabled: boolean;
  available: boolean;
  busy: boolean;
  engine?: string;
  model?: string;
  device?: string;
  compute_type?: string;
};

export type LocalTranscriptionResult = {
  text: string;
  language: string;
  duration_seconds: number;
  transcription_seconds: number;
  real_time_factor: number;
  engine: string;
  model: string;
};
