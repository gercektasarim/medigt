"use client";

import { useEffect } from "react";
import { useAuthStore } from "@medigt/core/auth";
import { useHospitalStore } from "@medigt/core/hospital";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";

export function RootDispatcher() {
  const nav = useNavigation();
  const isBootstrapped = useAuthStore((s) => s.isBootstrapped);
  const user = useAuthStore((s) => s.user);
  const orgs = useHospitalStore((s) => s.availableOrgs);
  const branches = useHospitalStore((s) => s.availableBranches);

  useEffect(() => {
    if (!isBootstrapped) return;
    if (!user) {
      nav.replace(paths.login());
      return;
    }
    if (orgs.length === 0) {
      nav.replace(paths.onboarding());
      return;
    }
    const org = orgs[0]!;
    const branch = branches.find((b) => b.organization_id === org.id) ?? branches[0];
    if (!branch) {
      nav.replace(paths.onboarding());
      return;
    }
    nav.replace(paths.hospital(org.slug).branch(branch.slug).baslangic());
  }, [isBootstrapped, user, orgs, branches, nav]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="text-sm text-muted-foreground">Yükleniyor...</div>
    </div>
  );
}
