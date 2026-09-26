import { ApiError } from "@/lib/http/api-client";
import type { Document } from "../clinical-review-workspace/model";
export const IMPORT_LIMIT = 1024 * 1024;
export type ExportRecord = {
  export_id: string;
  client_id: string;
  content_hash: string;
  state_version: number;
  generated_at: string;
  markdown: string;
  artifact: Document;
};
export type ExportPage = {
  items: {
    export_id: string;
    generated_at: string;
    state_version: number;
    content_hash: string;
    process_ref: string;
  }[];
  offset: number;
  has_more: boolean;
  requires_external_manual_consent: boolean;
};
export type ExternalRecord = {
  content_hash: string;
  proposal: Document & { provenance: Document };
  source_export_id: string;
};
export const byteSize = (text: string) =>
  new TextEncoder().encode(text).byteLength;
// Deliberately no parse/stringify: duplicate keys and original input reach the strict decoder.
export function importInputError(raw: string) {
  if (!raw.trim()) return "Pegá el bloque JSON estructurado.";
  if (byteSize(raw) > IMPORT_LIMIT)
    return "El límite de importación es 1 MiB (UTF-8).";
  if (!raw.trimStart().startsWith("{"))
    return "Formato no reconocido. Pegá solo el objeto JSON, sin cercas Markdown ni comentarios.";
  return null;
}
export function bridgeError(error: unknown) {
  if (error instanceof ApiError) {
    if (error.status === 409)
      return "Conflicto: el export puede estar desactualizado, el hash no coincide o la propuesta ya fue importada. Consultá el estado y generá un nuevo export si cambió; no se reintentó ni se hizo rebase.";
    if (error.status === 400 || error.status === 422)
      return "Importación o solicitud inválida: revisá el esquema, los campos requeridos y las referencias exactas del export. Una referencia desconocida no se aproxima ni se corrige automáticamente.";
    if (error.status === 403)
      return "Acción no autorizada. Verificá la asignación clínica y el consentimiento externo manual vigente.";
    if (error.status === 404)
      return "Export o propuesta no disponible para este paciente.";
    if (error.status === 401) return "La sesión expiró. Volvé a autenticarte.";
  }
  return "No se pudo completar la acción. No se confirmó ningún cambio; consultá el estado antes de intentarlo nuevamente.";
}
