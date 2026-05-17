"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, CheckCircle2, PlayCircle, Save, XCircle } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  dialysisSessionDetailOptions,
  diyalizKeys,
  saveDialysisRecord,
  updateDialysisStatus,
} from "@medigt/core/diyaliz";
import type { DialysisSession } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  Textarea,
  TextInput,
} from "../../common/form-fields";
import { ACCESS_LABELS, MODALITY_LABELS } from "./list-page";

export function DialysisSessionDetailPage({ sessionId }: { sessionId: string }) {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const session = useQuery(dialysisSessionDetailOptions(branchId, sessionId));

  if (session.isLoading) {
    return (
      <DashboardLayout>
        <div className="page-shell">Yükleniyor...</div>
      </DashboardLayout>
    );
  }
  if (session.isError || !session.data) {
    return (
      <DashboardLayout>
        <div className="page-shell">
          <div className="empty-state text-[var(--critical)]">Diyaliz seansı bulunamadı.</div>
        </div>
      </DashboardLayout>
    );
  }

  const s = session.data;

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title={`Diyaliz · ${s.session_no}`}
          subtitle={`${s.patient_first_name} ${s.patient_last_name} · MRN ${s.patient_mrn} · ${MODALITY_LABELS[s.modality] ?? s.modality}`}
          actions={<HeaderActions session={s} branchId={branchId} />}
        />

        <Meta session={s} />

        <RecordSection session={s} branchId={branchId} />
      </div>
    </DashboardLayout>
  );
}

function HeaderActions({ session, branchId }: { session: DialysisSession; branchId: string }) {
  const qc = useQueryClient();
  const start = useMutation({
    mutationFn: () => updateDialysisStatus(session.id, "in_progress"),
    onSuccess: () => qc.invalidateQueries({ queryKey: diyalizKeys.all(branchId) }),
  });
  const complete = useMutation({
    mutationFn: () => updateDialysisStatus(session.id, "completed"),
    onSuccess: () => qc.invalidateQueries({ queryKey: diyalizKeys.all(branchId) }),
  });
  const cancel = useMutation({
    mutationFn: () => updateDialysisStatus(session.id, "cancelled"),
    onSuccess: () => qc.invalidateQueries({ queryKey: diyalizKeys.all(branchId) }),
  });

  if (session.status === "scheduled") {
    return (
      <div className="flex gap-2">
        <SecondaryButton type="button" onClick={() => cancel.mutate()} disabled={cancel.isPending}>
          <span className="inline-flex items-center gap-1"><XCircle className="h-4 w-4" /> İptal</span>
        </SecondaryButton>
        <PrimaryButton type="button" onClick={() => start.mutate()} disabled={start.isPending}>
          <span className="inline-flex items-center gap-1">
            <PlayCircle className="h-4 w-4" /> Seansı Başlat
          </span>
        </PrimaryButton>
      </div>
    );
  }
  if (session.status === "in_progress") {
    return (
      <PrimaryButton type="button" onClick={() => complete.mutate()} disabled={complete.isPending}>
        <span className="inline-flex items-center gap-1">
          <CheckCircle2 className="h-4 w-4" /> Seansı Tamamla
        </span>
      </PrimaryButton>
    );
  }
  return null;
}

function Meta({ session }: { session: DialysisSession }) {
  return (
    <div className="grid grid-cols-2 gap-3 rounded-md border border-border bg-card p-3 text-sm sm:grid-cols-4">
      <Cell label="Planlanan" value={new Date(session.scheduled_at).toLocaleString("tr-TR")} />
      <Cell label="Süre" value={`${session.duration_minutes} dk`} />
      <Cell label="Modalite" value={MODALITY_LABELS[session.modality] ?? session.modality} />
      <Cell label="Damar yolu" value={ACCESS_LABELS[session.vascular_access] ?? session.vascular_access} />
      {session.machine_name && (
        <Cell label="Cihaz" value={`${session.machine_name} (${session.machine_code})`} />
      )}
      {session.dialyzer_type && <Cell label="Diyalizör" value={session.dialyzer_type} />}
      {session.dry_weight_kg !== undefined && session.dry_weight_kg !== null && (
        <Cell label="Kuru ağırlık" value={`${session.dry_weight_kg} kg`} />
      )}
      {session.ultrafiltration_target_ml !== undefined && session.ultrafiltration_target_ml !== null && (
        <Cell label="Hedef UF" value={`${session.ultrafiltration_target_ml} ml`} />
      )}
      <Cell label="Durum" value={session.status} />
      {session.started_at && (
        <Cell label="Başladı" value={new Date(session.started_at).toLocaleString("tr-TR")} />
      )}
      {session.ended_at && (
        <Cell label="Bitti" value={new Date(session.ended_at).toLocaleString("tr-TR")} />
      )}
      {session.anticoagulant && (
        <div className="col-span-full">
          <div className="text-xs text-muted-foreground">Antikoagülasyon</div>
          <div className="font-medium">{session.anticoagulant}</div>
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

function RecordSection({ session, branchId }: { session: DialysisSession; branchId: string }) {
  const qc = useQueryClient();
  const [preWeight, setPreWeight] = useState(session.pre_weight_kg?.toString() ?? "");
  const [preSys, setPreSys] = useState(session.pre_systolic_bp?.toString() ?? "");
  const [preDia, setPreDia] = useState(session.pre_diastolic_bp?.toString() ?? "");
  const [postWeight, setPostWeight] = useState(session.post_weight_kg?.toString() ?? "");
  const [postSys, setPostSys] = useState(session.post_systolic_bp?.toString() ?? "");
  const [postDia, setPostDia] = useState(session.post_diastolic_bp?.toString() ?? "");
  const [uf, setUf] = useState(session.actual_ultrafiltration_ml?.toString() ?? "");
  const [complications, setComplications] = useState(session.complications ?? "");
  const [notes, setNotes] = useState(session.session_notes ?? "");

  useEffect(() => {
    setPreWeight(session.pre_weight_kg?.toString() ?? "");
    setPreSys(session.pre_systolic_bp?.toString() ?? "");
    setPreDia(session.pre_diastolic_bp?.toString() ?? "");
    setPostWeight(session.post_weight_kg?.toString() ?? "");
    setPostSys(session.post_systolic_bp?.toString() ?? "");
    setPostDia(session.post_diastolic_bp?.toString() ?? "");
    setUf(session.actual_ultrafiltration_ml?.toString() ?? "");
    setComplications(session.complications ?? "");
    setNotes(session.session_notes ?? "");
  }, [
    session.pre_weight_kg, session.pre_systolic_bp, session.pre_diastolic_bp,
    session.post_weight_kg, session.post_systolic_bp, session.post_diastolic_bp,
    session.actual_ultrafiltration_ml, session.complications, session.session_notes,
  ]);

  const save = useMutation({
    mutationFn: () =>
      saveDialysisRecord(session.id, {
        pre_weight_kg: preWeight ? Number(preWeight) : undefined,
        pre_systolic_bp: preSys ? Number(preSys) : undefined,
        pre_diastolic_bp: preDia ? Number(preDia) : undefined,
        post_weight_kg: postWeight ? Number(postWeight) : undefined,
        post_systolic_bp: postSys ? Number(postSys) : undefined,
        post_diastolic_bp: postDia ? Number(postDia) : undefined,
        actual_ultrafiltration_ml: uf ? Number(uf) : undefined,
        complications: complications.trim() || undefined,
        session_notes: notes.trim() || undefined,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: diyalizKeys.all(branchId) }),
  });

  const readOnly = session.status === "cancelled";

  return (
    <section className="space-y-3 rounded-lg border border-border bg-card p-4">
      <h2 className="flex items-center gap-2 text-sm font-semibold">
        <Activity className="h-4 w-4" /> Seans Kaydı
      </h2>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <fieldset className="rounded-md border border-border p-3">
          <legend className="px-1 text-xs font-medium text-muted-foreground">Pre-diyaliz</legend>
          <div className="grid grid-cols-3 gap-2">
            <Field id="pre-w" label="Ağırlık (kg)">
              <TextInput id="pre-w" type="number" step="0.1" readOnly={readOnly}
                value={preWeight} onChange={(e) => setPreWeight(e.target.value)} />
            </Field>
            <Field id="pre-sys" label="Sistolik">
              <TextInput id="pre-sys" type="number" readOnly={readOnly}
                value={preSys} onChange={(e) => setPreSys(e.target.value)} />
            </Field>
            <Field id="pre-dia" label="Diyastolik">
              <TextInput id="pre-dia" type="number" readOnly={readOnly}
                value={preDia} onChange={(e) => setPreDia(e.target.value)} />
            </Field>
          </div>
        </fieldset>

        <fieldset className="rounded-md border border-border p-3">
          <legend className="px-1 text-xs font-medium text-muted-foreground">Post-diyaliz</legend>
          <div className="grid grid-cols-3 gap-2">
            <Field id="post-w" label="Ağırlık (kg)">
              <TextInput id="post-w" type="number" step="0.1" readOnly={readOnly}
                value={postWeight} onChange={(e) => setPostWeight(e.target.value)} />
            </Field>
            <Field id="post-sys" label="Sistolik">
              <TextInput id="post-sys" type="number" readOnly={readOnly}
                value={postSys} onChange={(e) => setPostSys(e.target.value)} />
            </Field>
            <Field id="post-dia" label="Diyastolik">
              <TextInput id="post-dia" type="number" readOnly={readOnly}
                value={postDia} onChange={(e) => setPostDia(e.target.value)} />
            </Field>
          </div>
        </fieldset>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Field id="uf-act" label="Gerçek UF (ml)">
          <TextInput id="uf-act" type="number" readOnly={readOnly}
            value={uf} onChange={(e) => setUf(e.target.value)} />
        </Field>
        <div className="sm:col-span-2">
          <Field id="complic" label="Komplikasyon">
            <Textarea id="complic" rows={2} readOnly={readOnly}
              value={complications}
              onChange={(e) => setComplications(e.target.value)}
              placeholder="Hipotansiyon, kramp, vb. Yoksa boş bırakın." />
          </Field>
        </div>
      </div>

      <Field id="notes" label="Seans notları">
        <Textarea id="notes" rows={4} readOnly={readOnly}
          value={notes} onChange={(e) => setNotes(e.target.value)}
          placeholder="Akış, ilave ilaç, sorunlar..." />
      </Field>

      {!readOnly && (
        <div className="flex justify-end">
          {save.isSuccess && <span className="self-center mr-2 text-xs text-muted-foreground">Kaydedildi.</span>}
          <PrimaryButton type="button" onClick={() => save.mutate()} disabled={save.isPending}>
            <span className="inline-flex items-center gap-1">
              <Save className="h-4 w-4" /> {save.isPending ? "Kaydediliyor..." : "Kaydı kaydet"}
            </span>
          </PrimaryButton>
        </div>
      )}
    </section>
  );
}
