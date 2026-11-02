"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useSession } from "@/features/auth/hooks/use-session";
import { apiFetch, ApiError } from "@/lib/http/api-client";
import { env } from "@/lib/config/env";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";

type ProjectReference = { project_id: string; project_name: string };
type Profile = { email: string; phone: string; birth_date: string; reminder_channel: "none" | "email" | "whatsapp"; reminder_lead_hours: number; reminder_consent: boolean; revision: number; chatgpt_projects?: ProjectReference[] | null };

export function ClientProfileCard({ clientId, name }: { clientId: string; name: string }) {
  const { authenticatedRequest } = useSession();
  const query = useQuery({
    queryKey: ["client-profile", clientId],
    queryFn: () => authenticatedRequest((session) => apiFetch<Profile>({ baseUrl: env.NEXT_PUBLIC_API_URL, path: `/clients/${clientId}/profile`, method: "GET", tenantId: session.tenantId, accessToken: session.accessToken })),
    refetchOnWindowFocus: false,
    staleTime: Infinity,
  });
  return <Card>
    <CardHeader><CardTitle>Perfil de {name}</CardTitle><CardDescription>Datos de contacto y preferencias de avisos de citas.</CardDescription></CardHeader>
    <CardContent>{query.isPending ? <p role="status">Cargando perfil…</p> : query.isError ? <div role="alert"><p>No se pudo cargar el perfil.</p><Button type="button" variant="outline" onClick={() => void query.refetch()}>Reintentar</Button></div> : <ProfileEditor key={`${clientId}:${query.data.revision}`} clientId={clientId} initial={query.data} />}</CardContent>
  </Card>;
}

function ProfileEditor({ clientId, initial }: { clientId: string; initial: Profile }) {
  const [profile, setProfile] = useState(initial);
  const { authenticatedRequest } = useSession();
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: () => authenticatedRequest((session) => apiFetch<Profile>({ baseUrl: env.NEXT_PUBLIC_API_URL, path: `/clients/${clientId}/profile`, method: "PUT", tenantId: session.tenantId, accessToken: session.accessToken, body: profile })),
    onSuccess: (saved) => { queryClient.setQueryData(["client-profile", clientId], saved); toast.success("Perfil y preferencias guardados."); },
  });
  return <form className="space-y-4" onSubmit={async (event) => { event.preventDefault(); try { await mutation.mutateAsync(); } catch { /* Visible feedback below. */ } }}>
    <div className="grid gap-4 md:grid-cols-3">
      <div className="space-y-2"><Label htmlFor="profile-email">Correo electrónico</Label><Input id="profile-email" type="email" maxLength={254} value={profile.email} onChange={(event) => setProfile({ ...profile, email: event.target.value })} /></div>
      <div className="space-y-2"><Label htmlFor="profile-phone">Teléfono / WhatsApp</Label><Input id="profile-phone" type="tel" placeholder="+5491112345678" pattern="\+[1-9][0-9]{7,14}" value={profile.phone} onChange={(event) => setProfile({ ...profile, phone: event.target.value })} /><p className="text-xs text-muted-foreground">Código de país y número, sin espacios.</p></div>
      <div className="space-y-2"><Label htmlFor="profile-birth">Fecha de nacimiento</Label><Input id="profile-birth" type="date" min="1900-01-01" value={profile.birth_date} onChange={(event) => setProfile({ ...profile, birth_date: event.target.value })} /></div>
    </div>
    <div className="space-y-2"><Label htmlFor="profile-channel">Canal preferido para recordatorios</Label><select id="profile-channel" className="block h-10 w-full rounded-xl border bg-background px-3" value={profile.reminder_channel} onChange={(event) => setProfile({ ...profile, reminder_channel: event.target.value as Profile["reminder_channel"] })}><option value="none">Sin recordatorios</option><option value="email">Correo electrónico</option><option value="whatsapp">WhatsApp</option></select></div>
    <p className="text-sm">Anticipación: 24 horas antes de la cita.</p>
    <label className="flex items-start gap-2 text-sm"><input type="checkbox" checked={profile.reminder_consent} onChange={(event) => setProfile({ ...profile, reminder_consent: event.target.checked })} />El paciente autorizó recibir recordatorios por el canal seleccionado.</label>
    <p className="rounded-xl bg-muted p-3 text-sm">Envíos pendientes de configurar: puedes guardar la preferencia; el envío automático por correo o WhatsApp todavía no está activado.</p>
    <fieldset className="space-y-3 rounded-xl border p-4">
      <legend className="px-2 font-semibold">Proyectos de ChatGPT asociados</legend>
      <p className="text-sm text-muted-foreground">Identifica los proyectos que corresponden a este paciente. La asociación guarda una referencia; la incorporación de informes o conversaciones requiere un paso posterior.</p>
      {(profile.chatgpt_projects ?? []).map((reference,index) => <div className="grid gap-2 rounded-xl border p-3 md:grid-cols-[1fr_2fr_auto]" key={index}>
        <div><Label htmlFor={`project-name-${index}`}>Nombre del proyecto {index+1}</Label><Input id={`project-name-${index}`} required maxLength={200} value={reference.project_name} onChange={(event) => setProfile({...profile,chatgpt_projects:(profile.chatgpt_projects ?? []).map((item,i) => i===index ? {...item,project_name:event.target.value} : item)})} /></div>
        <div><Label htmlFor={`project-id-${index}`}>ID del proyecto {index+1}</Label><Input id={`project-id-${index}`} required pattern="g-p-[a-f0-9]{32}" placeholder="g-p-…" value={reference.project_id} onChange={(event) => setProfile({...profile,chatgpt_projects:(profile.chatgpt_projects ?? []).map((item,i) => i===index ? {...item,project_id:event.target.value} : item)})} /></div>
        <Button type="button" variant="outline" onClick={() => setProfile({...profile,chatgpt_projects:(profile.chatgpt_projects ?? []).filter((_,i) => i!==index)})}>Quitar asociación {index+1}</Button>
      </div>)}
      {!(profile.chatgpt_projects?.length) ? <p className="text-sm">Sin proyecto asociado.</p> : null}
      <Button type="button" variant="outline" disabled={(profile.chatgpt_projects?.length ?? 0)>=5} onClick={() => setProfile({...profile,chatgpt_projects:[...(profile.chatgpt_projects ?? []),{project_id:"",project_name:""}]})}>Agregar referencia de proyecto</Button>
    </fieldset>
    {mutation.error ? <p role="alert" className="text-sm text-destructive">{mutation.error instanceof ApiError ? mutation.error.status === 409 ? "El perfil cambió en otra ventana. Recarga antes de guardar." : mutation.error.message : "No se pudo guardar el perfil."}</p> : null}
    <Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? "Guardando…" : "Guardar perfil y preferencias"}</Button>
  </form>;
}
