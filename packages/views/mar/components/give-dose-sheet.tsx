"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckSquare, Syringe } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  marKeys,
  recordAdministration,
  type AdministrationStatus,
  type MedicationOrder,
  type RecordAdministrationInput,
} from "@medigt/core/mar";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";

const STATUS_OPTIONS: { value: AdministrationStatus; label: string }[] = [
  { value: "given", label: "Verildi" },
  { value: "refused", label: "Hasta reddetti" },
  { value: "withheld", label: "Atlandı (NPO / doktor isteği)" },
  { value: "missed", label: "Kaçırıldı" },
  { value: "wrong_time", label: "Yanlış zamanda verildi" },
];

// GiveDoseSheet — bedside doz verme drawer. Hemşire 4 doğru checkbox'ı
// onaylar (hasta + ilaç barkod ayrıca taranır), opcional doz/yol override
// + not, sonra kaydeder. status='given' için 5_rights_checked zorunlu —
// backend de doğruluyor.
export function GiveDoseSheet({
  order,
  onClose,
}: {
  order: MedicationOrder;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";

  const [status, setStatus] = useState<AdministrationStatus>("given");
  // Checklist — beş "doğru" maddesi. Patient + Medication barkod taraması
  // ayrı saklanır ama "right patient" / "right medication" doğrulaması
  // genelde barkod ile yapılır. Burada hepsi kontrolü görsel onaylar.
  const [patientOk, setPatientOk] = useState(false);
  const [drugOk, setDrugOk] = useState(false);
  const [doseOk, setDoseOk] = useState(false);
  const [routeOk, setRouteOk] = useState(false);
  const [timeOk, setTimeOk] = useState(false);
  const allFive = patientOk && drugOk && doseOk && routeOk && timeOk;

  const [patientBarcode, setPatientBarcode] = useState("");
  const [drugBarcode, setDrugBarcode] = useState("");
  const [doseOverride, setDoseOverride] = useState("");
  const [doseUnit, setDoseUnit] = useState(order.dose_unit);
  const [notes, setNotes] = useState("");

  const save = useMutation({
    mutationFn: () => {
      const input: RecordAdministrationInput = {
        status,
        five_rights_checked: status === "given" ? allFive : true,
        patient_barcode_scanned: patientBarcode.trim() || undefined,
        medication_barcode_scanned: drugBarcode.trim() || undefined,
        dose_amount: doseOverride ? Number(doseOverride) : order.dose_amount,
        dose_unit: doseUnit || order.dose_unit,
        route: order.route,
        notes: notes.trim() || undefined,
      };
      return recordAdministration(order.id, input);
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: marKeys.all(branchId) });
      await qc.invalidateQueries({ queryKey: marKeys.adminsForOrder(order.id) });
      onClose();
    },
  });

  const canSubmit = status !== "given" || allFive;

  return (
    <SideSheet open onClose={onClose} title="Doz Ver">
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        <div className="rounded-md border border-border bg-muted/40 p-3 text-sm">
          <div className="font-semibold">{order.medication_name}</div>
          <div className="text-xs text-muted-foreground">
            {order.dose_amount} {order.dose_unit} · {order.route} · {order.frequency}
          </div>
          {order.instructions && (
            <p className="mt-1 text-xs italic">{order.instructions}</p>
          )}
        </div>

        <Field id="g-status" label="Durum">
          <SelectInput
            id="g-status"
            value={status}
            onChange={(e) => setStatus(e.target.value as AdministrationStatus)}
          >
            {STATUS_OPTIONS.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </SelectInput>
        </Field>

        {status === "given" && (
          <>
            <fieldset className="rounded-md border border-border p-3">
              <legend className="px-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                5 Doğru Kontrolü
              </legend>
              <div className="space-y-1.5 text-sm">
                <Check label="Doğru hasta" checked={patientOk} onChange={setPatientOk} />
                <Check label="Doğru ilaç" checked={drugOk} onChange={setDrugOk} />
                <Check label="Doğru doz" checked={doseOk} onChange={setDoseOk} />
                <Check label="Doğru uygulama yolu" checked={routeOk} onChange={setRouteOk} />
                <Check label="Doğru zaman" checked={timeOk} onChange={setTimeOk} />
              </div>
              {!allFive && (
                <p className="mt-2 text-xs text-amber-700 dark:text-amber-300">
                  5 doğru protokolü tamamlanmadan "verildi" olarak kaydedilemez.
                </p>
              )}
            </fieldset>

            <Field id="g-pb" label="Hasta barkodu (taratın)" hint="MRN bileziği QR/barkod">
              <TextInput
                id="g-pb"
                value={patientBarcode}
                onChange={(e) => setPatientBarcode(e.target.value)}
                placeholder="Bileklik kodunu okutun"
                autoFocus
              />
            </Field>
            <Field id="g-mb" label="İlaç barkodu (taratın)">
              <TextInput
                id="g-mb"
                value={drugBarcode}
                onChange={(e) => setDrugBarcode(e.target.value)}
                placeholder="Karekod / GTIN"
              />
            </Field>

            <details className="rounded-md border border-border bg-card p-2 text-sm">
              <summary className="cursor-pointer font-medium">
                Doz farklı verildi (override)
              </summary>
              <div className="mt-2 grid grid-cols-2 gap-2">
                <Field id="g-da" label="Verilen miktar">
                  <TextInput
                    id="g-da"
                    type="number"
                    min="0"
                    step="0.001"
                    value={doseOverride}
                    onChange={(e) => setDoseOverride(e.target.value)}
                    placeholder={String(order.dose_amount)}
                  />
                </Field>
                <Field id="g-du" label="Birim">
                  <TextInput
                    id="g-du"
                    value={doseUnit}
                    onChange={(e) => setDoseUnit(e.target.value)}
                  />
                </Field>
              </div>
            </details>
          </>
        )}

        <Field id="g-notes" label="Not">
          <Textarea
            id="g-notes"
            rows={2}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Yan etki, hastanın tepkisi vb."
          />
        </Field>

        {save.isError && (
          <p className="text-sm text-[var(--critical)]">Kayıt başarısız oldu.</p>
        )}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">
            İptal
          </SecondaryButton>
          <PrimaryButton
            type="submit"
            className="flex-1"
            disabled={!canSubmit || save.isPending}
          >
            <span className="inline-flex items-center gap-1">
              <Syringe className="h-4 w-4" />
              {save.isPending ? "Kaydediliyor..." : "Kaydet"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function Check({
  label,
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label className="flex cursor-pointer items-center gap-2">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      <CheckSquare className={`h-4 w-4 ${checked ? "text-emerald-600" : "text-muted-foreground"}`} />
      <span>{label}</span>
    </label>
  );
}
