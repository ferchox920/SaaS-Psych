const TENANT_STORAGE_KEY = "sessionflow.auth.tenant";
const LEGACY_SESSION_STORAGE_KEY = "sessionflow.auth.session";

function purgeLegacyTokenStorage() {
	window.localStorage.removeItem(LEGACY_SESSION_STORAGE_KEY);
}

export function getStoredTenantId(): string | null {
  if (typeof window === "undefined") {
    return null;
  }
	purgeLegacyTokenStorage();

	return window.localStorage.getItem(TENANT_STORAGE_KEY);
}

export function setStoredTenantId(tenantId: string) {
  if (typeof window === "undefined") {
    return;
  }
	purgeLegacyTokenStorage();

	window.localStorage.setItem(TENANT_STORAGE_KEY, tenantId);
}

export function clearStoredTenantId() {
  if (typeof window === "undefined") {
    return;
  }
	purgeLegacyTokenStorage();

	window.localStorage.removeItem(TENANT_STORAGE_KEY);
}
