"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Search } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  createHasta,
  hastaKeys,
  hastaListOptions,
  validateTCLocal,
  type CreateHastaInput,
} from "@medigt/core/hasta";
import { formatDateTr } from "@medigt/core/utils";
import type {
  BloodType,
  Patient,
  PatientGender,
  PatientIdentifierKind,
} from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";

const GENDER_LABELS: Record<PatientGender, string> = {
  male: "Erkek",
  female: "Kadın",
  unknown: "Belirtilmemiş",
};

const BLOOD_LABELS: Record<BloodType, string> = {
  A_pos: "A Rh+", A_neg: "A Rh-",
  B_pos: "B Rh+", B_neg: "B Rh-",
  AB_pos: "AB Rh+", AB_neg: "AB Rh-",
  O_pos: "0 Rh+", O_neg: "0 Rh-",
  unknown: "Bilinmiyor",
};

const IDENTIFIER_LABELS: Record<PatientIdentifierKind, string> = {
  tc: "T.C. Kimlik No",
  passport: "Pasaport",
  foreigner_id: "Yabancı Kimlik No",
  temporary_protection: "Geçici Koruma Kimlik No",
  newborn: "Yenidoğan (TC yok)",
};

export function HastaPage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const [search, setSearch] = useState("");
  const [debounced, setDebounced] = useState("");
  useEffect(() => {
    const t = setTimeout(() => setDebounced(search.trim()), 200);
    return () => clearTimeout(t);
  }, [search]);

  const list = useQuery(hastaListOptions(orgId, debounced));
  const [createOpen, setCreateOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Hastalar"
          subtitle="Hasta kayıtları, kabul ve dosya yönetimi. Arama: ad/soyad, TC, telefon, MRN."
          actions={
            <PrimaryButton type="button" onClick={() => setCreateOpen(true)}>
              <span className="inline-flex items-center gap-1">
                <Plus className="h-4 w-4" /> Yeni Hasta
              </span>
            </PrimaryButton>
          }
        />

        <div className="relative max-w-md">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <TextInput
            autoFocus
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Ara: Ahmet Yılmaz, 12345678901, 0555..."
            className="pl-9"
          />
        </div>

        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : list.isError ? (
          <div className="empty-state text-[var(--critical)]">Liste yüklenemedi.</div>
        ) : (
          <DataTable<Patient>
            rows={list.data ?? []}
            rowKey={(r) => r.id}
            columns={hastaColumns}
          />
        )}
      </div>

      <CreateHastaSheet open={createOpen} onClose={() => setCreateOpen(false)} orgId={orgId} />
    </DashboardLayout>
  );
}

const hastaColumns: Column<Patient>[] = [
  {
    key: "mrn",
    header: "MRN",
    cell: (p) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{p.mrn}</code>,
  },
  {
    key: "name",
    header: "Ad Soyad",
    cell: (p) => (
      <div>
        <div className="font-medium">{p.first_name} {p.last_name}</div>
        {p.birth_date && (
          <div className="text-xs text-muted-foreground">
            {formatDateTr(p.birth_date)}
          </div>
        )}
      </div>
    ),
  },
  {
    key: "identity",
    header: "Kimlik",
    cell: (p) =>
      p.identifier_value ? (
        <div className="text-sm">
          <div className="font-mono">{p.identifier_masked ?? p.identifier_value}</div>
          {p.identifier_kind && (
            <div className="text-xs text-muted-foreground">
              {IDENTIFIER_LABELS[p.identifier_kind] ?? p.identifier_kind}
            </div>
          )}
        </div>
      ) : (
        <span className="text-xs text-muted-foreground">—</span>
      ),
  },
  { key: "gender", header: "Cins.", cell: (p) => GENDER_LABELS[p.gender] ?? "—" },
  {
    key: "blood",
    header: "Kan Grubu",
    cell: (p) => (p.blood_type === "unknown" ? "—" : BLOOD_LABELS[p.blood_type]),
  },
  { key: "phone", header: "Telefon", cell: (p) => p.phone ?? "—" },
];

function CreateHastaSheet({ open, onClose, orgId }: { open: boolean; onClose: () => void; orgId: string }) {
  const qc = useQueryClient();
  const [form, setForm] = useState<CreateHastaInput>({
    first_name: "",
    last_name: "",
    gender: "unknown",
    blood_type: "unknown",
    identifier_kind: "tc",
  });

  // Reset form when sheet opens.
  useEffect(() => {
    if (open) {
      setForm({
        first_name: "",
        last_name: "",
        gender: "unknown",
        blood_type: "unknown",
        identifier_kind: "tc",
      });
    }
  }, [open]);

  const tcLooksValid =
    form.identifier_kind !== "tc" ||
    !form.identifier_value ||
    form.identifier_value.length === 0 ||
    validateTCLocal(form.identifier_value);

  const create = useMutation({
    mutationFn: () => createHasta(form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: hastaKeys.all(orgId) });
      onClose();
    },
  });

  const errorMsg = (() => {
    if (!create.isError) return null;
    const err = create.error as { code?: string; message?: string } | undefined;
    if (err?.code === "patient_exists") return "Bu kimlik bilgisiyle hasta zaten kayıtlı.";
    if (err?.code === "invalid_tc") return "TC kimlik no doğrulanmadı (checksum hatası).";
    return err?.message ?? "Kayıt başarısız.";
  })();

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Hasta">
      <form
        className="space-y-5"
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        {/* --- Kimlik bilgisi --- */}
        <section className="space-y-3">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Kimlik
          </h3>
          <div className="grid grid-cols-3 gap-3">
            <Field id="h-ikind" label="Tür">
              <SelectInput
                id="h-ikind"
                value={form.identifier_kind ?? "tc"}
                onChange={(e) =>
                  setForm({ ...form, identifier_kind: e.target.value as PatientIdentifierKind })
                }
              >
                {Object.entries(IDENTIFIER_LABELS).map(([k, label]) => (
                  <option key={k} value={k}>{label}</option>
                ))}
              </SelectInput>
            </Field>
            <Field
              id="h-ivalue"
              label={IDENTIFIER_LABELS[form.identifier_kind ?? "tc"] ?? "No"}
              error={
                form.identifier_value && form.identifier_kind === "tc" && !tcLooksValid
                  ? "TC kimlik no algoritmaya göre geçersiz"
                  : undefined
              }
            >
              <TextInput
                id="h-ivalue"
                value={form.identifier_value ?? ""}
                onChange={(e) =>
                  setForm({
                    ...form,
                    identifier_value:
                      form.identifier_kind === "tc"
                        ? e.target.value.replace(/\D/g, "").slice(0, 11)
                        : e.target.value,
                  })
                }
                inputMode={form.identifier_kind === "tc" ? "numeric" : "text"}
                placeholder={form.identifier_kind === "tc" ? "11 haneli TC" : ""}
              />
            </Field>
            <Field id="h-birth" label="Doğum tarihi">
              <TextInput
                id="h-birth"
                type="date"
                value={form.birth_date ?? ""}
                onChange={(e) => setForm({ ...form, birth_date: e.target.value })}
              />
            </Field>
          </div>
        </section>

        {/* --- Demografik --- */}
        <section className="space-y-3">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Demografik
          </h3>
          <div className="grid grid-cols-2 gap-3">
            <Field id="h-first" label="Ad" required>
              <TextInput
                id="h-first"
                required
                value={form.first_name}
                onChange={(e) => setForm({ ...form, first_name: e.target.value })}
              />
            </Field>
            <Field id="h-last" label="Soyad" required>
              <TextInput
                id="h-last"
                required
                value={form.last_name}
                onChange={(e) => setForm({ ...form, last_name: e.target.value })}
              />
            </Field>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <Field id="h-gender" label="Cinsiyet">
              <SelectInput
                id="h-gender"
                value={form.gender ?? "unknown"}
                onChange={(e) => setForm({ ...form, gender: e.target.value as PatientGender })}
              >
                {Object.entries(GENDER_LABELS).map(([k, label]) => (
                  <option key={k} value={k}>{label}</option>
                ))}
              </SelectInput>
            </Field>
            <Field id="h-blood" label="Kan grubu">
              <SelectInput
                id="h-blood"
                value={form.blood_type ?? "unknown"}
                onChange={(e) => setForm({ ...form, blood_type: e.target.value as BloodType })}
              >
                {Object.entries(BLOOD_LABELS).map(([k, label]) => (
                  <option key={k} value={k}>{label}</option>
                ))}
              </SelectInput>
            </Field>
          </div>
        </section>

        {/* --- İletişim --- */}
        <section className="space-y-3">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            İletişim
          </h3>
          <div className="grid grid-cols-2 gap-3">
            <Field id="h-phone" label="Telefon">
              <TextInput
                id="h-phone"
                value={form.phone ?? ""}
                onChange={(e) => setForm({ ...form, phone: e.target.value })}
                placeholder="+90 5XX XXX XX XX"
              />
            </Field>
            <Field id="h-email" label="E-posta">
              <TextInput
                id="h-email"
                type="email"
                value={form.email ?? ""}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
              />
            </Field>
          </div>
          <Field id="h-addr" label="Adres">
            <Textarea
              id="h-addr"
              rows={2}
              value={form.address ?? ""}
              onChange={(e) => setForm({ ...form, address: e.target.value })}
            />
          </Field>
        </section>

        {/* --- Yakını --- */}
        <section className="space-y-3">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Yakını / Acil durum
          </h3>
          <div className="grid grid-cols-2 gap-3">
            <Field id="h-kin-name" label="Adı">
              <TextInput
                id="h-kin-name"
                value={form.next_of_kin_name ?? ""}
                onChange={(e) => setForm({ ...form, next_of_kin_name: e.target.value })}
              />
            </Field>
            <Field id="h-kin-phone" label="Telefonu">
              <TextInput
                id="h-kin-phone"
                value={form.next_of_kin_phone ?? ""}
                onChange={(e) => setForm({ ...form, next_of_kin_phone: e.target.value })}
              />
            </Field>
          </div>
        </section>

        <Field id="h-notes" label="Notlar">
          <Textarea
            id="h-notes"
            rows={3}
            value={form.notes ?? ""}
            onChange={(e) => setForm({ ...form, notes: e.target.value })}
          />
        </Field>

        {errorMsg && <p className="text-sm text-[var(--critical)]">{errorMsg}</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">
            İptal
          </SecondaryButton>
          <PrimaryButton
            type="submit"
            className="flex-1"
            disabled={
              create.isPending ||
              !form.first_name ||
              !form.last_name ||
              !tcLooksValid
            }
          >
            {create.isPending ? "Kaydediliyor..." : "Hastayı Kaydet"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
