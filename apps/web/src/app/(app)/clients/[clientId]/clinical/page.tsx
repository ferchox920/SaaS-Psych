import { ClinicalReviewWorkspace } from "@/features/clinical-review-workspace/workspace";
export default async function Page({
  params,
}: {
  params: Promise<{ clientId: string }>;
}) {
  const { clientId } = await params;
  return <ClinicalReviewWorkspace client={clientId} />;
}
