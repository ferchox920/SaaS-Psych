import "server-only";

export const demoCredentials = process.env.NEXT_PUBLIC_DEMO_MODE === "true"
  ? {
      tenantId: "11111111-1111-1111-1111-111111111111",
      email: "owner@tenant-a.local",
      password: "ChangeMe123!",
    }
  : null;
