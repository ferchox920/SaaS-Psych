import type { Metadata } from "next";

import { AppProviders } from "@/providers/app-providers";
import { demoCredentials } from "@/features/auth/lib/demo-credentials";
import "./globals.css";

export const metadata: Metadata = {
  title: "SessionFlow",
  description: "Plataforma web para operacion profesional y administracion multi-tenant.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="es">
      <body className="antialiased">
        <AppProviders demoCredentials={demoCredentials ?? undefined}>{children}</AppProviders>
      </body>
    </html>
  );
}
