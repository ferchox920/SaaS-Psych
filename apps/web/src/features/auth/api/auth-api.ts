import type { AuthSession, LoginInput, LoginResponse, MeResponse } from "@/features/auth/lib/auth-types";
import { env } from "@/lib/config/env";
import { apiFetch } from "@/lib/http/api-client";

export async function login(input: LoginInput): Promise<LoginResponse> {
  const response = await apiFetch<{ access_token: string; token_type: string; expires_in: number }>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: "/auth/login",
    tenantId: input.tenantId,
    body: {
      email: input.email,
      password: input.password,
    },
  });
  return { accessToken: response.access_token, tokenType: response.token_type, expiresIn: response.expires_in };
}

export async function refreshTokens(tenantId: string): Promise<LoginResponse> {
  const response = await apiFetch<{ access_token: string; token_type: string; expires_in: number }>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: "/auth/refresh",
    tenantId,
  });
  return { accessToken: response.access_token, tokenType: response.token_type, expiresIn: response.expires_in };
}

export async function logout(tenantId: string): Promise<void> {
  await apiFetch<void>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "POST",
    path: "/auth/logout",
    tenantId,
  });
}

export async function fetchMe(session: Pick<AuthSession, "tenantId" | "accessToken">): Promise<MeResponse> {
  return apiFetch<MeResponse>({
    baseUrl: env.NEXT_PUBLIC_API_URL,
    method: "GET",
    path: "/auth/me",
    tenantId: session.tenantId,
    accessToken: session.accessToken,
  });
}
