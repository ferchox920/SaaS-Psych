import { ApiError } from "@/lib/http/api-client";

export function getSessionNoteErrorMessage(error: unknown) {
  if (!(error instanceof ApiError)) {
    return "No se pudo completar la operacion sobre notas de sesion.";
  }

  if (error.code === "validation_error") {
    return "Revisa el contenido de la nota antes de guardar.";
  }

  if (error.code === "not_found") {
    return "La nota o la cita ya no existen en este tenant.";
  }

  if (error.code === "forbidden") {
    return "No tienes permisos para ver o editar esta nota.";
  }

  if (error.code === "unauthorized") {
    return "La sesion no es valida para operar sobre notas.";
  }

  return error.message || "No se pudo completar la operacion sobre notas de sesion.";
}
