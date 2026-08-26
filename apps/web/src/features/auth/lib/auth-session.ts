import { AuthSession } from "@/features/auth/lib/auth-types";

const REFRESH_BUFFER_MS = 60_000;

export function getSessionExpiresIn(session: AuthSession) {
  return new Date(session.expiresAt).getTime() - Date.now();
}

export function isSessionExpired(session: AuthSession, bufferMs = 0) {
  return getSessionExpiresIn(session) <= bufferMs;
}

export function getRefreshDelay(session: AuthSession) {
  return Math.max(getSessionExpiresIn(session) - REFRESH_BUFFER_MS, 0);
}
