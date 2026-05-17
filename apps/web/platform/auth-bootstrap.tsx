"use client";

// AuthBootstrap reads tokens from localStorage on mount, calls /api/me to
// validate them and fetch user + accessible hospitals, then flips
// useAuthStore.isBootstrapped so downstream guards and dispatchers can
// make their routing decisions.

import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { meOptions, useAuthStore } from "@medigt/core/auth";
import { useHospitalStore } from "@medigt/core/hospital";

export function AuthBootstrap({ children }: { children: React.ReactNode }) {
  const hydrate = useAuthStore((s) => s.hydrateFromStorage);
  const setBootstrapped = useAuthStore((s) => s.setBootstrapped);
  const setUser = useAuthStore((s) => s.setUser);
  const logout = useAuthStore((s) => s.logout);
  const accessToken = useAuthStore((s) => s.accessToken);
  const isBootstrapped = useAuthStore((s) => s.isBootstrapped);
  const setAvailable = useHospitalStore((s) => s.setAvailable);
  const setOrganization = useHospitalStore((s) => s.setOrganization);
  const setBranch = useHospitalStore((s) => s.setBranch);

  // Hydrate tokens from storage exactly once on first client render.
  useEffect(() => {
    hydrate();
  }, [hydrate]);

  const enabled = !!accessToken;
  const me = useQuery(meOptions(enabled));

  useEffect(() => {
    if (!enabled) {
      // No token — we're done bootstrapping; let dispatcher send to /login.
      setBootstrapped(true);
      return;
    }
    if (me.isError) {
      logout();
      setBootstrapped(true);
      return;
    }
    if (me.data) {
      setUser(me.data.user);
      setAvailable(me.data.organizations, me.data.branches);
      const firstOrg = me.data.organizations[0];
      if (firstOrg) {
        setOrganization(firstOrg);
        const firstBranch = me.data.branches.find((b) => b.organization_id === firstOrg.id);
        if (firstBranch) setBranch(firstBranch);
      }
      setBootstrapped(true);
    }
  }, [enabled, me.data, me.isError, setBootstrapped, setUser, setAvailable, setOrganization, setBranch, logout]);

  if (!isBootstrapped) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="text-sm text-muted-foreground">Yükleniyor...</div>
      </div>
    );
  }
  return <>{children}</>;
}
