"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, ExternalLink, Eye, FileText, ImageIcon, Save } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  orderImageReferencesOptions,
  radOrderDetailOptions,
  radyolojiKeys,
  saveRadReport,
  updateRadOrderStatus,
  verifyRadReport,
} from "@medigt/core/radyoloji";
import type { RadiologyOrder } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  Textarea,
  TextInput,
} from "../../common/form-fields";
import { SideSheet } from "../../common/side-sheet";

export function RadyolojiOrderDetailPage({ orderId }: { orderId: string }) {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const order = useQuery(radOrderDetailOptions(branchId, orderId));

  if (order.isLoading) {
    return <DashboardLayout><div className="page-shell">Yükleniyor...</div></DashboardLayout>;
  }
  if (order.isError || !order.data) {
    return (
      <DashboardLayout>
        <div className="page-shell">
          <div className="empty-state text-[var(--critical)]">Radyoloji isteği bulunamadı.</div>
        </div>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title={`Görüntüleme · ${order.data.order_no}`}
          subtitle={`${order.data.patient_first_name} ${order.data.patient_last_name} · MRN ${order.data.patient_mrn}`}
          actions={<HeaderActions order={order.data} branchId={branchId} />}
        />

        <OrderMeta order={order.data} />

        <ReportSection order={order.data} branchId={branchId} />
      </div>
    </DashboardLayout>
  );
}

function OrderMeta({ order }: { order: RadiologyOrder }) {
  return (
    <div className="grid grid-cols-2 gap-3 rounded-md border border-border bg-card p-3 text-sm sm:grid-cols-4">
      <div>
        <div className="text-xs text-muted-foreground">Tetkik</div>
        <div className="font-medium">{order.procedure_name}</div>
        <div className="text-xs text-muted-foreground">
          {order.modality}{order.body_region && ` · ${order.body_region}`}
        </div>
      </div>
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
        <div className="text-xs text-muted-foreground">Öncelik · Durum</div>
        <div className={"font-medium " + (order.priority === "stat" ? "text-[var(--critical)]" : "")}>
          {order.priority.toUpperCase()} · {order.status}
        </div>
      </div>
      {order.clinical_indication && (
        <div className="col-span-full">
          <div className="text-xs text-muted-foreground">Klinik gerekçe</div>
          <div className="font-medium">{order.clinical_indication}</div>
        </div>
      )}
      {order.clinical_question && (
        <div className="col-span-full">
          <div className="text-xs text-muted-foreground">Klinik soru</div>
          <div className="font-medium">{order.clinical_question}</div>
        </div>
      )}
    </div>
  );
}

function HeaderActions({ order, branchId }: { order: RadiologyOrder; branchId: string }) {
  const qc = useQueryClient();
  const acquire = useMutation({
    mutationFn: () => updateRadOrderStatus(order.id, "acquired"),
    onSuccess: () => qc.invalidateQueries({ queryKey: radyolojiKeys.all(branchId) }),
  });
  const verify = useMutation({
    mutationFn: () => verifyRadReport(order.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: radyolojiKeys.all(branchId) }),
  });
  const [viewerOpen, setViewerOpen] = useState(false);

  return (
    <div className="flex flex-wrap gap-2">
      <SecondaryButton type="button" onClick={() => setViewerOpen(true)}>
        <span className="inline-flex items-center gap-1">
          <Eye className="h-4 w-4" /> Görüntüleri Görüntüle
        </span>
      </SecondaryButton>
      {(order.status === "ordered" || order.status === "scheduled" || order.status === "in_progress") && (
        <PrimaryButton type="button" onClick={() => acquire.mutate()} disabled={acquire.isPending}>
          <span className="inline-flex items-center gap-1">
            <ImageIcon className="h-4 w-4" /> Çekim Tamamlandı
          </span>
        </PrimaryButton>
      )}
      {order.status === "reported" && (
        <PrimaryButton type="button" onClick={() => verify.mutate()} disabled={verify.isPending}>
          <span className="inline-flex items-center gap-1">
            <CheckCircle2 className="h-4 w-4" /> Raporu Onayla
          </span>
        </PrimaryButton>
      )}

      {viewerOpen && (
        <DicomViewerDrawer
          orderId={order.id}
          orderNo={order.order_no}
          branchId={branchId}
          onClose={() => setViewerOpen(false)}
        />
      )}
    </div>
  );
}

function ReportSection({ order, branchId }: { order: RadiologyOrder; branchId: string }) {
  const qc = useQueryClient();
  const [findings, setFindings] = useState(order.findings ?? "");
  const [impression, setImpression] = useState(order.impression ?? "");
  const [recommendations, setRecommendations] = useState(order.recommendations ?? "");
  const [pacsUid, setPacsUid] = useState(order.pacs_study_uid ?? "");
  const [pacsAcc, setPacsAcc] = useState(order.pacs_accession_number ?? "");

  useEffect(() => {
    setFindings(order.findings ?? "");
    setImpression(order.impression ?? "");
    setRecommendations(order.recommendations ?? "");
    setPacsUid(order.pacs_study_uid ?? "");
    setPacsAcc(order.pacs_accession_number ?? "");
  }, [order.findings, order.impression, order.recommendations, order.pacs_study_uid, order.pacs_accession_number]);

  const save = useMutation({
    mutationFn: () =>
      saveRadReport(order.id, {
        findings: findings.trim() || undefined,
        impression: impression.trim() || undefined,
        recommendations: recommendations.trim() || undefined,
        pacs_study_uid: pacsUid.trim() || undefined,
        pacs_accession_number: pacsAcc.trim() || undefined,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: radyolojiKeys.all(branchId) }),
  });

  const readOnly = order.status === "verified" || order.status === "cancelled";

  return (
    <section className="space-y-3 rounded-lg border border-border bg-card p-4">
      <h2 className="flex items-center gap-2 text-sm font-semibold">
        <FileText className="h-4 w-4" /> Radyoloji Raporu
      </h2>
      <Field id="rep-findings" label="Bulgular (Findings)">
        <Textarea
          id="rep-findings"
          rows={6}
          readOnly={readOnly}
          value={findings}
          onChange={(e) => setFindings(e.target.value)}
          placeholder="Akciğer parankim alanları havalı. Sinüs frenik açıklar serbest. Kalp gölgesi normal."
        />
      </Field>
      <Field id="rep-impression" label="Sonuç (Impression)">
        <Textarea
          id="rep-impression"
          rows={3}
          readOnly={readOnly}
          value={impression}
          onChange={(e) => setImpression(e.target.value)}
          placeholder="Normal akciğer grafisi."
        />
      </Field>
      <Field id="rep-rec" label="Öneriler">
        <Textarea
          id="rep-rec"
          rows={2}
          readOnly={readOnly}
          value={recommendations}
          onChange={(e) => setRecommendations(e.target.value)}
          placeholder="İleri tetkik gerekmez. Klinik takip."
        />
      </Field>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field id="rep-pacs-uid" label="PACS Study UID" hint="DICOM bağlantısı için (opsiyonel)">
          <TextInput
            id="rep-pacs-uid"
            readOnly={readOnly}
            value={pacsUid}
            onChange={(e) => setPacsUid(e.target.value)}
            placeholder="1.2.840.113619..."
          />
        </Field>
        <Field id="rep-pacs-acc" label="Accession Number">
          <TextInput
            id="rep-pacs-acc"
            readOnly={readOnly}
            value={pacsAcc}
            onChange={(e) => setPacsAcc(e.target.value)}
          />
        </Field>
      </div>

      {!readOnly && (
        <div className="flex justify-end gap-2">
          {save.isSuccess && <span className="self-center text-xs text-muted-foreground">Kaydedildi.</span>}
          <PrimaryButton type="button" onClick={() => save.mutate()} disabled={save.isPending}>
            <span className="inline-flex items-center gap-1">
              <Save className="h-4 w-4" /> {save.isPending ? "Kaydediliyor..." : "Raporu Kaydet"}
            </span>
          </PrimaryButton>
        </div>
      )}

      {order.status === "verified" && order.verified_at && (
        <div className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-900 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-200">
          Rapor {new Date(order.verified_at).toLocaleString("tr-TR")} tarihinde onaylandı.
        </div>
      )}

      <ReadOnlyBlockIfCancelled order={order} />
    </section>
  );
}

function ReadOnlyBlockIfCancelled({ order }: { order: RadiologyOrder }) {
  if (order.status !== "cancelled") return null;
  return (
    <div className="rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-900 dark:border-rose-900/50 dark:bg-rose-950/30 dark:text-rose-200">
      Bu istek iptal edilmiş.
    </div>
  );
}

// Re-export to keep imports tidy in caller files.
export { SecondaryButton };

// ---------- DICOM viewer drawer ----------

function DicomViewerDrawer({
  orderId,
  orderNo,
  branchId,
  onClose,
}: {
  orderId: string;
  orderNo: string;
  branchId: string;
  onClose: () => void;
}) {
  const refs = useQuery(orderImageReferencesOptions(branchId, orderId));
  const [pickedIdx, setPickedIdx] = useState(0);

  const list = refs.data ?? [];
  const current = list[pickedIdx];

  return (
    <SideSheet open onClose={onClose} title={`Görüntüler · ${orderNo}`}>
      <div className="space-y-3">
        {refs.isLoading && (
          <p className="text-sm text-muted-foreground">PACS taranıyor…</p>
        )}
        {!refs.isLoading && list.length === 0 && (
          <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
            Henüz PACS'a yüklenmiş görüntü yok. Yeni istek oluşturulduktan sonra
            arka plan görevi study UID atar; bu pencere otomatik yenilenir
            (~3 sn).
          </div>
        )}

        {list.length > 1 && (
          <div className="flex flex-wrap gap-1">
            {list.map((r, i) => (
              <button
                key={r.id}
                type="button"
                onClick={() => setPickedIdx(i)}
                className={
                  "rounded-md border px-2 py-1 text-xs " +
                  (i === pickedIdx
                    ? "border-primary bg-primary/10"
                    : "border-border bg-background hover:bg-muted")
                }
              >
                {r.modality} · {r.study_instance_uid.slice(-8)}
              </button>
            ))}
          </div>
        )}

        {current && (
          <>
            <div className="space-y-1 rounded-md border border-border bg-muted/40 p-3 text-xs">
              <Row label="Modalite" value={current.modality} />
              <Row label="Study UID" value={current.study_instance_uid} mono />
              {current.series_instance_uid && (
                <Row label="Series UID" value={current.series_instance_uid} mono />
              )}
              {current.study_date && (
                <Row
                  label="Tarih"
                  value={new Date(current.study_date).toLocaleString("tr-TR")}
                />
              )}
              {current.instance_count > 0 && (
                <Row label="Görüntü sayısı" value={String(current.instance_count)} />
              )}
            </div>

            <div className="rounded-md border border-border bg-card p-2">
              <iframe
                src={current.viewer_url}
                title={`OHIF Viewer ${current.study_instance_uid}`}
                className="aspect-video w-full rounded-md border border-border bg-black"
                allow="fullscreen"
              />
              <a
                href={current.viewer_url}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-2 inline-flex items-center gap-1 text-xs text-muted-foreground hover:underline"
              >
                <ExternalLink className="h-3 w-3" /> Yeni sekmede aç
              </a>
            </div>

            <p className="text-xs text-muted-foreground">
              Üretim ortamında bu iframe hastanenin OHIF kurulumuna bağlanır;
              mock UID'ler kamu demo sunucusunda çözümlenmeyebilir.
            </p>
          </>
        )}
      </div>
    </SideSheet>
  );
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <span className="text-muted-foreground">{label}</span>
      <span className={mono ? "break-all font-mono text-[10px]" : ""}>{value}</span>
    </div>
  );
}
