"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { meKeys, meOptions } from "@medigt/core/auth";
import { createOrganization, useHospitalStore } from "@medigt/core/hospital";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import { useT } from "@medigt/core/i18n";

type Step = "welcome" | "org" | "branch" | "done";

export function OnboardingWizard() {
  const t = useT();
  const qc = useQueryClient();
  const nav = useNavigation();
  const setAvailable = useHospitalStore((s) => s.setAvailable);
  const setOrganization = useHospitalStore((s) => s.setOrganization);
  const setBranch = useHospitalStore((s) => s.setBranch);

  const [step, setStep] = useState<Step>("welcome");
  const [orgSlug, setOrgSlug] = useState("");
  const [orgName, setOrgName] = useState("");
  const [orgKind, setOrgKind] = useState<"single_hospital" | "hospital_group" | "clinic" | "polyclinic">("single_hospital");
  const [taxId, setTaxId] = useState("");
  const [branchSlug, setBranchSlug] = useState("merkez");
  const [branchName, setBranchName] = useState("Merkez Şube");
  const [branchKind, setBranchKind] = useState<"hospital" | "polyclinic" | "lab" | "imaging_center" | "dialysis_center" | "dental_clinic">("hospital");

  const create = useMutation({
    mutationFn: () =>
      createOrganization({
        slug: orgSlug.trim().toLowerCase(),
        name: orgName.trim(),
        kind: orgKind,
        tax_id: taxId.trim() || undefined,
        initial_branch: {
          slug: branchSlug.trim().toLowerCase(),
          name: branchName.trim(),
          kind: branchKind,
        },
      }),
    onSuccess: async () => {
      // Re-fetch /me so accessible orgs/branches are populated, then
      // pick the newly created tenant.
      qc.removeQueries({ queryKey: meKeys.all() });
      const me = await qc.fetchQuery(meOptions());
      setAvailable(me.organizations, me.branches);
      const justCreated = me.organizations.find((o) => o.slug === orgSlug.trim().toLowerCase());
      const org = justCreated ?? me.organizations[me.organizations.length - 1];
      if (!org) return;
      setOrganization(org);
      const branch = me.branches.find((b) => b.organization_id === org.id);
      if (branch) {
        setBranch(branch);
        nav.replace(paths.hospital(org.slug).branch(branch.slug).baslangic());
      }
    },
  });

  if (step === "welcome") {
    return (
      <Shell title="MediGt'ye hoş geldiniz" subtitle="Birlikte ilk hastanenizi kuralım.">
        <p className="text-sm text-muted-foreground">
          Bu kısa kurulum birkaç adım sürer:
        </p>
        <ol className="ml-4 list-decimal space-y-1 text-sm">
          <li>Hastane (organizasyon) bilgileri</li>
          <li>İlk şube</li>
          <li>Hazır — kontrol paneline geçiş</li>
        </ol>
        <button
          onClick={() => setStep("org")}
          className="mt-4 w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
        >
          Başla
        </button>
      </Shell>
    );
  }

  if (step === "org") {
    return (
      <Shell title="Hastane bilgileri" subtitle="Bu hastane grubunun temel kimliği.">
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            setStep("branch");
          }}
        >
          <Field label="Hastane adı" id="orgName" required>
            <input id="orgName" required value={orgName} onChange={(e) => setOrgName(e.target.value)}
              className={inputClass} placeholder="Örn. Acıbadem Şehir Hastanesi" />
          </Field>

          <Field label="URL kısa adı (slug)" id="orgSlug" required hint="URL'de görünür: /h/<slug>/.... 2-40 karakter, küçük harf/rakam/tire.">
            <input id="orgSlug" required value={orgSlug}
              onChange={(e) => setOrgSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, "").slice(0, 40))}
              className={inputClass} placeholder="acibademsehir" />
          </Field>

          <Field label="Tür" id="orgKind">
            <select id="orgKind" value={orgKind} onChange={(e) => setOrgKind(e.target.value as typeof orgKind)} className={inputClass}>
              <option value="single_hospital">Tek hastane</option>
              <option value="hospital_group">Hastane grubu</option>
              <option value="clinic">Klinik</option>
              <option value="polyclinic">Poliklinik</option>
            </select>
          </Field>

          <Field label="Vergi numarası (opsiyonel)" id="taxId">
            <input id="taxId" value={taxId} onChange={(e) => setTaxId(e.target.value)}
              className={inputClass} placeholder="1234567890" />
          </Field>

          <button type="submit" className="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
            disabled={!orgName.trim() || !orgSlug.trim()}>
            Devam
          </button>
        </form>
      </Shell>
    );
  }

  if (step === "branch") {
    return (
      <Shell title="İlk şubeyi ekleyin" subtitle="Hastanenin fiziksel/operasyonel yeri. Tek şube ile başlayabilirsiniz.">
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate();
          }}
        >
          <Field label="Şube adı" id="branchName" required>
            <input id="branchName" required value={branchName} onChange={(e) => setBranchName(e.target.value)}
              className={inputClass} />
          </Field>

          <Field label="Şube slug" id="branchSlug" required hint="URL'de görünür: /h/<org>/<slug>/...">
            <input id="branchSlug" required value={branchSlug}
              onChange={(e) => setBranchSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, "").slice(0, 40))}
              className={inputClass} />
          </Field>

          <Field label="Şube türü" id="branchKind">
            <select id="branchKind" value={branchKind} onChange={(e) => setBranchKind(e.target.value as typeof branchKind)} className={inputClass}>
              <option value="hospital">Hastane</option>
              <option value="polyclinic">Poliklinik</option>
              <option value="lab">Laboratuvar</option>
              <option value="imaging_center">Görüntüleme merkezi</option>
              <option value="dialysis_center">Diyaliz merkezi</option>
              <option value="dental_clinic">Diş kliniği</option>
            </select>
          </Field>

          {create.isError && (
            <p className="text-sm text-[var(--critical)]">
              Oluşturma başarısız oldu. Slug zaten kullanılıyor olabilir.
            </p>
          )}

          <div className="flex gap-2">
            <button type="button" onClick={() => setStep("org")}
              className="flex-1 rounded-md border border-input bg-background px-4 py-2 text-sm">
              Geri
            </button>
            <button type="submit"
              disabled={create.isPending || !branchName.trim() || !branchSlug.trim()}
              className="flex-1 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
              {create.isPending ? "Oluşturuluyor..." : t.common.create}
            </button>
          </div>
        </form>
      </Shell>
    );
  }

  return null;
}

const inputClass =
  "w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring";

function Field({
  id, label, required, hint, children,
}: { id: string; label: string; required?: boolean; hint?: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label htmlFor={id} className="text-sm font-medium">
        {label}{required && <span className="text-[var(--critical)]"> *</span>}
      </label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  );
}

function Shell({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4 py-8">
      <div className="w-full max-w-md space-y-6 rounded-lg border border-border bg-card p-8 shadow-sm">
        <div className="space-y-1">
          <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
          <p className="text-sm text-muted-foreground">{subtitle}</p>
        </div>
        {children}
      </div>
    </div>
  );
}
