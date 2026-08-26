import { ApiError } from "@/lib/http/api-client";

export function getClientErrorMessage(error: unknown) {
  if (!(error instanceof ApiError)) {
    return "No se pudo completar la operacion sobre clientes.";
  }

  if (error.code === "validation_error") {
    return "Revisa los datos del cliente e intenta nuevamente.";
  }

  if (error.code === "not_found") {
    return "El cliente ya no existe o no pertenece al tenant actual.";
  }

  if (error.code === "forbidden") {
    return "No tienes permisos para modificar este cliente.";
  }

  if (error.code === "unauthorized") {
    return "La sesion no es valida para operar sobre clientes.";
  }

  return error.message || "No se pudo completar la operacion sobre clientes.";
}
