"use client";

import { Building2, LogOut } from "lucide-react";
import { useRouter } from "next/navigation";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useSession } from "@/features/auth/hooks/use-session";
import { env } from "@/lib/config/env";

export function AppTopbar() {
  const router = useRouter();
  const { session, signOut } = useSession();

  return (
    <header className="flex flex-col gap-4 rounded-[28px] border border-white/60 bg-white/75 px-5 py-4 shadow-[0_10px_50px_-36px_rgba(29,41,57,0.45)] backdrop-blur sm:flex-row sm:items-center sm:justify-between">
      <div className="space-y-1">
        <p className="text-xs font-medium uppercase tracking-[0.24em] text-muted-foreground">
          Espacio profesional
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-xl font-semibold text-foreground">SessionFlow</h1>
          {env.NEXT_PUBLIC_DEMO_MODE ? <Badge variant="outline" className="border-red-300 bg-red-50 text-red-900">Demo simulada · datos ficticios · no es inferencia clínica</Badge> : null}
          <Badge variant="secondary">{session?.role === "owner" ? "Propietario" : session?.role === "admin" ? "Administración" : "Profesional"}</Badge>
          <details className="relative text-xs text-muted-foreground">
            <summary className="flex cursor-pointer items-center gap-1 rounded-xl border px-2 py-1 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"><Building2 className="size-3.5" /> Detalle de cuenta</summary>
            <div className="mt-1 max-w-full break-all rounded-xl border bg-white p-2 sm:absolute sm:z-10 sm:min-w-64 sm:shadow-lg">Tenant: {session?.tenantId}<br />Actor: {session?.userId}</div>
          </details>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          onClick={() => {
            void signOut();
            router.replace("/login");
          }}
        >
          <LogOut className="size-4" />
          Cerrar sesion
        </Button>
      </div>
    </header>
  );
}
