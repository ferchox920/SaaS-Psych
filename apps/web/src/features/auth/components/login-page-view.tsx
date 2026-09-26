"use client";

import { ArrowRight, KeyRound, LayoutGrid, Users } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { Badge } from "@/components/ui/badge";
import { LoginForm } from "@/features/auth/components/login-form";
import { useSession } from "@/features/auth/hooks/use-session";

export function LoginPageView({ next, demoIdentity }: { next: string; demoIdentity?: { tenantId: string; email: string } }) {
  const router = useRouter();
  const { clearFeedback, feedback, status } = useSession();

  useEffect(() => {
    if (status === "authenticated") {
      router.replace(next);
    }
  }, [next, router, status]);

  useEffect(() => {
    return () => {
      clearFeedback();
    };
  }, [clearFeedback]);

  return (
    <div className="relative min-h-screen overflow-hidden">
      <div className="mx-auto grid min-h-screen max-w-[1400px] items-center gap-10 px-4 py-10 lg:grid-cols-[1.1fr_520px] lg:px-8">
        <section className="rounded-[36px] border border-white/60 bg-[linear-gradient(135deg,rgba(58,110,165,0.12),rgba(221,245,240,0.9),rgba(255,255,255,0.92))] p-8 shadow-[0_30px_120px_-60px_rgba(29,41,57,0.55)] backdrop-blur md:p-12">
          <Badge variant="outline" className="mb-6">
            SessionFlow
          </Badge>
          {demoIdentity ? <Badge variant="outline" className="mb-6 ml-2 border-red-300 bg-red-50 text-red-900">Demo simulada · solo datos ficticios</Badge> : null}
          <div className="max-w-2xl space-y-6">
            <h1 className="text-4xl font-semibold tracking-tight text-foreground md:text-6xl">
              Tu espacio para organizar sesiones y revisar información clínica.
            </h1>
            <p className="text-lg leading-8 text-muted-foreground">
              Trabaja con pacientes, citas y registros desde una cuenta asignada. Las sugerencias de IA permanecen separadas de las decisiones profesionales.
            </p>
          </div>

          <div className="mt-8 grid gap-4 md:grid-cols-3">
            <Highlight icon={<KeyRound className="size-5" />} title="Acceso por organización" description="Tus datos permanecen dentro de la organización seleccionada." />
            <Highlight icon={<LayoutGrid className="size-5" />} title="Trabajo organizado" description="Citas, sesiones e informes conectados en un mismo recorrido." />
            <Highlight icon={<Users className="size-5" />} title="Revisión humana" description="La información clínica requiere decisión profesional explícita." />
          </div>

          {demoIdentity ? <div className="mt-8 flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
            <span>Tenant demo:</span>
            <code className="rounded-full bg-white px-3 py-1 font-mono text-foreground">
              {demoIdentity.tenantId}
            </code>
            <span>Owner:</span>
            <code className="rounded-full bg-white px-3 py-1 font-mono text-foreground">{demoIdentity.email}</code>
          </div> : null}

          {feedback ? (
            <div className="mt-6 rounded-[28px] border border-border/70 bg-white/75 px-5 py-4 text-sm text-foreground">
              {feedback.message}
            </div>
          ) : null}

          <div className="mt-10">
            <Link
              href="/dashboard"
              className="inline-flex items-center gap-2 text-sm font-medium text-muted-foreground transition hover:text-foreground"
            >
              Ir al panel
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </section>

        <LoginForm next={next} />
      </div>
    </div>
  );
}

function Highlight({
  icon,
  title,
  description,
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
}) {
  return (
    <div className="rounded-[28px] border border-white/70 bg-white/80 p-5">
      <div className="mb-3 inline-flex rounded-2xl bg-secondary p-3 text-secondary-foreground">{icon}</div>
      <h2 className="font-semibold">{title}</h2>
      <p className="mt-2 text-sm text-muted-foreground">{description}</p>
    </div>
  );
}
