"use client";

import { ReactNode } from "react";

import { AppTopbar } from "@/components/layout/app-topbar";
import { SidebarNav } from "@/components/layout/sidebar-nav";

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-transparent">
      <div className="mx-auto grid min-h-screen max-w-[1600px] grid-cols-1 gap-6 px-4 py-4 lg:grid-cols-[280px_minmax(0,1fr)] lg:px-6">
        <SidebarNav />
        <div className="flex min-h-[calc(100vh-2rem)] flex-col gap-4">
          <AppTopbar />
          <main className="flex-1 rounded-[28px] border border-white/60 bg-card/95 p-5 shadow-[0_20px_80px_-40px_rgba(47,78,101,0.45)] backdrop-blur md:p-8">
            {children}
          </main>
        </div>
      </div>
    </div>
  );
}
