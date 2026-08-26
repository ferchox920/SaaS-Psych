import { LoginPageView } from "@/features/auth/components/login-page-view";

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const next = typeof params.next === "string" ? params.next : "/dashboard";

  return <LoginPageView next={next} />;
}
