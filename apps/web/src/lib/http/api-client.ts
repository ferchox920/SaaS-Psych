type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

type ApiFetchOptions = {
  baseUrl: string;
  path: string;
  method: HttpMethod;
  tenantId?: string;
  accessToken?: string;
  body?: unknown;
  headers?: HeadersInit;
  signal?: AbortSignal;
};

type ErrorEnvelope = {
  error?: {
    code?: string;
    message?: string;
    details?: unknown;
  };
};

export class ApiError extends Error {
  status: number;
  code: string;
  details?: unknown;

  constructor(status: number, code: string, message: string, details?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export async function apiFetch<T>({
  baseUrl,
  path,
  method,
  tenantId,
  accessToken,
  body,
  headers,
  signal,
}: ApiFetchOptions): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, {
    method,
	credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(tenantId ? { "X-Tenant-ID": tenantId } : {}),
      ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      ...headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal,
  });

  if (!response.ok) {
    let payload: ErrorEnvelope | null = null;

    try {
      payload = (await response.json()) as ErrorEnvelope;
    } catch {
      payload = null;
    }

    throw new ApiError(
      response.status,
      payload?.error?.code || "http_error",
      payload?.error?.message || "Request failed",
      payload?.error?.details,
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}
