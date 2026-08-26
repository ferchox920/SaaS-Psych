"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { toast } from "sonner";

import { AppShell } from "@/components/layout/app-shell";
import { useSession } from "@/features/auth/hooks/use-session";

export default function ProtectedLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const pathname = usePathname();
  const router = useRouter();
  const { canAccess, status } = useSession();

  useEffect(() => {
    if (status === "anonymous" || status === "expired") {
      router.replace(`/login?next=${encodeURIComponent(pathname)}`);
    }
  }, [pathname, router, status]);

  useEffect(() => {
    if (status === "authenticated" && !canAccess(pathname)) {
      toast.error("No tienes permisos para esa seccion.");
      router.replace("/dashboard");
    }
  }, [canAccess, pathname, router, status]);

  if (status === "loading" || status === "refreshing") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="rounded-full border border-border bg-card px-4 py-2 text-sm text-muted-foreground shadow-sm">
          {status === "refreshing" ? "Renovando sesion..." : "Inicializando SessionFlow..."}
        </div>
      </div>
    );
  }

  if (status !== "authenticated" || !canAccess(pathname)) {
    return null;
  }

  return <AppShell>{children}</AppShell>;
}
