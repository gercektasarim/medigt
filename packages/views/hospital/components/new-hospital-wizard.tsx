"use client";

import { useMemo, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import {
  Building2,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  FileBadge,
  GitBranch,
  Palette as PaletteIcon,
  Sparkles,
} from "lucide-react";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import { createOrganizationFull } from "@medigt/core/hospital";
import { PALETTES, type Palette } from "@medigt/core/theme/palette";
import { useHospitalStore } from "@medigt/core/hospital";
import type { Branch, OrganizationKind } from "@medigt/core/types";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  TextInput,
} from "../../common/form-fields";

// Multi-step "yeni hastane oluştur" wizard.
//
//   1. Kimlik       — name, slug, kind, tax_id, sgk employer no
//   2. İlk şube     — branch name/slug/kind/sgk facility code
//   3. Tema         — palette pick + dark mode toggle (saved on submit)
//   4. Onay         — özet + "Oluştur" → POST → switch + redirect
//
// Step 4 calls createOrganizationFull (org + initial branch in one shot)
// then sets the current hospital + branch in the store and pushes the
// user to /h/:org/:branch/baslangic so the onboarding checklist greets
// them. The hospital switcher's header now shows the new hospital.

type Step = 1 | 2 | 3 | 4;

const ORG_KIND_LABELS: Record<OrganizationKind, string> = {
  single_hospital: "Tek hastane",
  hospital_group: "Hastane grubu / zincir",
  clinic: "Klinik",
  polyclinic: "Poliklinik",
};

const BRANCH_KIND_LABELS: Record<Branch["kind"], string> = {
  hospital: "Hastane",
  polyclinic: "Poliklinik",
  lab: "Laboratuvar",
  imaging_center: "Görüntüleme merkezi",
  dialysis_center: "Diyaliz merkezi",
  dental_clinic: "Diş kliniği",
};

const PALETTE_LABELS: Record<Palette, string> = {
  teal: "Teal (sağlık önerilen)",
  blue: "Mavi",
  indigo: "Indigo",
  violet: "Mor",
  rose: "Pembe",
  amber: "Sarı / amber",
  green: "Yeşil",
  slate: "Gri / nötr",
};

type WizardState = {
  // Org
  orgName: string;
  orgSlug: string;
  orgKind: OrganizationKind;
  taxID: string;
  sgkEmployerNo: string;
  // Branch
  branchName: string;
  branchSlug: string;
  branchKind: Branch["kind"];
  sgkFacilityCode: string;
  // Theme
  palette: Palette;
};

function emptyState(): WizardState {
  return {
    orgName: "",
    orgSlug: "",
    orgKind: "single_hospital",
    taxID: "",
    sgkEmployerNo: "",
    branchName: "Merkez Şube",
    branchSlug: "merkez",
    branchKind: "hospital",
    sgkFacilityCode: "",
    palette: "teal",
  };
}

/** Slugify Turkish text — replace Turkish chars, lowercase, hyphenate. */
function slugify(s: string): string {
  return s
    .toLocaleLowerCase("tr")
    .replace(/ç/g, "c")
    .replace(/ğ/g, "g")
    .replace(/ı/g, "i")
    .replace(/ö/g, "o")
    .replace(/ş/g, "s")
    .replace(/ü/g, "u")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 40);
}

export function NewHospitalWizard() {
  const nav = useNavigation();
  const setHospital = useHospitalStore((s) => s.setOrganization);
  const setBranch = useHospitalStore((s) => s.setBranch);

  const [step, setStep] = useState<Step>(1);
  const [state, setState] = useState<WizardState>(emptyState());

  // Slug autofill — wherever the user hasn't manually overridden, derive
  // from the name. We track "overridden" via a simple convention: if the
  // user edits slug directly we leave it alone.
  const [slugTouched, setSlugTouched] = useState(false);
  const [branchSlugTouched, setBranchSlugTouched] = useState(false);

  const setField = <K extends keyof WizardState>(k: K, v: WizardState[K]) => {
    setState((s) => ({ ...s, [k]: v }));
  };

  // Live computed slug — only when user hasn't typed in the slug field.
  const effectiveSlug = useMemo(
    () => (slugTouched ? state.orgSlug : slugify(state.orgName)),
    [slugTouched, state.orgSlug, state.orgName],
  );
  const effectiveBranchSlug = useMemo(
    () => (branchSlugTouched ? state.branchSlug : slugify(state.branchName)),
    [branchSlugTouched, state.branchSlug, state.branchName],
  );

  const create = useMutation({
    mutationFn: () =>
      createOrganizationFull({
        slug: effectiveSlug,
        name: state.orgName,
        kind: state.orgKind,
        tax_id: state.taxID || undefined,
        sgk_employer_no: state.sgkEmployerNo || undefined,
        initial_branch: {
          slug: effectiveBranchSlug,
          name: state.branchName,
          kind: state.branchKind,
          sgk_facility_code: state.sgkFacilityCode || undefined,
        },
      }),
    onSuccess: (res) => {
      // Persist palette choice on the document root so the new hospital
      // inherits the theme on next paint.
      if (typeof document !== "undefined") {
        document.documentElement.setAttribute("data-palette", state.palette);
        try {
          window.localStorage.setItem("medigt:palette", state.palette);
        } catch {
          /* SSR / private mode — ignore */
        }
      }
      setHospital(res.organization);
      if (res.branch) {
        setBranch(res.branch);
      }
      const target = paths
        .hospital(res.organization.slug)
        .branch(res.branch?.slug ?? "merkez")
        .baslangic();
      nav.push(target);
    },
  });

  const canNext1 =
    state.orgName.trim().length >= 2 && effectiveSlug.length >= 2;
  const canNext2 =
    state.branchName.trim().length >= 2 && effectiveBranchSlug.length >= 2;

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-6">
      <header>
        <h1 className="text-2xl font-bold">Yeni Hastane Oluştur</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Bir kaç dakika sürer — kimlik, ilk şube ve tema seçiminden sonra
          başlangıç ekranına yönlendirileceksiniz.
        </p>
      </header>

      <Stepper step={step} />

      {step === 1 && (
        <IdentityStep
          state={state}
          setField={setField}
          effectiveSlug={effectiveSlug}
          slugTouched={slugTouched}
          setSlugTouched={setSlugTouched}
          canNext={canNext1}
          onNext={() => setStep(2)}
        />
      )}
      {step === 2 && (
        <BranchStep
          state={state}
          setField={setField}
          effectiveBranchSlug={effectiveBranchSlug}
          branchSlugTouched={branchSlugTouched}
          setBranchSlugTouched={setBranchSlugTouched}
          canNext={canNext2}
          onBack={() => setStep(1)}
          onNext={() => setStep(3)}
        />
      )}
      {step === 3 && (
        <ThemeStep
          state={state}
          setField={setField}
          onBack={() => setStep(2)}
          onNext={() => setStep(4)}
        />
      )}
      {step === 4 && (
        <ConfirmStep
          state={state}
          effectiveSlug={effectiveSlug}
          effectiveBranchSlug={effectiveBranchSlug}
          onBack={() => setStep(3)}
          onCreate={() => create.mutate()}
          creating={create.isPending}
          error={
            create.isError
              ? (create.error instanceof Error ? create.error.message : "Oluşturma başarısız")
              : null
          }
        />
      )}
    </div>
  );
}

// ---------- Stepper ----------

function Stepper({ step }: { step: Step }) {
  const items: { n: Step; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
    { n: 1, label: "Kimlik", icon: Building2 },
    { n: 2, label: "İlk şube", icon: GitBranch },
    { n: 3, label: "Tema", icon: PaletteIcon },
    { n: 4, label: "Onay", icon: CheckCircle2 },
  ];
  return (
    <ol className="flex items-center gap-2 overflow-x-auto">
      {items.map((s, i) => {
        const Icon = s.icon;
        const active = step === s.n;
        const done = step > s.n;
        return (
          <li
            key={s.n}
            className="step-pill"
            data-state={active ? "active" : done ? "done" : "pending"}
          >
            <Icon className="h-4 w-4" />
            <span>
              {s.n}. {s.label}
            </span>
            {i < items.length - 1 && <ChevronRight className="h-3 w-3 opacity-50" />}
          </li>
        );
      })}
    </ol>
  );
}

// ---------- Step 1 ----------

function IdentityStep({
  state,
  setField,
  effectiveSlug,
  slugTouched,
  setSlugTouched,
  canNext,
  onNext,
}: {
  state: WizardState;
  setField: <K extends keyof WizardState>(k: K, v: WizardState[K]) => void;
  effectiveSlug: string;
  slugTouched: boolean;
  setSlugTouched: (v: boolean) => void;
  canNext: boolean;
  onNext: () => void;
}) {
  return (
    <section className="space-y-4 rounded-lg border border-border bg-card p-4">
      <h2 className="text-base font-semibold">1) Hastane kimliği</h2>

      <Field id="o-name" label="Hastane adı" required>
        <TextInput
          id="o-name"
          required
          value={state.orgName}
          onChange={(e) => setField("orgName", e.target.value)}
          placeholder="örn. Demir Sağlık Tıp Merkezi"
        />
      </Field>

      <Field id="o-slug" label="URL slug" hint="2-40 karakter, küçük harf/rakam/tire. URL'de görünür.">
        <TextInput
          id="o-slug"
          value={slugTouched ? state.orgSlug : effectiveSlug}
          onChange={(e) => {
            setSlugTouched(true);
            setField("orgSlug", e.target.value);
          }}
          placeholder="otomatik üretilir"
        />
        <p className="mt-1 text-xs text-muted-foreground">
          URL: <code className="rounded bg-muted px-1">/h/{effectiveSlug || "..."}</code>
        </p>
      </Field>

      <Field id="o-kind" label="Kurum türü">
        <SelectInput
          id="o-kind"
          value={state.orgKind}
          onChange={(e) => setField("orgKind", e.target.value as OrganizationKind)}
        >
          {(Object.keys(ORG_KIND_LABELS) as OrganizationKind[]).map((k) => (
            <option key={k} value={k}>
              {ORG_KIND_LABELS[k]}
            </option>
          ))}
        </SelectInput>
      </Field>

      <div className="grid grid-cols-2 gap-3">
        <Field id="o-tax" label="Vergi no" hint="Faturada görünür">
          <TextInput
            id="o-tax"
            value={state.taxID}
            onChange={(e) => setField("taxID", e.target.value)}
            placeholder="10 haneli"
          />
        </Field>
        <Field
          id="o-sgk"
          label="SGK işveren no"
          hint="Medula provizyon için gerekli (sonra da girilebilir)"
        >
          <TextInput
            id="o-sgk"
            value={state.sgkEmployerNo}
            onChange={(e) => setField("sgkEmployerNo", e.target.value)}
          />
        </Field>
      </div>

      <div className="flex justify-end">
        <PrimaryButton type="button" onClick={onNext} disabled={!canNext}>
          İleri <ChevronRight className="ml-1 inline h-4 w-4" />
        </PrimaryButton>
      </div>
    </section>
  );
}

// ---------- Step 2 ----------

function BranchStep({
  state,
  setField,
  effectiveBranchSlug,
  branchSlugTouched,
  setBranchSlugTouched,
  canNext,
  onBack,
  onNext,
}: {
  state: WizardState;
  setField: <K extends keyof WizardState>(k: K, v: WizardState[K]) => void;
  effectiveBranchSlug: string;
  branchSlugTouched: boolean;
  setBranchSlugTouched: (v: boolean) => void;
  canNext: boolean;
  onBack: () => void;
  onNext: () => void;
}) {
  return (
    <section className="space-y-4 rounded-lg border border-border bg-card p-4">
      <h2 className="text-base font-semibold">2) İlk şube</h2>
      <p className="text-sm text-muted-foreground">
        Tek hastane / klinik için bu "Merkez" olur. Zincir hastane sahibiyseniz
        ilk şubeyi şimdi tanımlayın; diğerlerini sonra ekleyebilirsiniz.
      </p>

      <div className="grid grid-cols-2 gap-3">
        <Field id="b-name" label="Şube adı" required>
          <TextInput
            id="b-name"
            required
            value={state.branchName}
            onChange={(e) => setField("branchName", e.target.value)}
          />
        </Field>
        <Field id="b-slug" label="Şube slug" hint="URL'de görünür">
          <TextInput
            id="b-slug"
            value={branchSlugTouched ? state.branchSlug : effectiveBranchSlug}
            onChange={(e) => {
              setBranchSlugTouched(true);
              setField("branchSlug", e.target.value);
            }}
          />
        </Field>
      </div>

      <Field id="b-kind" label="Şube türü">
        <SelectInput
          id="b-kind"
          value={state.branchKind}
          onChange={(e) => setField("branchKind", e.target.value as Branch["kind"])}
        >
          {(Object.keys(BRANCH_KIND_LABELS) as Branch["kind"][]).map((k) => (
            <option key={k} value={k}>
              {BRANCH_KIND_LABELS[k]}
            </option>
          ))}
        </SelectInput>
      </Field>

      <Field
        id="b-sgk"
        label="SGK tesis kodu"
        hint="Provizyon almak için zorunlu — boş bırakılabilir, sonra ayarlardan eklenir"
      >
        <TextInput
          id="b-sgk"
          value={state.sgkFacilityCode}
          onChange={(e) => setField("sgkFacilityCode", e.target.value)}
        />
      </Field>

      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack}>
          <ChevronLeft className="mr-1 inline h-4 w-4" /> Geri
        </SecondaryButton>
        <PrimaryButton type="button" onClick={onNext} disabled={!canNext}>
          İleri <ChevronRight className="ml-1 inline h-4 w-4" />
        </PrimaryButton>
      </div>
    </section>
  );
}

// ---------- Step 3 ----------

function ThemeStep({
  state,
  setField,
  onBack,
  onNext,
}: {
  state: WizardState;
  setField: <K extends keyof WizardState>(k: K, v: WizardState[K]) => void;
  onBack: () => void;
  onNext: () => void;
}) {
  return (
    <section className="space-y-4 rounded-lg border border-border bg-card p-4">
      <h2 className="text-base font-semibold">3) Tema</h2>
      <p className="text-sm text-muted-foreground">
        Hastanenizin görsel kimliğini seçin. Hastane oluşturulduktan sonra
        ayarlardan tekrar değiştirilebilir.
      </p>

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {PALETTES.map((p) => {
          const active = state.palette === p;
          return (
            <button
              key={p}
              type="button"
              onClick={() => setField("palette", p)}
              data-palette={p}
              className={
                "rounded-md border p-3 text-left text-sm transition " +
                (active
                  ? "border-primary ring-2 ring-primary"
                  : "border-border hover:border-foreground/30")
              }
            >
              <div className="mb-2 flex items-center gap-1">
                <Swatch palette={p} />
              </div>
              <div className="font-medium">{PALETTE_LABELS[p]}</div>
            </button>
          );
        })}
      </div>

      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack}>
          <ChevronLeft className="mr-1 inline h-4 w-4" /> Geri
        </SecondaryButton>
        <PrimaryButton type="button" onClick={onNext}>
          İleri <ChevronRight className="ml-1 inline h-4 w-4" />
        </PrimaryButton>
      </div>
    </section>
  );
}

function Swatch({ palette }: { palette: Palette }) {
  // Generic chip — actual palette CSS variables are applied on click via
  // data-palette on document.documentElement. Here we just render 3
  // representative shades using semantic tokens, which the palette
  // stylesheet remaps when this swatch is wrapped in data-palette=...
  return (
    <div
      data-palette={palette}
      className="flex h-6 w-full overflow-hidden rounded"
    >
      <span className="flex-1 bg-primary" />
      <span className="flex-1 bg-primary/60" />
      <span className="flex-1 bg-primary/30" />
    </div>
  );
}

// ---------- Step 4 ----------

function ConfirmStep({
  state,
  effectiveSlug,
  effectiveBranchSlug,
  onBack,
  onCreate,
  creating,
  error,
}: {
  state: WizardState;
  effectiveSlug: string;
  effectiveBranchSlug: string;
  onBack: () => void;
  onCreate: () => void;
  creating: boolean;
  error: string | null;
}) {
  return (
    <section className="space-y-4 rounded-lg border border-border bg-card p-4">
      <h2 className="text-base font-semibold">4) Onay</h2>

      <div className="rounded-md border border-border bg-muted/30 p-3 text-sm">
        <h3 className="mb-2 flex items-center gap-2 font-semibold">
          <Building2 className="h-4 w-4" /> Hastane
        </h3>
        <Row label="Ad" value={state.orgName} />
        <Row label="Slug" value={effectiveSlug} mono />
        <Row label="Tür" value={ORG_KIND_LABELS[state.orgKind]} />
        {state.taxID && <Row label="Vergi no" value={state.taxID} mono />}
        {state.sgkEmployerNo && <Row label="SGK işveren no" value={state.sgkEmployerNo} mono />}
      </div>

      <div className="rounded-md border border-border bg-muted/30 p-3 text-sm">
        <h3 className="mb-2 flex items-center gap-2 font-semibold">
          <GitBranch className="h-4 w-4" /> İlk şube
        </h3>
        <Row label="Ad" value={state.branchName} />
        <Row label="Slug" value={effectiveBranchSlug} mono />
        <Row label="Tür" value={BRANCH_KIND_LABELS[state.branchKind]} />
        {state.sgkFacilityCode && <Row label="SGK tesis kodu" value={state.sgkFacilityCode} mono />}
      </div>

      <div className="rounded-md border border-border bg-muted/30 p-3 text-sm">
        <h3 className="mb-2 flex items-center gap-2 font-semibold">
          <PaletteIcon className="h-4 w-4" /> Tema
        </h3>
        <Row label="Palet" value={PALETTE_LABELS[state.palette]} />
      </div>

      <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
        <FileBadge className="mr-1 inline h-3.5 w-3.5" />
        Oluşturma sonrası başlangıç ekranındaki <strong>onboarding checklist</strong> üzerinden
        kurum tanımları + hizmet kataloğu + ilk doktor/personel kayıtlarını ekleyebilirsiniz.
        Tek tuş "Varsayılanları Yükle" seçeneği temel master data'yı seed eder.
      </div>

      {error && (
        <div className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-900 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-200">
          {error}
        </div>
      )}

      <div className="flex justify-between">
        <SecondaryButton type="button" onClick={onBack} disabled={creating}>
          <ChevronLeft className="mr-1 inline h-4 w-4" /> Geri
        </SecondaryButton>
        <PrimaryButton type="button" onClick={onCreate} disabled={creating}>
          <span className="inline-flex items-center gap-1">
            <Sparkles className="h-4 w-4" />
            {creating ? "Oluşturuluyor..." : "Hastaneyi oluştur"}
          </span>
        </PrimaryButton>
      </div>
    </section>
  );
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex justify-between gap-3 py-0.5">
      <span className="text-muted-foreground">{label}</span>
      <span className={mono ? "font-mono" : ""}>{value}</span>
    </div>
  );
}
