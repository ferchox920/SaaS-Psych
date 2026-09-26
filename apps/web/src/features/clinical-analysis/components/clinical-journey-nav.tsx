import Link from "next/link";

const links = [
  { href: "/clients", label: "Pacientes y revisión clínica" },
  { href: "/clinical-workspace", label: "Análisis local en sesión" },
  { href: "/clinical-formulation", label: "Formulación aprobada" },
  { href: "/clinical-review", label: "Revisión local posterior" },
] as const;

export function ClinicalJourneyNav({ current, clientId }: { current?: string; clientId?: string }) {
  return <nav aria-label="Recorrido clínico" className="flex flex-wrap gap-2 rounded-2xl border bg-white/60 p-3 text-sm">
    {links.map((item) => {
      const href = item.href === "/clients" && clientId ? `/clients/${clientId}/clinical` : item.href;
      return <Link key={item.href} href={href} aria-current={current === item.href ? "page" : undefined} className="rounded-xl border px-3 py-2 text-foreground hover:bg-muted focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 aria-[current=page]:border-primary aria-[current=page]:font-semibold">{item.label}</Link>;
    })}
  </nav>;
}
