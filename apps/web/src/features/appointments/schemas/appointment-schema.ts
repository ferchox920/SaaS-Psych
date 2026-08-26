import { z } from "zod";

export const appointmentSchema = z
  .object({
    client_id: z.string().trim().uuid("Selecciona un cliente valido."),
    starts_at: z.string().min(1, "Selecciona fecha y hora de inicio."),
    ends_at: z.string().min(1, "Selecciona fecha y hora de fin."),
    location: z.string().trim().max(160, "La ubicacion es demasiado larga.").optional().or(z.literal("")),
  })
  .superRefine((value, ctx) => {
    const startsAt = new Date(value.starts_at);
    const endsAt = new Date(value.ends_at);

    if (Number.isNaN(startsAt.getTime())) {
      ctx.addIssue({
        code: "custom",
        message: "La fecha de inicio es invalida.",
        path: ["starts_at"],
      });
    }

    if (Number.isNaN(endsAt.getTime())) {
      ctx.addIssue({
        code: "custom",
        message: "La fecha de fin es invalida.",
        path: ["ends_at"],
      });
    }

    if (!Number.isNaN(startsAt.getTime()) && !Number.isNaN(endsAt.getTime()) && startsAt >= endsAt) {
      ctx.addIssue({
        code: "custom",
        message: "La fecha de fin debe ser posterior al inicio.",
        path: ["ends_at"],
      });
    }
  });

export type AppointmentFormValues = z.infer<typeof appointmentSchema>;
