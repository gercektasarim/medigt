"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Trash2 } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  createInvoice,
  faturaKeys,
  type CreateInvoiceItemInput,
} from "@medigt/core/fatura";
import { kurumListOptions } from "@medigt/core/kurum";
import { hizmetListOptions } from "@medigt/core/hizmet";
import { formatTl } from "@medigt/core/utils";
import type { Patient } from "@medigt/core/types";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";
import { HastaSearch } from "../../randevu/components/hasta-search";

type DraftItem = CreateInvoiceItemInput & { tempId: string };

function newTempId(): string {
  return Math.random().toString(36).slice(2, 10);
}

export function CreateInvoiceSheet({
  open,
  onClose,
  branchId,
}: {
  open: boolean;
  onClose: () => void;
  branchId: string;
}) {
  const qc = useQueryClient();
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const institutions = useQuery(kurumListOptions(orgId, true));
  const services = useQuery(hizmetListOptions(orgId, {}));

  const [patient, setPatient] = useState<Patient | null>(null);
  const [institutionId, setInstitutionId] = useState("");
  const [notes, setNotes] = useState("");
  const [items, setItems] = useState<DraftItem[]>([]);
  const [finalize, setFinalize] = useState(true);

  function addItem() {
    setItems([
      ...items,
      {
        tempId: newTempId(),
        code: "",
        name: "",
        quantity: 1,
        unit_price: 0,
        discount_pct: 0,
        vat_rate: 10,
      },
    ]);
  }

  function updateItem(tempId: string, patch: Partial<CreateInvoiceItemInput>) {
    setItems((prev) => prev.map((it) => (it.tempId === tempId ? { ...it, ...patch } : it)));
  }

  function removeItem(tempId: string) {
    setItems((prev) => prev.filter((it) => it.tempId !== tempId));
  }

  function pickServiceForItem(tempId: string, serviceId: string) {
    const svc = services.data?.find((s) => s.id === serviceId);
    if (!svc) {
      updateItem(tempId, { service_id: undefined });
      return;
    }
    updateItem(tempId, {
      service_id: svc.id,
      code: svc.code,
      name: svc.name,
      unit_price: svc.base_price ?? 0,
      vat_rate: svc.vat_rate,
    });
  }

  const create = useMutation({
    mutationFn: () =>
      createInvoice({
        patient_id: patient!.id,
        institution_id: institutionId || undefined,
        notes: notes.trim() || undefined,
        finalize,
        items: items.map(({ tempId: _t, ...rest }) => rest),
      }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: faturaKeys.all(branchId) });
      setPatient(null);
      setInstitutionId("");
      setNotes("");
      setItems([]);
      onClose();
    },
  });

  const subtotal = items.reduce((s, it) => s + it.quantity * it.unit_price * (1 - (it.discount_pct ?? 0) / 100), 0);
  const taxTotal = items.reduce((s, it) => {
    const ls = it.quantity * it.unit_price * (1 - (it.discount_pct ?? 0) / 100);
    return s + ls * (it.vat_rate ?? 0) / 100;
  }, 0);
  const grandTotal = subtotal + taxTotal;
  const canSubmit = patient && items.length > 0 && items.every((it) => it.name && it.quantity > 0 && it.unit_price >= 0) && !create.isPending;

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Fatura">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="i-patient" label="Hasta" required>
          {patient ? (
            <div className="flex items-center justify-between rounded-md border border-border bg-muted/40 px-3 py-2">
              <div>
                <div className="font-medium">{patient.first_name} {patient.last_name}</div>
                <div className="text-xs text-muted-foreground">MRN {patient.mrn}</div>
              </div>
              <button type="button" onClick={() => setPatient(null)} className="text-xs text-muted-foreground hover:underline">
                Değiştir
              </button>
            </div>
          ) : (
            <HastaSearch onPick={setPatient} />
          )}
        </Field>

        <Field id="i-inst" label="Kurum (opsiyonel)" hint="Boş bırakırsanız cepten ödeme olarak işlenir.">
          <SelectInput
            id="i-inst"
            value={institutionId}
            onChange={(e) => setInstitutionId(e.target.value)}
          >
            <option value="">— Cepten ödeme —</option>
            {(institutions.data ?? []).map((i) => (
              <option key={i.id} value={i.id}>{i.name}</option>
            ))}
          </SelectInput>
        </Field>

        <section className="space-y-2 border-t border-border pt-3">
          <header className="flex items-center justify-between">
            <h3 className="text-sm font-semibold">Kalemler</h3>
            <SecondaryButton type="button" onClick={addItem}>
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Kalem ekle</span>
            </SecondaryButton>
          </header>

          {items.length === 0 && (
            <p className="text-sm text-muted-foreground">Henüz kalem yok. Üstten "Kalem ekle" ile ekleyin.</p>
          )}

          <ul className="space-y-3">
            {items.map((it) => {
              const lineSubtotal = it.quantity * it.unit_price * (1 - (it.discount_pct ?? 0) / 100);
              const lineTax = lineSubtotal * (it.vat_rate ?? 0) / 100;
              const lineTotal = lineSubtotal + lineTax;
              return (
                <li key={it.tempId} className="rounded-md border border-border p-3 space-y-2">
                  <div className="grid grid-cols-12 gap-2 items-end">
                    <div className="col-span-12 sm:col-span-6">
                      <Field id={`svc-${it.tempId}`} label="Hizmet (katalogdan)">
                        <SelectInput
                          id={`svc-${it.tempId}`}
                          value={it.service_id ?? ""}
                          onChange={(e) => pickServiceForItem(it.tempId, e.target.value)}
                        >
                          <option value="">— Serbest kalem —</option>
                          {(services.data ?? []).slice(0, 200).map((s) => (
                            <option key={s.id} value={s.id}>{s.name}</option>
                          ))}
                        </SelectInput>
                      </Field>
                    </div>
                    <div className="col-span-12 sm:col-span-5">
                      <Field id={`name-${it.tempId}`} label="Ad" required>
                        <TextInput
                          id={`name-${it.tempId}`}
                          required
                          value={it.name}
                          onChange={(e) => updateItem(it.tempId, { name: e.target.value })}
                        />
                      </Field>
                    </div>
                    <div className="col-span-12 sm:col-span-1 flex">
                      <button
                        type="button"
                        onClick={() => removeItem(it.tempId)}
                        className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-input bg-background text-[var(--critical)] hover:bg-muted"
                        title="Kaldır"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </div>

                  <div className="grid grid-cols-4 gap-2">
                    <Field id={`qty-${it.tempId}`} label="Miktar">
                      <TextInput
                        id={`qty-${it.tempId}`}
                        type="number"
                        min="0"
                        step="0.001"
                        value={String(it.quantity)}
                        onChange={(e) => updateItem(it.tempId, { quantity: Number(e.target.value) || 0 })}
                      />
                    </Field>
                    <Field id={`up-${it.tempId}`} label="Birim Fiyat">
                      <TextInput
                        id={`up-${it.tempId}`}
                        type="number"
                        min="0"
                        step="0.01"
                        value={String(it.unit_price)}
                        onChange={(e) => updateItem(it.tempId, { unit_price: Number(e.target.value) || 0 })}
                      />
                    </Field>
                    <Field id={`disc-${it.tempId}`} label="İskonto %">
                      <TextInput
                        id={`disc-${it.tempId}`}
                        type="number"
                        min="0"
                        max="100"
                        step="0.1"
                        value={String(it.discount_pct ?? 0)}
                        onChange={(e) => updateItem(it.tempId, { discount_pct: Number(e.target.value) || 0 })}
                      />
                    </Field>
                    <Field id={`vat-${it.tempId}`} label="KDV %">
                      <TextInput
                        id={`vat-${it.tempId}`}
                        type="number"
                        min="0"
                        max="100"
                        step="0.5"
                        value={String(it.vat_rate ?? 0)}
                        onChange={(e) => updateItem(it.tempId, { vat_rate: Number(e.target.value) || 0 })}
                      />
                    </Field>
                  </div>
                  <div className="flex items-center justify-end gap-3 text-xs text-muted-foreground">
                    <span>Ara: {formatTl(lineSubtotal)}</span>
                    <span>KDV: {formatTl(lineTax)}</span>
                    <span className="font-mono font-medium text-foreground">
                      Toplam: {formatTl(lineTotal)}
                    </span>
                  </div>
                </li>
              );
            })}
          </ul>
        </section>

        {items.length > 0 && (
          <div className="space-y-1 rounded-md border border-border bg-muted/40 p-3 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Ara Toplam</span>
              <span className="font-mono">{formatTl(subtotal)}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">KDV</span>
              <span className="font-mono">{formatTl(taxTotal)}</span>
            </div>
            <div className="flex items-center justify-between border-t border-border pt-1 text-base font-semibold">
              <span>Genel Toplam</span>
              <span className="font-mono">{formatTl(grandTotal)}</span>
            </div>
          </div>
        )}

        <Field id="i-notes" label="Notlar">
          <Textarea id="i-notes" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </Field>

        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={finalize}
            onChange={(e) => setFinalize(e.target.checked)}
            className="h-4 w-4 rounded border-input"
          />
          Oluşturduğumda hemen onayla (ödeme alınabilir hale getir)
        </label>

        {create.isError && (
          <p className="text-sm text-[var(--critical)]">
            Kayıt başarısız: {(create.error as Error)?.message}
          </p>
        )}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            {create.isPending ? "Kaydediliyor..." : "Faturayı oluştur"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
