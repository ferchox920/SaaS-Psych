import { ApiError } from "@/lib/http/api-client";

export function getAppointmentErrorMessage(error: unknown) {
  if (!(error instanceof ApiError)) {
    return "No se pudo completar la operacion sobre la agenda.";
  }

  if (error.code === "validation_error") {
    return "Revisa horario, cliente y ubicacion antes de guardar.";
  }

  if (error.code === "not_found") {
    return "La cita o el cliente ya no existen en este tenant.";
  }

  if (error.code === "forbidden") {
    return "El cliente no pertenece al tenant actual.";
  }

  if (error.code === "conflict") {
    return "La cita cambió o se superpone con otro turno. Recarga la agenda antes de reintentar.";
  }

  if (error.code === "unauthorized") {
    return "La sesion no es valida para operar sobre la agenda.";
  }

  return error.message || "No se pudo completar la operacion sobre la agenda.";
}
