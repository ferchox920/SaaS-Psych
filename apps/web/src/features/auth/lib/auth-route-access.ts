import { UserRole } from "@/features/auth/lib/auth-types";

type RouteRule = {
  prefix: string;
  roles: UserRole[];
};

const routeRules: RouteRule[] = [
  {
    prefix: "/audit",
    roles: ["owner", "admin"],
  },
];

export function getRequiredRolesForPath(pathname: string): UserRole[] | null {
  const match = routeRules.find((rule) => pathname.startsWith(rule.prefix));
  return match?.roles ?? null;
}

export function canAccessPath(pathname: string, role?: UserRole | null) {
  const requiredRoles = getRequiredRolesForPath(pathname);

  if (!requiredRoles) {
    return true;
  }

  if (!role) {
    return false;
  }

  return requiredRoles.includes(role);
}
