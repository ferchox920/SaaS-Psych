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

export function AuthProvider({ children, demoCredentials }: Readonly<{ children: React.ReactNode; demoCredentials?: LoginFormValues }>) {
  const queryClient = useQueryClient();
  const [session, setSession] = useState<AuthSession | null>(null);
  const [status, setStatus] = useState<SessionStatus>("loading");
  const [feedback, setFeedback] = useState<SessionFeedback>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isSigningOut, setIsSigningOut] = useState(false);
  const refreshInFlightRef = useRef<Promise<AuthSession> | null>(null);
  const bootstrappedRef = useRef(false);
  const authGenerationRef = useRef(0);
  const sessionRef = useRef<AuthSession | null>(null);

  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: demoCredentials ?? { tenantId: "", email: "", password: "" },
  });

  const persistSession = async (
    tenantId: string,
    tokens: LoginResponse,
    generation: number,
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

    if (generation !== authGenerationRef.current) {
      throw new Error("Session changed");
    }

    setStoredTenantId(tenantId);
    sessionRef.current = nextSession;
    setSession(nextSession);
    setStatus("authenticated");
    setFeedback(null);

    return nextSession;
  };

  const signIn = async (values: LoginFormValues) => {
    const generation = ++authGenerationRef.current;
    try {
      setFeedback(null);
      const tokens = await login(values);
      const nextSession = await persistSession(values.tenantId, tokens, generation);
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
    sessionRef.current = null;
    setSession(null);
    setStatus(nextStatus);
    setFeedback(nextFeedback);
    queryClient.clear();
  }, [queryClient]);

  const signOut = useCallback(async (reason: SignOutReason = "user") => {
    ++authGenerationRef.current;
    refreshInFlightRef.current = null;
    setIsSigningOut(true);

    const currentSession = session;
    if (reason === "user") {
      toast.success("Sesion cerrada.");
      clearLocalSession("anonymous", { kind: "success", message: "Sesion cerrada correctamente." });
    } else if (reason === "unauthorized") {
      toast.error("No tienes permisos para esa seccion.");
      clearLocalSession("anonymous", { kind: "error", message: "Tu rol no tiene permisos para acceder a esa seccion." });
    } else {
      toast.error("Tu sesion vencio. Inicia sesion nuevamente.");
      clearLocalSession("expired", { kind: "info", message: "Tu sesion expiro. Vuelve a iniciar sesion." });
    }

    if (currentSession) {
      try {
        await logout(currentSession.tenantId);
      } catch {
        // Best effort logout. Local state still must be cleared.
      }
    }

    setIsSigningOut(false);
  }, [clearLocalSession, session]);

  const refreshSession = useCallback(async (currentSession: AuthSession) => {
    if (sessionRef.current !== currentSession) {
      throw new Error("Session changed");
    }
    if (refreshInFlightRef.current) {
      return refreshInFlightRef.current;
    }

    const generation = authGenerationRef.current;
    const refreshPromise = (async () => {
      setIsRefreshing(true);
      setStatus("refreshing");

      try {
        const refreshed = await refreshTokens(currentSession.tenantId);
        return await persistSession(currentSession.tenantId, refreshed, generation, currentSession.role);
      } catch {
        if (generation === authGenerationRef.current) {
          await signOut("expired");
        }
        throw new Error("Session expired");
      } finally {
        if (generation === authGenerationRef.current) {
          refreshInFlightRef.current = null;
          setIsRefreshing(false);
        }
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
      const generation = authGenerationRef.current;
      queueMicrotask(() => {
        if (generation === authGenerationRef.current) {
          setStatus("anonymous");
        }
      });
      return;
    }

    const generation = authGenerationRef.current;
    void refreshTokens(tenantId)
      .then((tokens) => persistSession(tenantId, tokens, generation))
      .catch(() => {
        if (generation === authGenerationRef.current) {
          clearStoredTenantId();
          sessionRef.current = null;
          setSession(null);
          setStatus("anonymous");
        }
      });
  }, []);

  useEffect(() => {
    if (!session) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      void refreshSession(session).catch(() => {
        // Expiration already updates auth state; timer failures are not unhandled promises.
      });
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

      if (sessionRef.current !== activeSession) {
        throw new Error("Session changed");
      }

      const nextSession = await refreshSession(activeSession);
      if (sessionRef.current !== nextSession) {
        throw new Error("Session changed");
      }
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
