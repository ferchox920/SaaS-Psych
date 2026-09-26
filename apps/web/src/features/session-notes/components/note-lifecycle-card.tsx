"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useSession } from "@/features/auth/hooks/use-session";
import { addSessionNoteAddendum, listSessionNoteVersions, signSessionNote } from "@/features/session-notes/api/session-notes-api";
import { SessionNote } from "@/types/api";

export function NoteLifecycleCard({ note, canEdit }: { note: SessionNote; canEdit: boolean }) {
  const { authenticatedRequest, session } = useSession();
  const cache = useQueryClient();
  const [showHistory, setShowHistory] = useState(false);
  const [body, setBody] = useState("");
  const [reason, setReason] = useState("");
  const key = ["session-note-versions", session?.tenantId, note.id];
  const history = useQuery({ queryKey: key, enabled: showHistory, queryFn: () => authenticatedRequest((current) => listSessionNoteVersions(current, note.id)), retry: false });
  const refresh = async () => {
    await Promise.all([
      cache.invalidateQueries({ queryKey: ["session-notes", note.appointment_id] }),
      cache.invalidateQueries({ queryKey: key }),
    ]);
  };
  const sign = useMutation({ mutationFn: () => authenticatedRequest((current) => signSessionNote(current, note.id)), onSuccess: refresh });
  const addendum = useMutation({
    mutationFn: () => authenticatedRequest((current) => addSessionNoteAddendum(current, note.id, body.trim(), reason.trim())),
    onSuccess: async () => { setBody(""); setReason(""); await refresh(); },
  });
  return <Card>
    <CardHeader><CardTitle>Firma e historial de nota</CardTitle></CardHeader>
    <CardContent className="space-y-4">
      <p>Estado: {note.status === "signed" ? "firmada" : "borrador"} · Versión {note.current_version} · Firma: {note.signed_at ?? "pendiente"}</p>
      {note.status === "draft" && canEdit ? <Button disabled={sign.isPending} onClick={() => sign.mutate()}>Firmar nota</Button> : null}
      {note.status === "signed" && canEdit ? <div className="space-y-3">
        <p className="text-sm">Una nota firmada no se edita directamente; las aclaraciones se agregan como nuevas versiones.</p>
        <label className="block">Texto de adenda
          <textarea className="block w-full rounded border p-2" value={body} onChange={(event) => setBody(event.target.value)} />
        </label>
        <label className="block">Motivo de adenda
          <input className="block w-full rounded border p-2" value={reason} onChange={(event) => setReason(event.target.value)} />
        </label>
        <Button disabled={addendum.isPending || !body.trim() || !reason.trim()} onClick={() => addendum.mutate()}>Guardar adenda</Button>
      </div> : null}
      {(sign.error || addendum.error || history.error) && <p role="alert">No se pudo confirmar el cambio. Consulta la nota actual antes de reintentar.</p>}
      <Button variant="outline" onClick={() => setShowHistory(!showHistory)}>{showHistory ? "Ocultar historial de versiones" : "Ver historial de versiones"}</Button>
      {showHistory && (history.isPending ? <p role="status">Cargando historial…</p> :
        <ol className="space-y-3">{history.data?.items.map((version) => <li key={version.id} className="rounded border p-3">
          <p className="font-semibold">Versión {version.version} · {version.change_kind}</p>
          <p className="text-sm">{version.created_at} · Actor {version.actor_user_id}</p>
          {version.change_reason && <p>Motivo: {version.change_reason}</p>}
          <p className="whitespace-pre-wrap">{version.body}</p>
        </li>)}</ol>)}
    </CardContent>
  </Card>;
}
