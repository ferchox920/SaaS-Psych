export function safeLoginNext(value: unknown): string {
  if (typeof value !== "string" || !value.startsWith("/") || value.startsWith("//") || value.includes("\\") || /[\u0000-\u001f\u007f]/.test(value)) {
    return "/dashboard";
  }

  try {
    const base = "http://sessionflow.invalid";
    const url = new URL(value, base);
    const decodedPath = decodeURIComponent(url.pathname);
    if (url.origin !== base || decodedPath.startsWith("//") || decodedPath.includes("\\") || /^\/login\/?$/.test(decodedPath)) {
      return "/dashboard";
    }
    return `${url.pathname}${url.search}${url.hash}`;
  } catch {
    return "/dashboard";
  }
}
