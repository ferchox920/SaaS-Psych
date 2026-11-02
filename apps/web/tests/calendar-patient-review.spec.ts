import { test, expect } from "@playwright/test";
import { reviewCalendarPatients } from "../src/features/appointments/lib/calendar-patient-review";
import type { Client, GoogleCalendarCandidate } from "../src/types/api";

const clients = ["Diego Pérez", "Diego García", "Lucía Fernández"].map((fullname, i) => ({
  id: String(i), tenant_id: "tenant", fullname, contact: "", notes_public: "", created_at: "", updated_at: "",
})) satisfies Client[];
const events = (...titles: string[]): GoogleCalendarCandidate[] => titles.map((summary, i) => ({ event_id: String(i), summary, starts_at: "", ends_at: "", location: "" }));

test("groups repeated sessions, preserves events and exposes ambiguous first names", () => {
  const groups = reviewCalendarPatients(events("Sesión: Diego", "diego", "Consulta: LUCIA FERNANDEZ"), clients);
  expect(groups).toHaveLength(2);
  expect(groups[0].events).toHaveLength(2);
  expect(groups[0].status).toBe("possible");
  expect(groups[0].matches).toHaveLength(2);
  expect(groups[1].status).toBe("existing");
  expect(groups[1].matches[0].id).toBe("2");
});

test("keeps nameless events separate and never proposes generic blocks as patients", () => {
  const groups = reviewCalendarPatients(events("Sesión", "", "Trámite", "Supervisión", "Sesión 123", "Turno: Ana López"), clients);
  expect(groups.filter((group) => group.status === "unresolved")).toHaveLength(5);
  expect(groups.at(-1)?.name).toBe("Ana López");
  expect(groups.at(-1)?.status).toBe("new");
});

test("deduplicates event IDs without collapsing different people into a suggested patient", () => {
  const input = events("Diego Pérez", "Diego García", "Diego");
  expect(reviewCalendarPatients([...input, input[0]], clients).map((group) => group.events.length)).toEqual([1, 1, 1]);
  expect(reviewCalendarPatients(events("Diego"), []) [0].matches).toEqual([]);
  expect(reviewCalendarPatients(events("Lucía Fernández"), [...clients, { ...clients[2], id: "other" }])[0].status).toBe("possible");
});

test("recognizes appointment suffixes and leaves classes and numbered titles for manual review", () => {
  const groups = reviewCalendarPatients(events("Lucía Fernández consulta", "Consulta: Lucía Fernández", "Clases de piano", "Ana 1", "Turnock"), clients);
  expect(groups[0].status).toBe("existing");
  expect(groups[0].events).toHaveLength(2);
  expect(groups[1].status).toBe("unresolved");
  expect(groups[2].status).toBe("unresolved");
  expect(groups[3].name).toBe("Turnock");
});

test("measures a synthetic weekly workload without asserting machine-dependent latency", () => {
  const input = events(...Array.from({ length: 500 }, (_, i) => i % 2 ? "Diego" : "Lucía Fernández"));
  const start = performance.now();
  const groups = reviewCalendarPatients(input, clients);
  const elapsed = performance.now() - start;
  expect(groups).toHaveLength(2);
  expect(groups.reduce((count, group) => count + group.events.length, 0)).toBe(500);
  console.log(`Synthetic 500 events / 3 patients: ${elapsed.toFixed(2)} ms`);
});
