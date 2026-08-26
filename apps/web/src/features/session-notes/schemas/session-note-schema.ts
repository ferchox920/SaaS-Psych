import { z } from "zod";

export const sessionNoteSchema = z.object({
  body: z.string().trim().min(3, "La nota debe tener al menos 3 caracteres.").max(4000, "La nota es demasiado larga."),
  is_private: z.boolean(),
});

export type SessionNoteFormValues = z.infer<typeof sessionNoteSchema>;
