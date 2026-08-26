"use client";

import { createContext, useCallback, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";

import { fetchMe, login, logout, refreshTokens } from "@/features/auth/api/auth-api";
import { getAuthErrorMessage } from "@/features/auth/lib/auth-error-messages";
import { canAccessPath } from "@/features/auth/lib/auth-route-access";
import { getRefreshDelay, isSessionExpired } from "@/features/auth/lib/auth-session";
import {
  AuthSession,
  LoginFormValues,
  LoginResponse,
  SignOutReason,
  SessionStatus,
} from "@/features/auth/lib/auth-types";
import { clearStoredTenantId, getStoredTenantId, setStoredTenantId } from "@/features/auth/lib/auth-storage";
import { loginSchema } from "@/features/auth/schemas/login-schema";
import { ApiError } from "@/lib/http/api-client";

type SessionFeedback = {
  kind: "info" | "error" | "success";
  message: string;
} | null;

type AuthContextValue = {
  status: SessionStatus;
  session: AuthSession | null;
  isRefreshing: boolean;
  isSigningOut: boolean;
  feedback: SessionFeedback;
  form: ReturnType<typeof useForm<LoginFormValues>>;
  signIn: (values: LoginFormValues) => Promise<AuthSession>;
  signOut: (reason?: SignOutReason) => Promise<void>;
  authenticatedRequest: <T>(request: (session: AuthSession) => Promise<T>) => Promise<T>;
  canAccess: (pathname: string) => boolean;
  clearFeedback: () => void;
};

export const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: Readonly<{ children: React.ReactNode }>) {
  const queryClient = useQueryClient();
  const [session, setSession] = useState<AuthSession | null>(null);
  const [status, setStatus] = useState<SessionStatus>("loading");
  const [feedback, setFeedback] = useState<SessionFeedback>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isSigningOut, setIsSigningOut] = useState(false);
  const refreshInFlightRef = useRef<Promise<AuthSession> | null>(null);
  const bootstrappedRef = useRef(false);

  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      tenantId: "11111111-1111-1111-1111-111111111111",
      email: "owner@tenant-a.local",
      password: "ChangeMe123!",
    },
  });

  const persistSession = async (
    tenantId: string,
    tokens: LoginResponse,
    previousRole?: AuthSession["role"],
  ): Promise<AuthSession> => {
    const me = await fetchMe({
      tenantId,
      accessToken: tokens.accessToken,
    });

    const nextSession: AuthSession = {
      tenantId,
      accessToken: tokens.accessToken,
      tokenType: tokens.tokenType,
      expiresAt: new Date(Date.now() + tokens.expiresIn * 1000).toISOString(),
      userId: me.user_id,
      role: me.role ?? previousRole ?? "member",
    };

    setStoredTenantId(tenantId);
    setSession(nextSession);
    setStatus("authenticated");
    setFeedback(null);

    return nextSession;
  };

  const signIn = async (values: LoginFormValues) => {
    try {
      setFeedback(null);
      const tokens = await login(values);
      const nextSession = await persistSession(values.tenantId, tokens);
      toast.success("Sesion iniciada.");
      return nextSession;
    } catch (error) {
      const message = getAuthErrorMessage(error);
      setFeedback({ kind: "error", message });
      throw new Error(message);
    }
  };

  const clearLocalSession = useCallback((nextStatus: SessionStatus, nextFeedback: SessionFeedback) => {
    clearStoredTenantId();
    setSession(null);
    setStatus(nextStatus);
    setFeedback(nextFeedback);
    queryClient.clear();
  }, [queryClient]);

  const signOut = useCallback(async (reason: SignOutReason = "user") => {
    setIsSigningOut(true);

    if (session) {
      try {
        await logout(session.tenantId);
      } catch {
        // Best effort logout. Local state still must be cleared.
      }
    }

    try {
      if (reason === "user") {
        toast.success("Sesion cerrada.");
        clearLocalSession("anonymous", {
          kind: "success",
          message: "Sesion cerrada correctamente.",
        });
        return;
      }

      if (reason === "unauthorized") {
        toast.error("No tienes permisos para esa seccion.");
        clearLocalSession("anonymous", {
          kind: "error",
          message: "Tu rol no tiene permisos para acceder a esa seccion.",
        });
        return;
      }

      toast.error("Tu sesion vencio. Inicia sesion nuevamente.");
      clearLocalSession("expired", {
        kind: "info",
        message: "Tu sesion expiro. Vuelve a iniciar sesion.",
      });
    } finally {
      setIsSigningOut(false);
    }
  }, [clearLocalSession, session]);

  const refreshSession = useCallback(async (currentSession: AuthSession) => {
    if (refreshInFlightRef.current) {
      return refreshInFlightRef.current;
    }

    const refreshPromise = (async () => {
      setIsRefreshing(true);
      setStatus("refreshing");

      try {
        const refreshed = await refreshTokens(currentSession.tenantId);
        return await persistSession(currentSession.tenantId, refreshed, currentSession.role);
      } catch {
        await signOut("expired");
        throw new Error("Session expired");
      } finally {
        refreshInFlightRef.current = null;
        setIsRefreshing(false);
      }
    })();

    refreshInFlightRef.current = refreshPromise;
    return refreshPromise;
  }, [signOut]);

  useEffect(() => {
    if (bootstrappedRef.current) {
      return;
    }
    bootstrappedRef.current = true;
    const tenantId = getStoredTenantId();
    if (!tenantId) {
      setStatus("anonymous");
      return;
    }

    void refreshTokens(tenantId)
      .then((tokens) => persistSession(tenantId, tokens))
      .catch(() => {
        clearStoredTenantId();
        setSession(null);
        setStatus("anonymous");
      });
  }, []);

  useEffect(() => {
    if (!session) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      void refreshSession(session);
    }, getRefreshDelay(session));

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [refreshSession, session]);

  const authenticatedRequest = async <T,>(request: (currentSession: AuthSession) => Promise<T>): Promise<T> => {
    if (!session) {
      throw new Error("No active session");
    }

    let activeSession = session;

    if (isSessionExpired(activeSession)) {
      activeSession = await refreshSession(activeSession);
    }

    try {
      return await request(activeSession);
    } catch (error) {
      if (!(error instanceof ApiError) || error.status !== 401) {
        throw error;
      }

      const nextSession = await refreshSession(activeSession);
      return request(nextSession);
    }
  };

  const value: AuthContextValue = {
    status,
    session,
    isRefreshing,
    isSigningOut,
    feedback,
    form,
    signIn,
    signOut,
    authenticatedRequest,
    canAccess: (pathname) => canAccessPath(pathname, session?.role),
    clearFeedback: () => setFeedback(null),
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
