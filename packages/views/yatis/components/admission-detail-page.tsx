"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRightLeft, LogOut, Network } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  admissionADTMessagesOptions,
  admissionDetailOptions,
  admissionTransfersOptions,
  bedMapOptions,
  discharge,
  transferAdmission,
  yatisKeys,
  type ADTOutboundMessage,
} from "@medigt/core/yatis";
import type { Admission, BedMapEntry, DischargeKind } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { SideSheet } from "../../common/side-sheet";
import { MARSheet } from "../../mar";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
} from "../../common/form-fields";

const KIND_LABELS: Record<DischargeKind, string> = {
  home: "Evde takip",
  home_with_help: "Evde sağlık hizmeti ile",
  referred: "Başka kuruma sevk",
  against_advice: "Önerilere rağmen taburcu",
  left_without_notice: "Haber vermeden ayrıldı",
  transferred: "İç transfer",
  expired: "Ex",
};

export function AdmissionDetailPage({ admissionId }: { admissionId: string }) {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const admission = useQuery(admissionDetailOptions(branchId, admissionId));
  const transfers = useQuery(admissionTransfersOptions(admissionId));
  const [transferOpen, setTransferOpen] = useState(false);
  const [dischargeOpen, setDischargeOpen] = useState(false);

  if (admission.isLoading) {
    return <DashboardLayout><div className="page-shell">Yükleniyor...</div></DashboardLayout>;
  }
  if (admission.isError || !admission.data) {
    return (
      <DashboardLayout>
        <div className="page-shell">
          <div className="empty-state text-[var(--critical)]">Yatış bulunamadı.</div>
        </div>
      </DashboardLayout>
    );
  }

  const a = admission.data;
  const isActive = a.status === "active";

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title={`Yatış · ${a.admission_no}`}
          subtitle={`${a.patient_first_name} ${a.patient_last_name} · MRN ${a.patient_mrn}`}
          actions={
            isActive ? (
              <div className="flex gap-2">
                <SecondaryButton type="button" onClick={() => setTransferOpen(true)}>
                  <span className="inline-flex items-center gap-1">
                    <ArrowRightLeft className="h-4 w-4" /> Transfer
                  </span>
                </SecondaryButton>
                <PrimaryButton type="button" onClick={() => setDischargeOpen(true)}>
                  <span className="inline-flex items-center gap-1">
                    <LogOut className="h-4 w-4" /> Taburcu
                  </span>
                </PrimaryButton>
              </div>
            ) : null
          }
        />

        <Meta admission={a} />

        <section className="space-y-2">
          <h2 className="text-sm font-semibold text-muted-foreground">Yatak Hareketleri</h2>
          {transfers.isLoading ? (
            <div className="empty-state">Yükleniyor...</div>
          ) : (transfers.data ?? []).length === 0 ? (
            <div className="empty-state">Hiç transfer yok.</div>
          ) : (
            <ul className="space-y-1">
              {(transfers.data ?? []).map((t) => (
                <li key={t.id} className="rounded-md border border-border p-3 text-sm">
                  <div className="flex items-center justify-between">
                    <div>
                      <span className="font-medium">
                        {t.from_bed_code && t.from_ward_name
                          ? `${t.from_ward_name} · ${t.from_bed_code}`
                          : "(ilk yerleşim)"}
                      </span>
                      <span className="mx-2 text-muted-foreground">→</span>
                      <span className="font-medium">
                        {t.to_ward_name} · {t.to_bed_code}
                      </span>
                    </div>
                    <span className="text-xs text-muted-foreground">
                      {new Date(t.transferred_at).toLocaleString("tr-TR")}
                    </span>
                  </div>
                  {t.reason && <div className="mt-1 text-xs italic text-muted-foreground">{t.reason}</div>}
                </li>
              ))}
            </ul>
          )}
        </section>

        <ADTMessagesSection admissionId={admissionId} />

        <MARSheet admissionId={admissionId} />
      </div>

      <TransferSheet
        open={transferOpen}
        onClose={() => setTransferOpen(false)}
        admission={a}
        branchId={branchId}
      />
      <DischargeSheet
        open={dischargeOpen}
        onClose={() => setDischargeOpen(false)}
        admission={a}
        branchId={branchId}
      />
    </DashboardLayout>
  );
}

function Meta({ admission }: { admission: Admission }) {
  return (
    <div className="grid grid-cols-2 gap-3 rounded-md border border-border bg-card p-3 text-sm sm:grid-cols-4">
      <Cell label="Servis" value={`${admission.ward_name} (${admission.ward_code})`} />
      <Cell label="Yatak" value={admission.bed_code ?? "—"} />
      <Cell
        label="Yatış Hekimi"
        value={
          admission.doctor_first_name
            ? `${admission.doctor_title ? admission.doctor_title + " " : ""}${admission.doctor_first_name} ${admission.doctor_last_name}`
            : "—"
        }
      />
      <Cell label="Tür" value={admission.kind} />
      <Cell label="Yatış" value={new Date(admission.admitted_at).toLocaleString("tr-TR")} />
      {admission.discharged_at && (
        <>
          <Cell label="Taburcu" value={new Date(admission.discharged_at).toLocaleString("tr-TR")} />
          <Cell label="Taburcu türü" value={admission.discharge_kind ? KIND_LABELS[admission.discharge_kind] : "—"} />
        </>
      )}
      {admission.chief_complaint && (
        <div className="col-span-full">
          <div className="text-xs text-muted-foreground">Şikayet</div>
          <div className="font-medium">{admission.chief_complaint}</div>
        </div>
      )}
      {admission.admission_diagnosis && (
        <div className="col-span-full">
          <div className="text-xs text-muted-foreground">Yatış tanısı</div>
          <div className="font-medium">{admission.admission_diagnosis}</div>
        </div>
      )}
      {admission.discharge_summary && (
        <div className="col-span-full">
          <div className="text-xs text-muted-foreground">Taburcu özeti</div>
          <div className="rounded-md bg-muted/40 p-2">{admission.discharge_summary}</div>
        </div>
      )}
    </div>
  );
}

function Cell({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="font-medium">{value}</div>
    </div>
  );
}

function TransferSheet({
  open,
  onClose,
  admission,
  branchId,
}: {
  open: boolean;
  onClose: () => void;
  admission: Admission;
  branchId: string;
}) {
  const qc = useQueryClient();
  const bedMap = useQuery(bedMapOptions(branchId));
  const [bedId, setBedId] = useState("");
  const [reason, setReason] = useState("");

  const transfer = useMutation({
    mutationFn: () => transferAdmission(admission.id, { to_bed_id: bedId, reason: reason.trim() || undefined }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: yatisKeys.all(branchId) });
      setBedId("");
      setReason("");
      onClose();
    },
  });

  const freeBeds = (bedMap.data ?? []).filter(
    (e: BedMapEntry) => e.bed.status === "free" && e.bed.is_active && e.bed.id !== admission.bed_id,
  );

  return (
    <SideSheet open={open} onClose={onClose} title="Yatak Transferi">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); transfer.mutate(); }}>
        <div className="rounded-md border border-border bg-muted/40 p-3 text-sm">
          <div className="text-xs text-muted-foreground">Mevcut</div>
          <div className="font-medium">
            {admission.ward_name}{admission.bed_code ? ` · Yatak ${admission.bed_code}` : " · yatak atanmamış"}
          </div>
        </div>

        <Field id="t-bed" label="Hedef yatak" required hint="Yalnızca boş yataklar listelenir.">
          <SelectInput id="t-bed" required value={bedId} onChange={(e) => setBedId(e.target.value)}>
            <option value="">— Seçiniz —</option>
            {freeBeds.map((e: BedMapEntry) => (
              <option key={e.bed.id} value={e.bed.id}>
                {e.ward_name} · {e.bed.code}{e.bed.kind !== "standard" && ` (${e.bed.kind})`}
              </option>
            ))}
          </SelectInput>
        </Field>

        <Field id="t-reason" label="Sebep">
          <Textarea
            id="t-reason"
            rows={2}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Örn. Yoğun bakımdan servise düşüş"
          />
        </Field>

        {transfer.isError && <p className="text-sm text-[var(--critical)]">Transfer başarısız.</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={transfer.isPending || !bedId}>
            {transfer.isPending ? "Aktarılıyor..." : "Transfer et"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function DischargeSheet({
  open,
  onClose,
  admission,
  branchId,
}: {
  open: boolean;
  onClose: () => void;
  admission: Admission;
  branchId: string;
}) {
  const qc = useQueryClient();
  const [kind, setKind] = useState<DischargeKind>("home");
  const [summary, setSummary] = useState("");

  const do_ = useMutation({
    mutationFn: () => discharge(admission.id, { kind, summary: summary.trim() || undefined }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: yatisKeys.all(branchId) });
      onClose();
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Taburcu">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); do_.mutate(); }}>
        <Field id="d-kind" label="Taburcu türü" required>
          <SelectInput id="d-kind" value={kind} onChange={(e) => setKind(e.target.value as DischargeKind)}>
            {Object.entries(KIND_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>

        <Field id="d-summary" label="Taburcu özeti">
          <Textarea
            id="d-summary"
            rows={5}
            value={summary}
            onChange={(e) => setSummary(e.target.value)}
            placeholder="Klinik seyir, tedavi, çıkış tedavisi, öneriler..."
          />
        </Field>

        {do_.isError && <p className="text-sm text-[var(--critical)]">Taburcu başarısız.</p>}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">Vazgeç</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={do_.isPending}>
            {do_.isPending ? "Taburcu ediliyor..." : "Taburcu et"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

// ---------- HL7 ADT outbound messages ----------
//
// Every successful admit/transfer/discharge enqueues a row in
// hl7_outbound_message. This section surfaces them per-admission so ops
// can confirm downstream peers received the events — and replay
// manually if dead.

const ADT_STATUS_COLORS: Record<ADTOutboundMessage["status"], string> = {
  pending: "bg-slate-200 text-slate-800 dark:bg-slate-700/60 dark:text-slate-200",
  in_flight: "bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-300",
  sent: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200",
  failed: "bg-amber-100 text-amber-900 dark:bg-amber-950/40 dark:text-amber-200",
  dead: "bg-rose-100 text-rose-800 dark:bg-rose-950/40 dark:text-rose-200",
};

const ADT_EVENT_LABELS: Record<ADTOutboundMessage["event_type"], string> = {
  A01: "Yatış (A01)",
  A02: "Transfer (A02)",
  A03: "Taburcu (A03)",
  A04: "Kabul (A04)",
  A08: "Bilgi güncelleme (A08)",
};

function ADTMessagesSection({ admissionId }: { admissionId: string }) {
  const msgs = useQuery(admissionADTMessagesOptions(admissionId));
  const [openId, setOpenId] = useState<string | null>(null);

  // Hide entirely when there's nothing to show. Most admissions will
  // have at least one row once they pass through admit.
  if ((msgs.data ?? []).length === 0) return null;

  return (
    <section className="space-y-2">
      <h2 className="flex items-center gap-2 text-sm font-semibold text-muted-foreground">
        <Network className="h-4 w-4" /> HL7 ADT mesajları
      </h2>
      <ul className="space-y-1">
        {(msgs.data ?? []).map((m) => {
          const expanded = openId === m.id;
          return (
            <li key={m.id} className="rounded-md border border-border p-3 text-sm">
              <button
                type="button"
                onClick={() => setOpenId(expanded ? null : m.id)}
                className="flex w-full items-center justify-between gap-3 text-left"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{ADT_EVENT_LABELS[m.event_type]}</span>
                    <span
                      className={`rounded px-2 py-0.5 text-xs ${ADT_STATUS_COLORS[m.status]}`}
                    >
                      {m.status}
                    </span>
                    {m.retry_count > 0 && (
                      <span className="text-xs text-muted-foreground">
                        deneme: {m.retry_count}
                      </span>
                    )}
                  </div>
                  <div className="mt-0.5 text-xs text-muted-foreground">
                    <code className="rounded bg-muted px-1">{m.message_control_id}</code>
                    {" · "}
                    {new Date(m.created_at).toLocaleString("tr-TR")}
                    {m.sent_at && (
                      <> · gönderildi {new Date(m.sent_at).toLocaleString("tr-TR")}</>
                    )}
                  </div>
                  {m.last_error && (
                    <div className="mt-1 text-xs text-rose-700 dark:text-rose-300">
                      {m.last_error}
                    </div>
                  )}
                </div>
                <span className="text-xs text-muted-foreground">
                  {expanded ? "kapat" : "aç"}
                </span>
              </button>
              {expanded && (
                <div className="mt-3 space-y-2 border-t border-border pt-3">
                  <div>
                    <h4 className="mb-1 text-xs font-semibold text-muted-foreground">
                      HL7 mesajı
                    </h4>
                    <pre className="max-h-48 overflow-auto rounded-md border border-border bg-muted/40 p-2 text-xs">
                      {m.raw_message.replace(/\r/g, "\n")}
                    </pre>
                  </div>
                  {m.ack_raw && (
                    <div>
                      <h4 className="mb-1 text-xs font-semibold text-muted-foreground">
                        ACK
                      </h4>
                      <pre className="max-h-32 overflow-auto rounded-md border border-border bg-muted/40 p-2 text-xs">
                        {m.ack_raw.replace(/\r/g, "\n")}
                      </pre>
                    </div>
                  )}
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
