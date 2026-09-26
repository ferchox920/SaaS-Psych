import { SessionWorkspace } from "@/features/session-workspace/workspace";

export default async function Page({
  params,
  searchParams,
}: {
  params: Promise<{ clientId: string }>;
  searchParams: Promise<{ appointmentId?: string | string[] }>;
}) {
  const { clientId } = await params;
  const query = await searchParams;
  const appointmentId = typeof query.appointmentId === "string" ? query.appointmentId : undefined;
  return <SessionWorkspace clientId={clientId} appointmentId={appointmentId} />;
}
