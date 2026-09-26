"use client";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Fields,
  ReadError,
  useClinicalRead,
} from "../clinical-review-workspace/shared";
import type { ExternalRecord } from "./model";
export function ExternalProvenance({
  client,
  id,
}: {
  client: string;
  id?: string;
}) {
  const [open, setOpen] = useState(false);
  const q = useClinicalRead<ExternalRecord>(
    client,
    `/clients/${client}/project-proposals/${id}`,
    open && !!id,
  );
  if (!id) return null;
  return (
    <section className="space-y-2">
      <Button
        variant="outline"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
      >
        Inspeccionar provenance externa
      </Button>
      {open &&
        (q.error ? (
          <ReadError error={q.error} />
        ) : q.data ? (
          <div>
            <p>
              Propuesta externa manual · Datos declarados por terapeuta
              (user-supplied), no verificados por SaaS-Psi. No es un
              ClinicalAIRun.
            </p>
            <Fields value={q.data.proposal.provenance} />
            <p className="break-all">
              Hash de propuesta: {q.data.content_hash}
            </p>
            <details>
              <summary>Original externo inmutable y recibo de origen</summary>
              <p className="break-all">Export: {q.data.source_export_id}</p>
              <Fields value={q.data.proposal} />
            </details>
          </div>
        ) : (
          <p role="status">Consultando provenance…</p>
        ))}
    </section>
  );
}
