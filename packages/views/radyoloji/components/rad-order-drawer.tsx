"use client";

// RadOrderDrawer — embedded inside the visit detail's "Görüntüleme" tab.
// One procedure per order (unlike lab's multi-test), so the picker selects
// exactly one item then opens the priority + clinical fields.

import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  createRadOrder,
  radProcedureSearchOptions,
  radyolojiKeys,
} from "@medigt/core/radyoloji";
import { poliklinikKeys } from "@medigt/core/poliklinik";
import type {
  RadiologyModality,
  RadiologyOrderPriority,
  RadiologyProcedure,
} from "@medigt/core/types";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";

const PRIORITY_LABELS: Record<RadiologyOrderPriority, string> = {
  routine: "Rutin",
  urgent: "Acil",
  stat: "STAT",
};

const MODALITY_LABELS: Record<RadiologyModality, string> = {
  XR: "Röntgen",
  USG: "Ultrason",
  CT: "BT",
  MR: "MR",
  MAMMO: "Mamografi",
  NM: "Nükleer Tıp",
  DEXA: "Kemik Dansitometri",
  PET: "PET",
  ANGIO: "Anjiyo",
  FLUORO: "Floroskopi",
  OTHER: "Diğer",
};

export function RadOrderDrawer({ visitId }: { visitId: string }) {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const orgId = org?.id ?? "";
  const branchId = branch?.id ?? "";

  const qc = useQueryClient();
  const [picked, setPicked] = useState<RadiologyProcedure | null>(null);
  const [priority, setPriority] = useState<RadiologyOrderPriority>("routine");
  const [indication, setIndication] = useState("");
  const [question, setQuestion] = useState("");

  const create = useMutation({
    mutationFn: () =>
      createRadOrder({
        visit_id: visitId,
        procedure_id: picked!.id,
        priority,
        clinical_indication: indication.trim() || undefined,
        clinical_question: question.trim() || undefined,
      }),
    onSuccess: async () => {
      setPicked(null);
      setIndication("");
      setQuestion("");
      setPriority("routine");
      await qc.invalidateQueries({ queryKey: radyolojiKeys.all(branchId) });
      await qc.invalidateQueries({ queryKey: poliklinikKeys.all(branchId) });
    },
  });

  return (
    <div className="space-y-5">
      <div className="space-y-3">
        <h3 className="text-sm font-semibold">Yeni Görüntüleme İsteği</h3>
        {!picked ? (
          <Field id="rad-search" label="Tetkik ara">
            <RadProcedurePicker orgId={orgId} onPick={setPicked} />
          </Field>
        ) : (
          <div className="space-y-2 rounded-md border border-border bg-card p-3">
            <div className="flex items-start justify-between gap-2">
              <div>
                <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{picked.code}</code>{" "}
                <span className="font-medium">{picked.name}</span>
                <div className="mt-0.5 text-xs text-muted-foreground">
                  {MODALITY_LABELS[picked.modality]}
                  {picked.body_region && ` · ${picked.body_region}`}
                  {picked.estimated_minutes && ` · ~${picked.estimated_minutes} dk`}
                </div>
              </div>
              <SecondaryButton
                type="button"
                onClick={() => setPicked(null)}
                className="px-2 py-1 text-xs"
              >
                Değiştir
              </SecondaryButton>
            </div>
            {picked.preparation_notes && (
              <div className="rounded-md border border-amber-200 bg-amber-50 px-2 py-1.5 text-xs text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200">
                <strong>Hazırlık:</strong> {picked.preparation_notes}
              </div>
            )}
          </div>
        )}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field id="rad-priority" label="Öncelik">
            <SelectInput
              id="rad-priority"
              value={priority}
              onChange={(e) => setPriority(e.target.value as RadiologyOrderPriority)}
            >
              {Object.entries(PRIORITY_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
        </div>

        <Field id="rad-indic" label="Klinik gerekçe">
          <Textarea
            id="rad-indic"
            rows={2}
            value={indication}
            onChange={(e) => setIndication(e.target.value)}
            placeholder="Örn. öksürük 3 haftadır, kilo kaybı"
          />
        </Field>
        <Field id="rad-question" label="Klinik soru" hint="Spesifik sorulması istenen (opsiyonel)">
          <TextInput
            id="rad-question"
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder='"Pnömoni var mı?"'
          />
        </Field>

        {create.isError && <p className="text-sm text-[var(--critical)]">İstek oluşturulamadı.</p>}

        <div className="flex justify-end">
          <PrimaryButton
            type="button"
            onClick={() => create.mutate()}
            disabled={!picked || create.isPending}
          >
            <span className="inline-flex items-center gap-1">
              <Plus className="h-4 w-4" /> Görüntüleme İsteği oluştur
            </span>
          </PrimaryButton>
        </div>
      </div>
    </div>
  );
}

function RadProcedurePicker({
  orgId,
  onPick,
}: {
  orgId: string;
  onPick: (procedure: RadiologyProcedure) => void;
}) {
  const [q, setQ] = useState("");
  const [modality, setModality] = useState<RadiologyModality | "">("");
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const search = useQuery(radProcedureSearchOptions(orgId, q, modality || undefined));

  useEffect(() => {
    const onClickOutside = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("click", onClickOutside);
    return () => document.removeEventListener("click", onClickOutside);
  }, []);

  return (
    <div ref={ref} className="space-y-2">
      <div className="flex gap-2">
        <SelectInput
          value={modality}
          onChange={(e) => setModality(e.target.value as RadiologyModality | "")}
          className="max-w-[12rem]"
        >
          <option value="">Tüm modaliteler</option>
          {Object.entries(MODALITY_LABELS).map(([k, label]) => (
            <option key={k} value={k}>{label}</option>
          ))}
        </SelectInput>
        <div className="relative flex-1">
          <TextInput
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              setOpen(true);
            }}
            onFocus={() => setOpen(true)}
            placeholder="Tetkik ara (akciğer, batın, MR diz...)"
          />
        </div>
      </div>
      {open && (search.data ?? []).length > 0 && (
        <ul className="max-h-72 overflow-y-auto rounded-md border border-border bg-card shadow-sm">
          {(search.data ?? []).slice(0, 20).map((p) => (
            <li key={p.id}>
              <button
                type="button"
                onClick={() => {
                  onPick(p);
                  setQ("");
                  setOpen(false);
                }}
                className="flex w-full items-start gap-2 px-3 py-2 text-left text-sm hover:bg-muted"
              >
                <code className="mt-0.5 rounded bg-muted px-1.5 py-0.5 text-xs">{p.code}</code>
                <div className="min-w-0">
                  <div className="font-medium">{p.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {MODALITY_LABELS[p.modality]}
                    {p.body_region && ` · ${p.body_region}`}
                    {p.estimated_minutes && ` · ~${p.estimated_minutes} dk`}
                  </div>
                </div>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
