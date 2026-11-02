import type { Client, GoogleCalendarCandidate } from "../../../types/api";

export type PatientSuggestion = {
  key: string;
  name: string;
  events: GoogleCalendarCandidate[];
  matches: { id: string; name: string; reason: string }[];
  status: "existing" | "possible" | "new" | "unresolved";
};

export function normalizeName(value: string) {
  return value.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase()
    .replace(/[^\p{L}\s]/gu, " ").replace(/\s+/g, " ").trim();
}

function extractName(title: string): string {
  const value = title.trim()
    .replace(/^(?:sesi[oó]n|consulta|terapia|turno|paciente)(?=\s|[:\-–]|$)(?:\s+con)?\s*[:\-–]?\s*/iu, "")
    .replace(/\s+(?:sesi[oó]n|consulta|terapia|turno)\s*$/iu, "").trim();
  const words = normalizeName(value).split(" ");
  // A title is only a lead. Generic blocks and titles with metadata need manual review.
  if (!value || /\d|[@/()[\]]/.test(value) || words.length > 5 ||
    words.some((word) => ["sesion", "consulta", "terapia", "turno", "reunion", "tramite", "almuerzo", "bloque", "libre", "supervision", "personal", "clase", "clases", "curso"].includes(word))) return "";
  return value;
}

/** Read-only: only compares patients already authorized by the existing clients API. */
export function reviewCalendarPatients(events: GoogleCalendarCandidate[], clients: Client[]): PatientSuggestion[] {
  const exact = new Map<string, Client[]>();
  const tokens = new Map<string, Client[]>();
  for (const client of clients) {
    const name = normalizeName(client.fullname);
    const sameName = exact.get(name) ?? [];
    sameName.push(client);
    exact.set(name, sameName);
    for (const token of new Set(name.split(" "))) {
      const sameToken = tokens.get(token) ?? [];
      sameToken.push(client);
      tokens.set(token, sameToken);
    }
  }
  const groups = new Map<string, PatientSuggestion>();
  const seen = new Set<string>();
  for (const event of events) {
    if (seen.has(event.event_id)) continue;
    seen.add(event.event_id);
    const name = extractName(event.summary);
    const normalized = normalizeName(name);
    const key = normalized || `unresolved:${event.event_id}`;
    const group = groups.get(key);
    if (group) { group.events.push(event); continue; }
    const exactMatches = exact.get(normalized) ?? [];
    const words = normalized.split(" ").filter(Boolean);
    const candidates = exactMatches.length ? exactMatches : (tokens.get(words[0]) ?? []).filter((client) => {
      const parts = new Set(normalizeName(client.fullname).split(" "));
      return words.every((word) => parts.has(word));
    });
    groups.set(key, {
      key, name: name || "Identidad pendiente", events: [event],
      matches: candidates.map((client) => ({ id: client.id, name: client.fullname, reason: exactMatches.length ? "Nombre completo coincidente; confirmar identidad" : "Coincidencia parcial; confirmar identidad" })),
      status: !name ? "unresolved" : exactMatches.length === 1 ? "existing" : candidates.length ? "possible" : "new",
    });
  }
  return [...groups.values()];
}
