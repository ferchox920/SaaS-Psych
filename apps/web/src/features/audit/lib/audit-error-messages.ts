import { ApiError } from "@/lib/http/api-client";

export function getAuditErrorMessage(error: unknown) {
  if (!(error instanceof ApiError)) {
    return "No se pudo cargar la auditoria.";
  }

  if (error.code === "validation_error") {
    return "Alguno de los filtros de auditoria es invalido.";
  }

  if (error.code === "forbidden") {
    return "Tu rol no puede acceder a auditoria.";
  }

  if (error.code === "unauthorized") {
    return "La sesion no es valida para consultar auditoria.";
  }

  return error.message || "No se pudo cargar la auditoria.";
}
