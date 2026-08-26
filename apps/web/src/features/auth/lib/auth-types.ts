export type UserRole = "owner" | "admin" | "member";

export type LoginFormValues = {
  tenantId: string;
  email: string;
  password: string;
};

export type LoginInput = LoginFormValues;

export type LoginResponse = {
  accessToken: string;
  tokenType: string;
  expiresIn: number;
};

export type MeResponse = {
  user_id: string;
  tenant_id: string;
  role: UserRole;
};

export type AuthSession = {
  tenantId: string;
  accessToken: string;
  tokenType: string;
  expiresAt: string;
  userId: string;
  role: UserRole;
};

export type SessionStatus = "loading" | "authenticated" | "refreshing" | "anonymous" | "expired";

export type SignOutReason = "user" | "expired" | "unauthorized" | "invalid_session";
