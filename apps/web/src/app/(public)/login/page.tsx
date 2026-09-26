import { LoginPageView } from "@/features/auth/components/login-page-view";
import { demoCredentials } from "@/features/auth/lib/demo-credentials";
import { safeLoginNext } from "@/features/auth/lib/login-next";

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const next = safeLoginNext(params.next);

  return <LoginPageView next={next} demoIdentity={demoCredentials ? { tenantId: demoCredentials.tenantId, email: demoCredentials.email } : undefined} />;
}
