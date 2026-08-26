"use client";

import { useMutation } from "@tanstack/react-query";
import { LoaderCircle, LockKeyhole, ShieldCheck } from "lucide-react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useSession } from "@/features/auth/hooks/use-session";
import { loginSchema } from "@/features/auth/schemas/login-schema";

export function LoginForm({ next }: { next: string }) {
  const router = useRouter();
  const { clearFeedback, feedback, form, isRefreshing, isSigningOut, signIn, status } = useSession();

  const mutation = useMutation({
    mutationFn: signIn,
    onSuccess: () => {
      router.replace(next);
    },
  });

  const onSubmit = form.handleSubmit(async (values) => {
    clearFeedback();
    const payload = loginSchema.parse(values);
    await mutation.mutateAsync(payload);
  });

  return (
    <Card className="border-white/70 bg-white/85 shadow-[0_24px_80px_-42px_rgba(29,41,57,0.45)] backdrop-blur">
      <CardHeader className="space-y-3">
        <div className="inline-flex w-fit items-center gap-2 rounded-full bg-secondary px-3 py-1 text-xs font-semibold uppercase tracking-[0.24em] text-secondary-foreground">
          <ShieldCheck className="size-3.5" />
          Access portal
        </div>
        <CardTitle className="text-3xl">Ingresar al workspace</CardTitle>
        <CardDescription>
          El tenant viaja por header, asi que el login necesita el tenant UUID desde el inicio.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={onSubmit}>
          <div className="space-y-2">
            <Label htmlFor="tenantId">Tenant ID</Label>
            <Input id="tenantId" placeholder="11111111-1111-1111-1111-111111111111" {...form.register("tenantId")} />
            <FieldError message={form.formState.errors.tenantId?.message} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input id="email" type="email" placeholder="owner@tenant-a.local" {...form.register("email")} />
            <FieldError message={form.formState.errors.email?.message} />
          </div>

          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input id="password" type="password" placeholder="ChangeMe123!" {...form.register("password")} />
            <FieldError message={form.formState.errors.password?.message} />
          </div>

          {feedback ? (
            <div className="rounded-2xl border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {feedback.message}
            </div>
          ) : null}

          <Button className="w-full" type="submit" disabled={mutation.isPending || isRefreshing || isSigningOut}>
            {mutation.isPending ? <LoaderCircle className="size-4 animate-spin" /> : <LockKeyhole className="size-4" />}
            {status === "refreshing" ? "Renovando sesion..." : "Entrar"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function FieldError({ message }: { message?: string }) {
  if (!message) {
    return null;
  }

  return <p className="text-sm text-destructive">{message}</p>;
}
