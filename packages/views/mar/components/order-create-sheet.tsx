"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Pill } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  createMedicationOrder,
  marKeys,
  type CreateMedicationOrderInput,
  type MedicationRoute,
} from "@medigt/core/mar";
import { medicationListOptions } from "@medigt/core/ilac";
import type { Medication } from "@medigt/core/types";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";

const ROUTE_OPTIONS: { value: MedicationRoute; label: string }[] = [
  { value: "oral", label: "Oral (PO)" },
  { value: "iv", label: "İntravenöz (IV)" },
  { value: "im", label: "İntramüsküler (IM)" },
  { value: "sc", label: "Subkütan (SC)" },
  { value: "topical", label: "Topikal" },
  { value: "inhalation", label: "İnhalasyon" },
  { value: "rectal", label: "Rektal" },
  { value: "sublingual", label: "Sublingual" },
  { value: "intranasal", label: "Burun içi" },
  { value: "ophthalmic", label: "Göz" },
  { value: "otic", label: "Kulak" },
  { value: "other", label: "Diğer" },
];

const FREQ_PRESETS = ["QD", "BID", "TID", "QID", "Q4H", "Q6H", "Q8H", "Q12H", "QHS", "PRN"];

// OrderCreateSheet — doktor için ilaç emri açar. Medikasyon kataloğundan
// arama + doz/yol/sıklık. Sıklık serbest metin ama yaygın preset'ler
// yardımcı butonlarla doldurulur.
export function OrderCreateSheet({
  admissionId,
  onClose,
}: {
  admissionId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const orgId = org?.id ?? "";
  const branchId = branch?.id ?? "";

  const [search, setSearch] = useState("");
  const medSearch = useQuery({
    ...medicationListOptions(orgId, { q: search, activeOnly: true }),
    enabled: !!orgId && search.length >= 2,
  });
  const [selected, setSelected] = useState<Medication | null>(null);

  const [form, setForm] = useState<CreateMedicationOrderInput>({
    medication_id: "",
    dose_amount: 0,
    dose_unit: "mg",
    route: "oral",
    frequency: "",
    scheduled_times: [],
    is_prn: false,
  });
  const [timesRaw, setTimesRaw] = useState("");

  const create = useMutation({
    mutationFn: () => {
      const times = timesRaw
        .split(/[,\s]+/)
        .map((s) => s.trim())
        .filter(Boolean);
      const input: CreateMedicationOrderInput = {
        ...form,
        medication_id: selected?.id ?? "",
        scheduled_times: form.is_prn ? [] : times,
      };
      return createMedicationOrder(admissionId, input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: marKeys.all(branchId) });
      onClose();
    },
  });

  const canSubmit =
    !!selected &&
    form.dose_amount > 0 &&
    !!form.dose_unit.trim() &&
    !!form.frequency.trim();

  return (
    <SideSheet open onClose={onClose} title="Yeni İlaç Emri">
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        <Field id="o-med" label="İlaç (kataloğu ara)" required>
          <TextInput
            id="o-med"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setSelected(null);
            }}
            placeholder="parol, paracetamol, ATC kodu..."
          />
        </Field>
        {!selected && search.length >= 2 && (
          <ul className="max-h-48 overflow-auto rounded-md border border-border">
            {(medSearch.data ?? []).slice(0, 20).map((m) => (
              <li key={m.id}>
                <button
                  type="button"
                  className="flex w-full items-start gap-2 px-3 py-2 text-left hover:bg-muted"
                  onClick={() => {
                    setSelected(m);
                    setSearch(m.name);
                  }}
                >
                  <Pill className="mt-0.5 h-4 w-4 text-muted-foreground" />
                  <div className="min-w-0">
                    <div className="text-sm font-medium">{m.name}</div>
                    <div className="text-xs text-muted-foreground">
                      {m.atc_code ?? ""} {m.form ? `· ${m.form}` : ""} {m.strength ?? ""}
                    </div>
                  </div>
                </button>
              </li>
            ))}
            {(medSearch.data ?? []).length === 0 && !medSearch.isLoading && (
              <li className="px-3 py-2 text-xs text-muted-foreground">
                Sonuç yok.
              </li>
            )}
          </ul>
        )}

        <div className="grid grid-cols-2 gap-3">
          <Field id="o-dose" label="Doz" required>
            <TextInput
              id="o-dose"
              type="number"
              min="0"
              step="0.001"
              required
              value={String(form.dose_amount)}
              onChange={(e) =>
                setForm({ ...form, dose_amount: Number(e.target.value) || 0 })
              }
            />
          </Field>
          <Field id="o-unit" label="Birim" required>
            <SelectInput
              id="o-unit"
              value={form.dose_unit}
              onChange={(e) => setForm({ ...form, dose_unit: e.target.value })}
            >
              {["mg", "g", "mcg", "mL", "L", "IU", "tab", "kap", "amp", "damla"].map(
                (u) => (
                  <option key={u} value={u}>
                    {u}
                  </option>
                ),
              )}
            </SelectInput>
          </Field>
        </div>

        <Field id="o-route" label="Uygulama yolu" required>
          <SelectInput
            id="o-route"
            value={form.route}
            onChange={(e) =>
              setForm({ ...form, route: e.target.value as MedicationRoute })
            }
          >
            {ROUTE_OPTIONS.map((r) => (
              <option key={r.value} value={r.value}>
                {r.label}
              </option>
            ))}
          </SelectInput>
        </Field>

        <Field id="o-freq" label="Sıklık" required hint="QD/BID/Q8H/PRN gibi mnemoticler veya serbest metin">
          <TextInput
            id="o-freq"
            required
            value={form.frequency}
            onChange={(e) => setForm({ ...form, frequency: e.target.value })}
            placeholder="örn. Q8H"
          />
          <div className="mt-1 flex flex-wrap gap-1">
            {FREQ_PRESETS.map((f) => (
              <button
                key={f}
                type="button"
                onClick={() => setForm({ ...form, frequency: f })}
                className="rounded border border-input bg-background px-2 py-0.5 text-xs hover:bg-muted"
              >
                {f}
              </button>
            ))}
          </div>
        </Field>

        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={form.is_prn ?? false}
            onChange={(e) => setForm({ ...form, is_prn: e.target.checked })}
          />
          <span>PRN (hasta isteğine bağlı, sabit saatte değil)</span>
        </label>

        {form.is_prn ? (
          <Field id="o-prn" label="PRN sebebi" hint="Hangi durumda verilebilir?">
            <TextInput
              id="o-prn"
              value={form.prn_reason ?? ""}
              onChange={(e) => setForm({ ...form, prn_reason: e.target.value })}
              placeholder="örn. ağrı, ateş, bulantı"
            />
          </Field>
        ) : (
          <Field
            id="o-times"
            label="Planlanan saatler"
            hint="HH:MM formatında, virgül veya boşlukla ayır. örn: 08:00, 16:00, 00:00"
          >
            <TextInput
              id="o-times"
              value={timesRaw}
              onChange={(e) => setTimesRaw(e.target.value)}
              placeholder="08:00, 16:00, 00:00"
            />
          </Field>
        )}

        <Field id="o-inst" label="Talimat / Not">
          <Textarea
            id="o-inst"
            rows={2}
            value={form.instructions ?? ""}
            onChange={(e) => setForm({ ...form, instructions: e.target.value })}
            placeholder="Yemekten önce, yavaş IV puşe vb."
          />
        </Field>

        {create.isError && (
          <p className="text-sm text-[var(--critical)]">
            Kayıt başarısız oldu. Tüm alanları kontrol edin.
          </p>
        )}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">
            İptal
          </SecondaryButton>
          <PrimaryButton
            type="submit"
            className="flex-1"
            disabled={!canSubmit || create.isPending}
          >
            {create.isPending ? "Kaydediliyor..." : "Emir aç"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
