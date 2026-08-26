import { z } from "zod";

export const clientSchema = z.object({
  fullname: z.string().trim().min(3, "El nombre completo debe tener al menos 3 caracteres."),
  contact: z.string().trim().max(160, "El contacto es demasiado largo.").optional().or(z.literal("")),
  notes_public: z.string().trim().max(1000, "Las notas publicas no pueden superar 1000 caracteres.").optional().or(z.literal("")),
});

export type ClientFormValues = z.infer<typeof clientSchema>;
