"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2 } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  labKeys,
  labOrderDetailOptions,
  updateLabItemResult,
  updateLabOrderStatus,
} from "@medigt/core/laboratuvar";
import { formatDateTr } from "@medigt/core/utils";
import type {
  LabOrder,
  LabOrderItem,
  LabResultFlag,
} from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  TextInput,
} from "../../common/form-fields";

const FLAG_LABELS: Record<LabResultFlag, string> = {
  normal: "Normal",
  low: "Düşük",
  high: "Yüksek",
  critical_low: "Kritik düşük",
  critical_high: "Kritik yüksek",
  abnormal: "Anormal",
};

const FLAG_COLORS: Record<LabResultFlag, string> = {
  normal: "text-emerald-700 dark:text-emerald-300",
  low: "text-amber-700 dark:text-amber-300",
  high: "text-amber-700 dark:text-amber-300",
  critical_low: "text-[var(--critical)] font-semibold",
  critical_high: "text-[var(--critical)] font-semibold",
  abnormal: "text-amber-800 dark:text-amber-300",
};

export function LabOrderDetailPage({ orderId }: { orderId: string }) {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const order = useQuery(labOrderDetailOptions(branchId, orderId));

  if (order.isLoading) {
    return <DashboardLayout><div className="page-shell">Yükleniyor...</div></DashboardLayout>;
  }
  if (order.isError || !order.data) {
    return (
      <DashboardLayout>
        <div className="page-shell">
          <div className="empty-state text-[var(--critical)]">Lab istek bulunamadı.</div>
        </div>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title={`Lab İstek · ${order.data.order_no}`}
          subtitle={`${order.data.patient_first_name} ${order.data.patient_last_name} · MRN ${order.data.patient_mrn}`}
          actions={<HeaderActions order={order.data} branchId={branchId} />}
        />

        <OrderMeta order={order.data} />

        <section className="space-y-2">
          <h2 className="text-sm font-semibold text-muted-foreground">Testler</h2>
          <div className="space-y-2">
            {order.data.items.map((item) => (
              <ResultRow key={item.id} item={item} branchId={branchId} orderId={order.data!.id} />
            ))}
          </div>
        </section>
      </div>
    </DashboardLayout>
  );
}

function OrderMeta({ order }: { order: LabOrder }) {
  return (
    <div className="grid grid-cols-2 gap-3 rounded-md border border-border bg-card p-3 text-sm sm:grid-cols-4">
      <div>
        <div className="text-xs text-muted-foreground">İstendi</div>
        <div className="font-medium">{new Date(order.ordered_at).toLocaleString("tr-TR")}</div>
      </div>
      <div>
        <div className="text-xs text-muted-foreground">İsteyen</div>
        <div className="font-medium">
          {order.doctor_first_name
            ? `${order.doctor_title ? order.doctor_title + " " : ""}${order.doctor_first_name} ${order.doctor_last_name}`
            : "—"}
        </div>
      </div>
      <div>
        <div className="text-xs text-muted-foreground">Öncelik</div>
        <div className={"font-medium " + (order.priority === "stat" ? "text-[var(--critical)]" : "")}>
          {order.priority.toUpperCase()}
        </div>
      </div>
      <div>
        <div className="text-xs text-muted-foreground">Durum</div>
        <div className="font-medium">{order.status}</div>
      </div>
      {order.clinical_indication && (
        <div className="col-span-full">
          <div className="text-xs text-muted-foreground">Klinik gerekçe</div>
          <div className="font-medium">{order.clinical_indication}</div>
        </div>
      )}
    </div>
  );
}

function HeaderActions({ order, branchId }: { order: LabOrder; branchId: string }) {
  const qc = useQueryClient();
  const sample = useMutation({
    mutationFn: () => updateLabOrderStatus(order.id, "sampled"),
    onSuccess: () => qc.invalidateQueries({ queryKey: labKeys.all(branchId) }),
  });
  const verify = useMutation({
    mutationFn: () => updateLabOrderStatus(order.id, "verified"),
    onSuccess: () => qc.invalidateQueries({ queryKey: labKeys.all(branchId) }),
  });

  if (order.status === "ordered") {
    return (
      <PrimaryButton type="button" onClick={() => sample.mutate()} disabled={sample.isPending}>
        Numune Alındı
      </PrimaryButton>
    );
  }
  if (order.status === "resulted") {
    return (
      <PrimaryButton type="button" onClick={() => verify.mutate()} disabled={verify.isPending}>
        <span className="inline-flex items-center gap-1">
          <CheckCircle2 className="h-4 w-4" /> Sonuçları Onayla
        </span>
      </PrimaryButton>
    );
  }
  return null;
}

function ResultRow({
  item,
  branchId,
  orderId,
}: {
  item: LabOrderItem;
  branchId: string;
  orderId: string;
}) {
  const qc = useQueryClient();
  const [valueNumeric, setValueNumeric] = useState(item.value_numeric?.toString() ?? "");
  const [valueText, setValueText] = useState(item.value_text ?? "");
  const [flag, setFlag] = useState<LabResultFlag | "">(item.flag ?? "");
  const [notes, setNotes] = useState(item.notes ?? "");
  const [edit, setEdit] = useState(false);

  // Local UI mirrors server when the item updates.
  useEffect(() => {
    setValueNumeric(item.value_numeric?.toString() ?? "");
    setValueText(item.value_text ?? "");
    setFlag(item.flag ?? "");
    setNotes(item.notes ?? "");
  }, [item.value_numeric, item.value_text, item.flag, item.notes]);

  const save = useMutation({
    mutationFn: () =>
      updateLabItemResult(item.id, {
        value_numeric: valueNumeric.trim() ? Number(valueNumeric) : undefined,
        value_text: valueText.trim() || undefined,
        flag: flag || undefined,
        notes: notes.trim() || undefined,
      }),
    onSuccess: async () => {
      setEdit(false);
      await qc.invalidateQueries({ queryKey: labKeys.order(branchId, orderId) });
      await qc.invalidateQueries({ queryKey: labKeys.all(branchId) });
    },
  });

  const hasResult = item.status === "resulted" || item.value_numeric != null || item.value_text;

  return (
    <div className="rounded-md border border-border bg-card p-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{item.test_code}</code>
            <span className="font-medium">{item.test_name}</span>
            <span className="text-xs text-muted-foreground">· {item.sample_type}</span>
          </div>
          {item.reference_range && (
            <div className="text-xs text-muted-foreground">Referans: {item.reference_range}</div>
          )}
        </div>
        {!edit && hasResult && item.resulted_at && (
          <div className="text-right text-xs text-muted-foreground">
            {formatDateTr(item.resulted_at)} {new Date(item.resulted_at).toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" })}
          </div>
        )}
      </div>

      {!edit ? (
        <div className="mt-2 flex flex-wrap items-center gap-3">
          {hasResult ? (
            <>
              <div>
                <span className="text-lg font-semibold">
                  {item.value_numeric != null
                    ? item.value_numeric
                    : item.value_text ?? "—"}
                </span>
                {item.unit && <span className="ml-1 text-sm text-muted-foreground">{item.unit}</span>}
              </div>
              {item.flag && (
                <span className={"text-sm " + FLAG_COLORS[item.flag]}>
                  {FLAG_LABELS[item.flag]}
                </span>
              )}
              {item.notes && (
                <span className="text-xs italic text-muted-foreground">{item.notes}</span>
              )}
            </>
          ) : (
            <span className="text-sm text-muted-foreground">Henüz sonuç girilmedi.</span>
          )}
          <SecondaryButton
            type="button"
            onClick={() => setEdit(true)}
            className="ml-auto px-2 py-1 text-xs"
          >
            {hasResult ? "Düzenle" : "Sonuç gir"}
          </SecondaryButton>
        </div>
      ) : (
        <div className="mt-2 grid grid-cols-1 gap-2 sm:grid-cols-5">
          <Field id={`v-num-${item.id}`} label="Değer (sayısal)">
            <TextInput
              id={`v-num-${item.id}`}
              type="number"
              step="0.0001"
              value={valueNumeric}
              onChange={(e) => setValueNumeric(e.target.value)}
            />
          </Field>
          <Field id={`v-text-${item.id}`} label="veya metin">
            <TextInput
              id={`v-text-${item.id}`}
              value={valueText}
              onChange={(e) => setValueText(e.target.value)}
              placeholder="Pozitif / negatif vb."
            />
          </Field>
          <Field id={`v-flag-${item.id}`} label="Bayrak">
            <SelectInput
              id={`v-flag-${item.id}`}
              value={flag}
              onChange={(e) => setFlag(e.target.value as LabResultFlag | "")}
            >
              <option value="">—</option>
              {Object.entries(FLAG_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
          <Field id={`v-notes-${item.id}`} label="Not">
            <TextInput
              id={`v-notes-${item.id}`}
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
            />
          </Field>
          <div className="flex items-end gap-2">
            <SecondaryButton type="button" onClick={() => setEdit(false)} className="flex-1 text-xs">
              İptal
            </SecondaryButton>
            <PrimaryButton type="button" onClick={() => save.mutate()} disabled={save.isPending} className="flex-1 text-xs">
              {save.isPending ? "..." : "Kaydet"}
            </PrimaryButton>
          </div>
        </div>
      )}
    </div>
  );
}
