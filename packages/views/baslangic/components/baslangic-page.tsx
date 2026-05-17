"use client";

import { useHospitalStore } from "@medigt/core/hospital";
import { DashboardLayout, PageHeader } from "../../layout";
import { OnboardingChecklist } from "./onboarding-checklist";

export function BaslangicPage() {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title={`Hoş Geldiniz · ${org?.name ?? ""}`}
          subtitle={branch?.name ?? ""}
        />

        <OnboardingChecklist />

        <p className="text-sm text-muted-foreground">
          Rol bazlı landing (doktor, hemşire, kasiyer, admin) görünümleri sonraki
          turda eklenecek. Şimdilik üst menüden modüllere geçebilirsiniz.
        </p>
      </div>
    </DashboardLayout>
  );
}
