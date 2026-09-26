import { expect, test } from "@playwright/test";
import { appointmentSchema } from "../src/features/appointments/schemas/appointment-schema";

const validDates = {
  starts_at: "2026-10-02T10:00",
  ends_at: "2026-10-02T11:00",
  location: "Cita ficticia",
};

test("appointment client ID accepts canonical UUIDs parsed by the API, including the demo client", () => {
  expect(appointmentSchema.safeParse({ ...validDates, client_id: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11" }).success).toBe(true);
  expect(appointmentSchema.safeParse({ ...validDates, client_id: "not-a-uuid" }).success).toBe(false);
  expect(appointmentSchema.safeParse({ ...validDates, client_id: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeee11extra" }).success).toBe(false);
});
