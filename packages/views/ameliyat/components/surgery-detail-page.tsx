"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, FileText, PlayCircle, Save, XCircle } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  ameliyatKeys,
  saveOpNote,
  surgeryDetailOptions,
  updateSurgeryStatus,
} from "@medigt/core/ameliyat";
import type { Surgery } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  Textarea,
  TextInput,
} from "../../common/form-fields";

export function SurgeryDetailPage({ surgeryId }: { surgeryId: string }) {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const surgery = useQuery(surgeryDetailOptions(branchId, surgeryId));

  if (surgery.isLoading) {
    return <DashboardLayout><div className="page-shell">Yükleniyor...</div></DashboardLayout>;
  }
  if (surgery.isError || !surgery.data) {
    return (
      <DashboardLayout>
        <div className="page-shell">
          <div className="empty-state text-[var(--critical)]">Ameliyat bulunamadı.</div>
        </div>
      </DashboardLayout>
    );
  }

  const s = surgery.data;

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title={`Ameliyat · ${s.surgery_no}`}
          subtitle={`${s.patient_first_name} ${s.patient_last_name} · MRN ${s.patient_mrn} · ${s.procedure_name}`}
          actions={<HeaderActions surgery={s} branchId={branchId} />}
        />

        <Meta surgery={s} />

        <OpNoteSection surgery={s} branchId={branchId} />

        <TeamSection surgery={s} />
      </div>
    </DashboardLayout>
  );
}

function HeaderActions({ surgery, branchId }: { surgery: Surgery; branchId: string }) {
  const qc = useQueryClient();
  const start = useMutation({
    mutationFn: () => updateSurgeryStatus(surgery.id, "in_progress"),
    onSuccess: () => qc.invalidateQueries({ queryKey: ameliyatKeys.all(branchId) }),
  });
  const complete = useMutation({
    mutationFn: () => updateSurgeryStatus(surgery.id, "completed"),
    onSuccess: () => qc.invalidateQueries({ queryKey: ameliyatKeys.all(branchId) }),
  });
  const cancel = useMutation({
    mutationFn: () => updateSurgeryStatus(surgery.id, "cancelled"),
    onSuccess: () => qc.invalidateQueries({ queryKey: ameliyatKeys.all(branchId) }),
  });

  if (surgery.status === "scheduled") {
    return (
      <div className="flex gap-2">
        <SecondaryButton
          type="button"
          onClick={() => cancel.mutate()}
          disabled={cancel.isPending}
        >
          <span className="inline-flex items-center gap-1"><XCircle className="h-4 w-4" /> İptal</span>
        </SecondaryButton>
        <PrimaryButton type="button" onClick={() => start.mutate()} disabled={start.isPending}>
          <span className="inline-flex items-center gap-1">
            <PlayCircle className="h-4 w-4" /> Ameliyatı Başlat
          </span>
        </PrimaryButton>
      </div>
    );
  }
  if (surgery.status === "in_progress") {
    return (
      <PrimaryButton type="button" onClick={() => complete.mutate()} disabled={complete.isPending}>
        <span className="inline-flex items-center gap-1">
          <CheckCircle2 className="h-4 w-4" /> Ameliyatı Tamamla
        </span>
      </PrimaryButton>
    );
  }
  return null;
}

function Meta({ surgery }: { surgery: Surgery }) {
  return (
    <div className="grid grid-cols-2 gap-3 rounded-md border border-border bg-card p-3 text-sm sm:grid-cols-4">
      <Cell label="Planlanan" value={new Date(surgery.scheduled_at).toLocaleString("tr-TR")} />
      <Cell label="Süre" value={`${surgery.estimated_minutes} dk`} />
      <Cell label="Salon" value={`${surgery.operating_room_name} (${surgery.operating_room_code})`} />
      <Cell label="Anestezi" value={surgery.anesthesia_type} />
      {surgery.surgeon_first_name && (
        <Cell
          label="Birincil cerrah"
          value={`${surgery.surgeon_title ? surgery.surgeon_title + " " : ""}${surgery.surgeon_first_name} ${surgery.surgeon_last_name}`}
        />
      )}
      <Cell label="Öncelik · Durum" value={`${surgery.priority.toUpperCase()} · ${surgery.status}`} />
      {surgery.started_at && (
        <Cell label="Başladı" value={new Date(surgery.started_at).toLocaleString("tr-TR")} />
      )}
      {surgery.ended_at && (
        <Cell label="Bitti" value={new Date(surgery.ended_at).toLocaleString("tr-TR")} />
      )}
      {surgery.indication && (
        <div className="col-span-full">
          <div className="text-xs text-muted-foreground">Endikasyon</div>
          <div className="font-medium">{surgery.indication}</div>
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

function OpNoteSection({ surgery, branchId }: { surgery: Surgery; branchId: string }) {
  const qc = useQueryClient();
  const [opNote, setOpNote] = useState(surgery.op_note ?? "");
  const [complications, setComplications] = useState(surgery.complications ?? "");
  const [bloodLoss, setBloodLoss] = useState(surgery.blood_loss_ml?.toString() ?? "");
  const [specimen, setSpecimen] = useState(surgery.specimen_sent);

  useEffect(() => {
    setOpNote(surgery.op_note ?? "");
    setComplications(surgery.complications ?? "");
    setBloodLoss(surgery.blood_loss_ml?.toString() ?? "");
    setSpecimen(surgery.specimen_sent);
  }, [surgery.op_note, surgery.complications, surgery.blood_loss_ml, surgery.specimen_sent]);

  const save = useMutation({
    mutationFn: () =>
      saveOpNote(surgery.id, {
        op_note: opNote.trim() || undefined,
        complications: complications.trim() || undefined,
        blood_loss_ml: bloodLoss.trim() ? Number(bloodLoss) : undefined,
        specimen_sent: specimen,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ameliyatKeys.all(branchId) }),
  });

  const readOnly = surgery.status === "cancelled";

  return (
    <section className="space-y-3 rounded-lg border border-border bg-card p-4">
      <h2 className="flex items-center gap-2 text-sm font-semibold">
        <FileText className="h-4 w-4" /> Op-Not
      </h2>
      <Field id="op-note" label="Operasyon notu">
        <Textarea
          id="op-note"
          rows={8}
          readOnly={readOnly}
          value={opNote}
          onChange={(e) => setOpNote(e.target.value)}
          placeholder="Cerrahi prosedür, bulgular, yapılan işlem adımları, kapatma..."
        />
      </Field>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Field id="op-complic" label="Komplikasyon">
          <Textarea
            id="op-complic" rows={2} readOnly={readOnly}
            value={complications}
            onChange={(e) => setComplications(e.target.value)}
            placeholder="Yoksa boş bırakın"
          />
        </Field>
        <Field id="op-blood" label="Kan kaybı (ml)">
          <TextInput
            id="op-blood" type="number" min={0} readOnly={readOnly}
            value={bloodLoss}
            onChange={(e) => setBloodLoss(e.target.value)}
          />
        </Field>
        <div className="flex items-end">
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={specimen}
              disabled={readOnly}
              onChange={(e) => setSpecimen(e.target.checked)}
              className="h-4 w-4 rounded border-input"
            />
            Patoloji örneği gönderildi
          </label>
        </div>
      </div>

      {!readOnly && (
        <div className="flex justify-end">
          {save.isSuccess && <span className="self-center mr-2 text-xs text-muted-foreground">Kaydedildi.</span>}
          <PrimaryButton type="button" onClick={() => save.mutate()} disabled={save.isPending}>
            <span className="inline-flex items-center gap-1">
              <Save className="h-4 w-4" /> {save.isPending ? "Kaydediliyor..." : "Op-Not'u kaydet"}
            </span>
          </PrimaryButton>
        </div>
      )}
    </section>
  );
}

const ROLE_LABELS: Record<string, string> = {
  primary_surgeon: "Birincil cerrah",
  assistant: "Asistan",
  anesthesiologist: "Anestezist",
  scrub_nurse: "Yıkanmış hemşire",
  circulating_nurse: "Sirkülasyon hemşiresi",
  technician: "Teknisyen",
};

function TeamSection({ surgery }: { surgery: Surgery }) {
  if (surgery.team.length === 0) return null;
  return (
    <section className="space-y-2 rounded-lg border border-border bg-card p-4">
      <h2 className="text-sm font-semibold">Cerrahi Ekip</h2>
      <ul className="space-y-1 text-sm">
        {surgery.team.map((m, i) => (
          <li key={i} className="flex items-center justify-between">
            <span className="font-medium">{m.name}</span>
            <span className="text-xs text-muted-foreground">{ROLE_LABELS[m.role] ?? m.role}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
