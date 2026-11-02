import { PatientMaterials } from "@/features/patient-materials/workspace";

export default async function Page({ params }: { params: Promise<{ clientId: string }> }) {
  const { clientId } = await params;
  return <PatientMaterials clientId={clientId} />;
}
