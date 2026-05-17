"use client";

import { useEffect, type ReactNode } from "react";
import { useAuthStore } from "@medigt/core/auth";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";

export function DashboardGuard({ children }: { children: ReactNode }) {
  const user = useAuthStore((s) => s.user);
  const isBootstrapped = useAuthStore((s) => s.isBootstrapped);
  const nav = useNavigation();

  useEffect(() => {
    if (isBootstrapped && !user) {
      nav.replace(paths.login());
    }
  }, [user, isBootstrapped, nav]);

  if (!isBootstrapped || !user) return null;
  return <>{children}</>;
}
