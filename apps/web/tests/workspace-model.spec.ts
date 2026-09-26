import { test, expect } from "@playwright/test";
import {
  scopes,
  granted,
  validateAudio,
  MAX_AUDIO_BYTES,
  terminal,
  canRetry,
  canCancel,
  acceptJob,
  safeError,
  workspaceKey,
  type Job,
  type Consent,
} from "../src/features/session-workspace/model";

test("consent is scoped, effective and never inferred", () => {
  const c: Consent = {
    id: "synthetic",
    scope: "AUDIO_RECORDING",
    status: "granted",
    definition_version: 1,
    granted_at: "2026-01-01",
    effective_from: "2026-01-01",
  };
  const now = Date.parse("2026-02-01");
  expect(Object.keys(scopes)).toHaveLength(4);
  expect(granted([c], "AUDIO_RECORDING", now)).toBe(true);
  expect(granted([c], "LOCAL_TRANSCRIPTION", now)).toBe(false);
  expect(granted([{ ...c, status: "revoked" }], c.scope, now)).toBe(false);
  expect(granted([{ ...c, effective_from: "2027-01-01" }], c.scope, now)).toBe(
    false,
  );
});
test("audio limit and MIME boundary", () => {
  expect(validateAudio(MAX_AUDIO_BYTES, "audio/wav")).toBeNull();
  expect(validateAudio(MAX_AUDIO_BYTES + 1, "audio/wav")).toContain("25 MiB");
  expect(validateAudio(0, "audio/wav")).toContain("vacío");
  expect(validateAudio(1, "text/plain")).toContain("Formato");
});
test("terminal jobs stop, budget is preserved, terminal cache rejects stale success", () => {
  const j: Job = {
    id: "synthetic",
    session_id: "s",
    job_type: "transcribe_audio",
    status: "queued",
    attempt: 1,
    max_attempts: 3,
    error_code: "timeout",
  };
  expect(terminal(j)).toBe(false);
  expect(canRetry(j)).toBe(true);
  expect(canCancel(j)).toBe(true);
  expect(canRetry({ ...j, attempt: 3 })).toBe(false);
  expect(canRetry({ ...j, error_code: "invalid_output" })).toBe(false);
  for (const status of ["failed", "cancelled", "succeeded"] as const) {
    const done = { ...j, status };
    expect(terminal(done)).toBe(true);
    expect(canRetry(done)).toBe(false);
    expect(canCancel(done)).toBe(false);
    expect(acceptJob(done, { ...j, status: "succeeded" })).toEqual(done);
  }
});
test("query isolation covers tenant, actor, client and session", () => {
  const keys = [
    workspaceKey("t", "u", "c", "s"),
    workspaceKey("t2", "u", "c", "s"),
    workspaceKey("t", "u2", "c", "s"),
    workspaceKey("t", "u", "c2", "s"),
    workspaceKey("t", "u", "c", "s2"),
  ];
  expect(new Set(keys.map((k) => JSON.stringify(k))).size).toBe(5);
});
test("safe errors never reveal backend/provider diagnostics", () => {
  for (const status of [401, 403, 404, 409, 413, 415, 422, 500, 503]) {
    expect(
      safeError({
        status,
        message: "SECRET SQL / raw clinical prompt",
        details: "SECRET",
      }),
    ).not.toContain("SECRET");
  }
  expect(safeError({ status: 409 })).toContain("terminal");
  expect(safeError({ status: 503 })).toContain("temporalmente");
});
