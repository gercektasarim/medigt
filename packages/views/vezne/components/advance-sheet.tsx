"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Wallet } from "lucide-react";
import {
  patientAccountOptions,
  receiveAdvance,
  cariKeys,
  type ReceiveAdvanceInput,
} from "@medigt/core/cari";
import { vezneKeys } from "@medigt/core/vezne";
import { formatTl } from "@medigt/core/utils";
import type { Patient, PaymentMethod } from "@medigt/core/types";
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

const METHOD_LABELS: Record<PaymentMethod, string> = {
  cash: "Nakit",
  card: "Kart",
  transfer: "Havale",
  mobile: "Mobil",
  other: "Diğer",
};

export function AdvanceSheet({
  branchId,
  registerId,
  onClose,
}: {
  branchId: string;
  registerId: string;
  onClose: () => void;
}) {
  const qc = useQueryClient();
  const [patient, setPatient] = useState<Patient | null>(null);
  const [amount, setAmount] = useState("");
  const [method, setMethod] = useState<PaymentMethod>("cash");
  const [notes, setNotes] = useState("");

  // Show current advance balance once a patient is picked.
  const account = useQuery({
    ...patientAccountOptions(patient?.id ?? ""),
    enabled: !!patient?.id,
  });

  const save = useMutation({
    mutationFn: () => {
      const input: ReceiveAdvanceInput = {
        amount: Number(amount),
        method,
        cash_register_id: method === "cash" ? registerId : undefined,
        notes: notes.trim() || undefined,
      };
      return receiveAdvance(patient!.id, input);
    },
    onSuccess: async () => {
      await Promise.all([
        qc.invalidateQueries({ queryKey: vezneKeys.all(branchId) }),
        qc.invalidateQueries({ queryKey: cariKeys.patientAccount(patient!.id) }),
      ]);
      onClose();
    },
  });

  const canSubmit = !!patient && Number(amount) > 0 && !save.isPending;

  return (
    <SideSheet open onClose={onClose} title="Avans Al">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
        <Field id="adv-patient" label="Hasta" required>
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

        {patient && account.data && (
          <div className="rounded-md border border-border bg-muted/40 p-3 text-sm">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">Mevcut avans bakiyesi</span>
              <span className={"font-mono font-medium " + (account.data.balance > 0 ? "text-emerald-700 dark:text-emerald-300" : "")}>
                {formatTl(account.data.balance)}
              </span>
            </div>
          </div>
        )}

        <Field id="adv-method" label="Ödeme yöntemi">
          <SelectInput id="adv-method" value={method} onChange={(e) => setMethod(e.target.value as PaymentMethod)}>
            {Object.entries(METHOD_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>

        <Field id="adv-amount" label="Tutar (TRY)" required>
          <TextInput
            id="adv-amount"
            type="number"
            min="0"
            step="0.01"
            required
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
          />
        </Field>

        <Field id="adv-notes" label="Notlar">
          <Textarea id="adv-notes" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </Field>

        {save.isError && (
          <p className="text-sm text-[var(--critical)]">Kayıt başarısız: {(save.error as Error)?.message}</p>
        )}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={!canSubmit}>
            <span className="inline-flex items-center gap-1">
              <Wallet className="h-4 w-4" /> {save.isPending ? "Kaydediliyor..." : "Avans al"}
            </span>
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
