"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Building2,
  CheckCircle2,
  ChevronRight,
  Circle,
  Database,
  ListChecks,
  Sparkles,
  Stethoscope,
  Warehouse,
  Wallet,
  UserPlus,
  Users,
} from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  onboardingKeys,
  onboardingStatusOptions,
  seedOnboardingDefaults,
} from "@medigt/core/onboarding";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import { PrimaryButton } from "../../common/form-fields";

// Hospital-onboarding checklist for new orgs. Shows progress, lets the
// admin one-click seed sensible defaults (1 SGK + 1 CEPTEN kurum, 10 temel
// hizmet, 1 Eczane Deposu), and routes to the relevant create pages for
// the remaining items (doctor, patient, kasa).
//
// Auto-hides itself once every required item is satisfied.
export function OnboardingChecklist() {
  const qc = useQueryClient();
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const nav = useNavigation();
  const orgId = org?.id ?? "";
  const branchId = branch?.id ?? "";
  const status = useQuery(onboardingStatusOptions(orgId, branchId));

  const seed = useMutation({
    mutationFn: seedOnboardingDefaults,
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: onboardingKeys.status(orgId, branchId) });
    },
  });

  if (!status.data) return null;
  const s = status.data;

  const branchPaths = paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "");

  const items: ChecklistItem[] = [
    {
      key: "institutions",
      label: "Kurum tanımları",
      icon: Building2,
      detail: `${s.institutions_count} kurum tanımlı`,
      done: s.institutions_count > 0,
      href: branchPaths.kurum.list(),
    },
    {
      key: "services",
      label: "Hizmet kataloğu",
      icon: ListChecks,
      detail: `${s.services_count} hizmet`,
      done: s.services_count > 0,
      href: branchPaths.hizmet.list(),
    },
    {
      key: "warehouses",
      label: "Depo (eczane / stok)",
      icon: Warehouse,
      detail: `${s.warehouses_count} depo`,
      done: s.warehouses_count > 0,
      href: branchPaths.depo.root(),
    },
    {
      key: "doctors",
      label: "Doktor kaydı",
      icon: Stethoscope,
      detail: `${s.doctors_count} doktor`,
      done: s.doctors_count > 0,
      href: branchPaths.doktor.list(),
    },
    {
      key: "patients",
      label: "İlk hasta kaydı",
      icon: UserPlus,
      detail: `${s.patients_count} hasta`,
      done: s.patients_count > 0,
      href: branchPaths.hasta.list(),
    },
    {
      key: "kasa",
      label: "Vezne kasa oturumu",
      icon: Wallet,
      detail: `${s.cash_registers_count} oturum`,
      done: s.cash_registers_count > 0,
      href: branchPaths.vezne.root(),
    },
    {
      key: "specs",
      label: "Branşlar (sistem)",
      icon: Users,
      detail: `${s.specializations_count} branş erişilebilir`,
      done: s.specializations_count > 0,
      href: branchPaths.brans(),
      systemSeeded: true,
    },
    {
      key: "icd10",
      label: "ICD-10 (sistem)",
      icon: Database,
      detail: `${s.icd10_system_count} kod erişilebilir`,
      done: s.icd10_system_count > 0,
      href: branchPaths.icd10(),
      systemSeeded: true,
    },
  ];

  const doneCount = items.filter((i) => i.done).length;
  // Hide entirely once everything's set up — admin can still access
  // each section from the sidebar.
  if (doneCount === items.length) return null;

  return (
    <section className="surface-card p-4 grid-bg">
      <header className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="eyebrow">Kurulum</div>
          <h2 className="mt-1 heading-lg">İlk Kurulum Adımları</h2>
          <p className="lede mt-1 text-sm">
            Hastanenizi üretime hazırlamak için temel master data + kasa adımları
            <span className="ml-1 chip-accent">{doneCount}/{items.length} tamam</span>
          </p>
        </div>
        <PrimaryButton
          type="button"
          onClick={() => seed.mutate()}
          disabled={seed.isPending}
        >
          <span className="inline-flex items-center gap-1">
            <Sparkles className="h-4 w-4" /> {seed.isPending ? "Ekleniyor..." : "Varsayılanları Yükle"}
          </span>
        </PrimaryButton>
      </header>

      <div className={"bar mb-4 " + (doneCount === items.length ? "bar-success" : "")}>
        <span style={{ width: `${(doneCount / items.length) * 100}%` }} />
      </div>

      {seed.data && (
        <div className="mb-3 rounded-md border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200">
          ✓ {seed.data.institutions_added} kurum · {seed.data.services_added} hizmet ·{" "}
          {seed.data.warehouses_added} depo eklendi (idempotent — tekrar tıklamak güvenli).
        </div>
      )}

      <ul className="space-y-1">
        {items.map((it) => (
          <li key={it.key}>
            <button
              type="button"
              onClick={() => nav.push(it.href)}
              className={
                "flex w-full items-center gap-3 rounded-md border border-transparent px-3 py-2 text-left hover:border-border hover:bg-muted " +
                (it.done ? "text-muted-foreground" : "")
              }
            >
              {it.done ? (
                <CheckCircle2 className="h-5 w-5 shrink-0 text-emerald-600 dark:text-emerald-400" />
              ) : (
                <Circle className="h-5 w-5 shrink-0 text-muted-foreground" />
              )}
              <it.icon className="h-4 w-4 shrink-0 text-muted-foreground" />
              <div className="min-w-0 flex-1">
                <div className={"text-sm " + (it.done ? "" : "font-medium")}>
                  {it.label}
                  {it.systemSeeded && (
                    <span className="ml-2 rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                      sistem
                    </span>
                  )}
                </div>
                <div className="text-xs text-muted-foreground">{it.detail}</div>
              </div>
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}

type ChecklistItem = {
  key: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  detail: string;
  done: boolean;
  href: string;
  systemSeeded?: boolean;
};
