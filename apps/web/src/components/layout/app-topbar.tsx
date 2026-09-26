"use client";

import { Bell, Building2, LogOut } from "lucide-react";
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
          <Badge variant="secondary">{session?.role ?? "member"}</Badge>
          <Badge variant="outline" className="gap-1">
            <Building2 className="size-3.5" />
            {session?.tenantId}
          </Badge>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <Button variant="ghost" size="icon" aria-label="Notifications">
          <Bell className="size-4" />
        </Button>
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
