import { z } from "zod";

const canonicalUuidShape = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export const loginSchema = z.object({
  // The backend intentionally accepts canonical UUID text regardless of RFC
  // version/variant bits; legacy demo tenant IDs use that same contract.
  tenantId: z.string().trim().regex(canonicalUuidShape, "Tenant ID invalido"),
  email: z.email("Email invalido"),
  password: z.string().min(8, "Password demasiado corto"),
});
