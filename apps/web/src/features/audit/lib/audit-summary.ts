import { AuditEntry } from "@/types/api";

const actionLabels: Record<string, string> = {
  "clinical_diff.merged": "Cambios longitudinales fusionados",
  "clinical_diff.operation_reviewed": "Operación longitudinal revisada",
  "session_report.approved": "Informe aprobado por una persona",
  "clinical_assignment.granted": "Profesional asignado",
  "clinical_assignment.ended": "Asignación clínica finalizada",
};

export function auditTitle(entry: AuditEntry): string {
  return actionLabels[entry.action] ?? entry.action.replaceAll("_", " ").replaceAll(".", " · ");
}

export function auditSummary(entry: AuditEntry): string {
  if (entry.action === "clinical_diff.merged") {
    const count = Number(entry.metadata.operation_count);
    if (Number.isFinite(count)) return `${count} ${count === 1 ? "operación aprobada" : "operaciones aprobadas"} incorporadas al estado longitudinal.`;
  }
  return "Cambio registrado con actor, fecha y procedencia disponibles en el detalle técnico.";
}
