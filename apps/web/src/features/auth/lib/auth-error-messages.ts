import { ApiError } from "@/lib/http/api-client";

export function getAuthErrorMessage(error: unknown) {
  if (!(error instanceof ApiError)) {
    return "No se pudo completar la autenticacion. Reintenta.";
  }

  if (error.code === "unauthorized") {
    return "Tenant, email o password invalidos.";
  }

  if (error.code === "validation_error") {
    return "Revisa los datos de acceso e intenta de nuevo.";
  }

  if (error.code === "forbidden") {
    return "Tu rol no tiene permisos para entrar a esta seccion.";
  }

  if (error.code === "service_unavailable") {
    return "El servicio de autenticacion no esta disponible.";
  }

  return error.message || "No se pudo completar la autenticacion.";
}
