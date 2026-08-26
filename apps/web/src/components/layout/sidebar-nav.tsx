"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ClipboardList,
	BrainCircuit,
	BookOpenCheck,
	SearchCheck,
  FilePenLine,
  LayoutDashboard,
  ShieldCheck,
  Users,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { useSession } from "@/features/auth/hooks/use-session";
import { navigationItems } from "@/lib/config/navigation";
import { cn } from "@/lib/utils/cn";

const icons = {
  dashboard: LayoutDashboard,
  clients: Users,
  appointments: ClipboardList,
	clinicalWorkspace: BrainCircuit,
	clinicalFormulation: BookOpenCheck,
	clinicalReview: SearchCheck,
  sessionNotes: FilePenLine,
  audit: ShieldCheck,
};

export function SidebarNav() {
  const pathname = usePathname();
  const { session } = useSession();

  return (
    <aside className="rounded-[32px] bg-sidebar px-5 py-6 text-sidebar-foreground shadow-[0_30px_90px_-50px_rgba(16,24,40,0.8)]">
      <div className="space-y-3 border-b border-white/10 pb-6">
        <div className="inline-flex rounded-full bg-white/10 px-3 py-1 text-xs uppercase tracking-[0.24em] text-white/70">
          MVP cockpit
        </div>
        <div>
          <h2 className="text-2xl font-semibold">Professional workspace</h2>
          <p className="mt-2 max-w-xs text-sm text-white/70">
            Base comun para operacion profesional y funciones administrativas dentro del mismo frontend.
          </p>
        </div>
      </div>

      <nav className="mt-6 space-y-2">
        {navigationItems.map((item) => {
          const Icon = icons[item.icon];
          const active = pathname === item.href;
          const disabled = item.requiresAdmin && !["owner", "admin"].includes(session?.role ?? "");

          return (
            <Link
              key={item.href}
              href={disabled ? "#" : item.href}
              aria-disabled={disabled}
              className={cn(
                buttonVariants({ variant: active ? "secondary" : "ghost" }),
                "h-auto w-full justify-start gap-3 rounded-2xl px-4 py-3 text-left",
                active && "bg-white text-sidebar shadow-sm hover:bg-white/95",
                !active && "text-white hover:bg-white/10 hover:text-white",
                disabled && "pointer-events-none opacity-45",
              )}
            >
              <Icon className="size-4 shrink-0" />
              <span className="flex-1">
                <span className="block text-sm font-medium">{item.label}</span>
                <span className={cn("block text-xs", active ? "text-slate-500" : "text-white/60")}>
                  {item.description}
                </span>
              </span>
              {item.requiresAdmin ? <Badge variant="outline">admin</Badge> : null}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
