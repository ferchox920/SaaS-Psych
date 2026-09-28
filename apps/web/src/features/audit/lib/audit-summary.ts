import { AuditEntry } from "@/types/api";

export const auditActionFilters = [
  { value: "", label: "Todos los cambios" },
  { value: "client.", label: "Pacientes" },
  { value: "appointment.", label: "Citas" },
  { value: "clinical_session.", label: "Sesiones clínicas" },
  { value: "session_note.", label: "Notas de sesión" },
  { value: "session_report.", label: "Informes" },
  { value: "clinical_assignment.", label: "Asignaciones clínicas" },
  { value: "clinical_diff.", label: "Propuestas longitudinales" },
  { value: "clinical_diff.merged", label: "Cambios longitudinales fusionados" },
  { value: "clinical_consent.", label: "Consentimiento" },
];

const entityLabels: Record<string, string> = {
  client: "Paciente", appointment: "Cita", clinical_session: "Sesión clínica",
  session_note: "Nota de sesión", session_report: "Informe", clinical_assignment: "Asignación clínica",
  clinical_diff: "Propuesta longitudinal", clinical_consent: "Consentimiento",
  clinical_process: "Proceso clínico", clinical_hypothesis: "Hipótesis",
  clinical_event: "Evento clínico", clinical_evidence: "Evidencia",
  clinical_goal: "Objetivo clínico", clinical_target: "Meta clínica",
  clinical_formulation: "Formulación clínica", clinical_audio: "Audio clínico",
  clinical_transcript: "Transcripción", clinical_ai_run: "Generación asistida",
};

export const auditEntityFilters = [
  { value: "", label: "Todas las entidades" },
  ...Object.entries(entityLabels).map(([value, label]) => ({ value, label })),
];

export function auditEntityLabel(entity: string): string {
  return entityLabels[entity] ?? "Otra entidad";
}

const actionLabels: Record<string, string> = {
  "client.create": "Paciente creado", "client.update": "Paciente actualizado",
  "client.archive": "Paciente archivado", "client.restore": "Paciente restaurado",
  "appointment.create": "Cita creada", "appointment.update": "Cita actualizada",
  "appointment.cancel": "Cita cancelada", "clinical_session.created": "Sesión clínica iniciada",
  "session_note.create": "Nota de sesión creada", "session_note.update": "Nota de sesión actualizada",
  "session_note.sign": "Nota de sesión firmada", "session_note.addendum": "Anexo de nota agregado",
  "session_report.generated": "Borrador de informe generado", "session_report.updated": "Informe actualizado",
  "session_report.approved": "Informe aprobado por una persona",
  "clinical_assignment.granted": "Profesional asignado", "clinical_assignment.ended": "Asignación clínica finalizada",
  "clinical_assignment.grant": "Profesional asignado", "clinical_assignment.end": "Asignación clínica finalizada",
  "clinical_diff.created": "Propuesta longitudinal creada", "clinical_diff.reviewed": "Propuesta longitudinal revisada",
  "clinical_diff.operation_reviewed": "Operación longitudinal revisada",
  "clinical_diff.rejected": "Propuesta longitudinal descartada", "clinical_diff.merged": "Cambios longitudinales fusionados",
  "clinical_consent.granted": "Consentimiento otorgado", "clinical_consent.revoked": "Consentimiento revocado",
  "clinical_formulation.create_draft": "Borrador de formulación creado",
  "clinical_formulation.approve": "Formulación aprobada",
};

const actionVerbs: Record<string, string> = {
  proposed: "propuesto", approved: "aprobado", rejected: "descartado", updated: "actualizado",
  created: "creado", corrected: "corregido", invalidated: "invalidado", retired: "retirado",
  closed: "cerrado", reopened: "reabierto", resolved: "resuelto", succeeded: "completado",
  failed: "fallido", queued: "encolado", cancelled: "cancelado", uploaded: "cargado",
  deleted: "eliminado", started: "iniciado", imported: "importado", generated: "generado",
};

export function auditTitle(entry: AuditEntry): string {
  const explicit = actionLabels[entry.action];
  if (explicit) return explicit;
  const [entity, action, extra] = entry.action.split(".");
  if (!extra && entityLabels[entity] && actionVerbs[action]) {
    return `${entityLabels[entity]}: cambio ${actionVerbs[action]}`;
  }
  return "Otra acción registrada";
}

export function auditSummary(entry: AuditEntry): string {
  if (entry.action === "clinical_diff.merged") {
    const count = Number(entry.metadata.operation_count);
    if (Number.isFinite(count)) return `${count} ${count === 1 ? "operación aprobada" : "operaciones aprobadas"} incorporadas al estado longitudinal.`;
  }
  if (entry.action === "session_report.approved") return "Una persona autorizada aprobó esta revisión del informe.";
  if (entry.action === "clinical_assignment.ended" || entry.action === "clinical_assignment.end") return "Se finalizó el acceso clínico de un profesional; el motivo consta en el detalle técnico.";
  return `Cambio sobre ${auditEntityLabel(entry.entity).toLowerCase()} registrado con fecha y procedencia disponibles en el detalle técnico.`;
}
