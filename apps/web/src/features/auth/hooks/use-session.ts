"use client";

import { useContext } from "react";

import { AuthContext } from "@/features/auth/lib/auth-context";

export function useSession() {
  const context = useContext(AuthContext);

  if (!context) {
    throw new Error("useSession must be used within AuthProvider");
  }

  return context;
}
