import { SessionNotesView } from "@/features/session-notes/components/session-notes-view";

export default async function SessionNotesPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const initialAppointmentId =
    typeof params.appointmentId === "string" ? params.appointmentId : undefined;

  return <SessionNotesView initialAppointmentId={initialAppointmentId} />;
}
