"use client";

// LabOrderDrawer — reusable. Mounted inside the visit detail page as the
// "Lab İstek" tab's body. The drawer is content-only, the surrounding tab
// container in visit-detail-page.tsx handles layout.

import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, X } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  createLabOrder,
  labKeys,
  labTestSearchOptions,
} from "@medigt/core/laboratuvar";
import { poliklinikKeys } from "@medigt/core/poliklinik";
import type { LabOrderPriority, LabTest } from "@medigt/core/types";
import {
  Field,
  PrimaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";

const PRIORITY_LABELS: Record<LabOrderPriority, string> = {
  routine: "Rutin",
  urgent: "Acil",
  stat: "STAT",
};

export function LabOrderDrawer({ visitId }: { visitId: string }) {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const orgId = org?.id ?? "";
  const branchId = branch?.id ?? "";

  const qc = useQueryClient();
  const [picked, setPicked] = useState<LabTest[]>([]);
  const [priority, setPriority] = useState<LabOrderPriority>("routine");
  const [indication, setIndication] = useState("");

  const create = useMutation({
    mutationFn: () =>
      createLabOrder({
        visit_id: visitId,
        priority,
        clinical_indication: indication.trim() || undefined,
        test_ids: picked.map((t) => t.id),
      }),
    onSuccess: async () => {
      setPicked([]);
      setIndication("");
      setPriority("routine");
      await qc.invalidateQueries({ queryKey: labKeys.all(branchId) });
      await qc.invalidateQueries({ queryKey: poliklinikKeys.all(branchId) });
    },
  });

  return (
    <div className="space-y-5">
      <div className="space-y-3">
        <h3 className="text-sm font-semibold">Yeni Lab İstek</h3>
        <Field id="lab-test-search" label="Test ekle">
          <LabTestPicker
            orgId={orgId}
            excludeIds={new Set(picked.map((t) => t.id))}
            onPick={(t) => setPicked((prev) => [...prev, t])}
          />
        </Field>

        {picked.length > 0 && (
          <ul className="space-y-1">
            {picked.map((t) => (
              <li
                key={t.id}
                className="flex items-center justify-between gap-2 rounded-md border border-border bg-card px-3 py-2 text-sm"
              >
                <div>
                  <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{t.code}</code>{" "}
                  <span className="font-medium">{t.name}</span>
                  <span className="ml-2 text-xs text-muted-foreground">· {t.sample_type}</span>
                </div>
                <button
                  type="button"
                  onClick={() => setPicked((prev) => prev.filter((p) => p.id !== t.id))}
                  className="text-muted-foreground hover:text-[var(--critical)]"
                  aria-label="Çıkar"
                >
                  <X className="h-4 w-4" />
                </button>
              </li>
            ))}
          </ul>
        )}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field id="lab-priority" label="Öncelik">
            <SelectInput
              id="lab-priority"
              value={priority}
              onChange={(e) => setPriority(e.target.value as LabOrderPriority)}
            >
              {Object.entries(PRIORITY_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
        </div>

        <Field id="lab-indic" label="Klinik gerekçe">
          <Textarea
            id="lab-indic"
            rows={2}
            value={indication}
            onChange={(e) => setIndication(e.target.value)}
            placeholder="Örn. göğüs ağrısı, kardiyak enzim takibi"
          />
        </Field>

        {create.isError && <p className="text-sm text-[var(--critical)]">İstek oluşturulamadı.</p>}

        <div className="flex justify-end">
          <PrimaryButton
            type="button"
            onClick={() => create.mutate()}
            disabled={create.isPending || picked.length === 0}
          >
            <span className="inline-flex items-center gap-1">
              <Plus className="h-4 w-4" /> Lab İstek oluştur ({picked.length} test)
            </span>
          </PrimaryButton>
        </div>
      </div>
    </div>
  );
}

function LabTestPicker({
  orgId,
  excludeIds,
  onPick,
}: {
  orgId: string;
  excludeIds: Set<string>;
  onPick: (test: LabTest) => void;
}) {
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const search = useQuery(labTestSearchOptions(orgId, q));

  useEffect(() => {
    const onClickOutside = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("click", onClickOutside);
    return () => document.removeEventListener("click", onClickOutside);
  }, []);

  const filtered = (search.data ?? []).filter((t) => !excludeIds.has(t.id)).slice(0, 20);

  return (
    <div ref={ref} className="relative">
      <TextInput
        value={q}
        onChange={(e) => {
          setQ(e.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        placeholder="Test ara (HGB, GLU, kreatinin...)"
      />
      {open && filtered.length > 0 && (
        <ul className="absolute z-30 mt-1 max-h-72 w-full overflow-y-auto rounded-md border border-border bg-card shadow-lg">
          {filtered.map((t) => (
            <li key={t.id}>
              <button
                type="button"
                onClick={() => {
                  onPick(t);
                  setQ("");
                  setOpen(false);
                }}
                className="flex w-full items-start gap-2 px-3 py-2 text-left text-sm hover:bg-muted"
              >
                <code className="mt-0.5 rounded bg-muted px-1.5 py-0.5 text-xs">{t.code}</code>
                <div className="min-w-0">
                  <div className="font-medium">{t.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {t.sample_type}
                    {t.unit && ` · ${t.unit}`}
                    {t.reference_range && ` · ${t.reference_range}`}
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
