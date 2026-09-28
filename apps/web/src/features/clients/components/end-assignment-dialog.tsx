"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";

type Props = {
  professional: string;
  pending: boolean;
  error?: string;
  onCancel: () => void;
  onConfirm: (reason: string) => Promise<void>;
};

export function EndAssignmentDialog({ professional, pending, error, onCancel, onConfirm }: Props) {
  const dialog = useRef<HTMLDialogElement>(null);
  const reasonField = useRef<HTMLTextAreaElement>(null);
  const [reason, setReason] = useState("");
  const [validation, setValidation] = useState("");

  useEffect(() => {
    const current = dialog.current;
    current?.showModal();
    reasonField.current?.focus();
    return () => current?.close();
  }, []);

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (pending) return;
    const trimmed = reason.trim();
    if (!trimmed) {
      setValidation("Escribe un motivo para finalizar la asignación.");
      reasonField.current?.focus();
      return;
    }
    setValidation("");
    void onConfirm(trimmed).catch(() => {
      // The parent renders the API error while keeping the dialog open.
    });
  };

  return (
    <dialog
      ref={dialog}
      aria-labelledby="end-assignment-title"
      aria-describedby="end-assignment-description"
      onCancel={(event) => { event.preventDefault(); if (!pending) onCancel(); }}
      className="m-auto w-[calc(100%-2rem)] max-w-lg rounded-3xl border border-border bg-white p-6 text-foreground shadow-xl backdrop:bg-slate-950/60"
    >
      <form onSubmit={submit} className="space-y-4">
        <h3 id="end-assignment-title" className="text-xl font-semibold">Finalizar asignación clínica</h3>
        <p id="end-assignment-description" className="text-sm text-muted-foreground">
          Finalizarás el acceso clínico de {professional}. Indica el motivo para dejar constancia en la auditoría.
        </p>
        <label htmlFor="end-assignment-reason" className="block space-y-2 text-sm font-medium">
          Motivo de finalización
          <textarea
            ref={reasonField}
            id="end-assignment-reason"
            value={reason}
            onChange={(event) => { setReason(event.target.value); setValidation(""); }}
            disabled={pending}
            aria-invalid={Boolean(validation)}
            aria-describedby={validation ? "end-assignment-error" : undefined}
            rows={3}
            className="block w-full resize-y rounded-2xl border border-input p-3 text-sm outline-none focus-visible:ring-4 focus-visible:ring-ring"
          />
        </label>
        {(validation || error) && <p id="end-assignment-error" role="alert" className="text-sm text-red-700">{validation || error}</p>}
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="outline" disabled={pending} onClick={onCancel}>Cancelar</Button>
          <Button type="submit" disabled={pending}>{pending ? "Finalizando…" : "Confirmar finalización"}</Button>
        </div>
      </form>
    </dialog>
  );
}
