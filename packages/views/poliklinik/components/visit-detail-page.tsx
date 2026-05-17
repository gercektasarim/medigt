"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle, FileText, Pill, Plus, ShieldCheck, Stethoscope, Trash2 } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  addDiagnosis,
  addVitals,
  completeVisit,
  createPrescription,
  deleteDiagnosis,
  diagnosisListOptions,
  poliklinikKeys,
  prescriptionListOptions,
  signPrescription,
  updateVisitNotes,
  visitDetailOptions,
  vitalsListOptions,
  type CreatePrescriptionItemInput,
} from "@medigt/core/poliklinik";
import { hastaDetailOptions } from "@medigt/core/hasta";
import { initSignature, sha256Hex } from "@medigt/core/imza";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import { formatDateTr } from "@medigt/core/utils";
import type {
  Diagnosis,
  DiagnosisKind,
  Icd10Code,
  Patient,
  Prescription,
  Visit,
} from "@medigt/core/types";
import { DashboardLayout } from "../../layout";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";
import { SignDrawer } from "../../common/sign-drawer";
import { Icd10Picker } from "../../icd10";
import { LabOrderDrawer } from "../../laboratuvar";
import { labOrderListOptions } from "@medigt/core/laboratuvar";
import { RadOrderDrawer } from "../../radyoloji";
import { radOrderListOptions } from "@medigt/core/radyoloji";

type TabKey = "anamnez" | "vital" | "tani" | "recete" | "lab" | "rad";

const TAB_LABELS: Record<TabKey, string> = {
  anamnez: "Anamnez & Muayene",
  vital: "Vital Bulgular",
  tani: "Tanılar",
  recete: "Reçeteler",
  lab: "Lab İstek",
  rad: "Görüntüleme",
};

const DX_KIND_LABELS: Record<DiagnosisKind, string> = {
  primary: "Birincil",
  secondary: "İkincil",
  provisional: "Geçici",
  differential: "Ayırıcı",
  ruled_out: "Dışlanmış",
};

export function VisitDetailPage({ visitId }: { visitId: string }) {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const visit = useQuery(visitDetailOptions(branchId, visitId));
  const patient = useQuery({
    ...hastaDetailOptions(org?.id ?? "", visit.data?.patient_id ?? ""),
    enabled: !!visit.data?.patient_id,
  });
  const [tab, setTab] = useState<TabKey>("anamnez");

  if (visit.isLoading) return <DashboardLayout><div className="page-shell">Yükleniyor...</div></DashboardLayout>;
  if (visit.isError || !visit.data) {
    return (
      <DashboardLayout>
        <div className="page-shell">
          <div className="empty-state text-[var(--critical)]">Muayene bulunamadı.</div>
        </div>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout>
      <div className="page-shell">
        <VisitHeader visit={visit.data} branchId={branchId} orgSlug={org?.slug ?? ""} branchSlug={branch?.slug ?? ""} />

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-[320px_1fr]">
          {/* --- Sol panel: hasta özet --- */}
          <PatientPanel patient={patient.data} visit={visit.data} />

          {/* --- Sağ panel: tab'lar --- */}
          <div className="space-y-4 rounded-lg border border-border bg-card">
            <div className="flex gap-1 border-b border-border px-2 pt-2">
              {(Object.keys(TAB_LABELS) as TabKey[]).map((k) => (
                <button
                  key={k}
                  type="button"
                  onClick={() => setTab(k)}
                  className={
                    "rounded-t-md px-4 py-2 text-sm font-medium " +
                    (tab === k
                      ? "bg-background text-foreground"
                      : "text-muted-foreground hover:text-foreground")
                  }
                >
                  {TAB_LABELS[k]}
                </button>
              ))}
            </div>

            <div className="p-4">
              {tab === "anamnez" && <AnamnezTab visit={visit.data} branchId={branchId} />}
              {tab === "vital" && <VitalTab visitId={visitId} />}
              {tab === "tani" && <TaniTab visitId={visitId} />}
              {tab === "recete" && <ReceteTab visit={visit.data} />}
              {tab === "lab" && (
                <LabTab
                  visitId={visitId}
                  branchId={branchId}
                  orgSlug={org?.slug ?? ""}
                  branchSlug={branch?.slug ?? ""}
                />
              )}
              {tab === "rad" && (
                <RadTab
                  visitId={visitId}
                  branchId={branchId}
                  orgSlug={org?.slug ?? ""}
                  branchSlug={branch?.slug ?? ""}
                />
              )}
            </div>
          </div>
        </div>
      </div>
    </DashboardLayout>
  );
}

function VisitHeader({
  visit,
  branchId,
  orgSlug,
  branchSlug,
}: {
  visit: Visit;
  branchId: string;
  orgSlug: string;
  branchSlug: string;
}) {
  const qc = useQueryClient();
  const nav = useNavigation();
  const complete = useMutation({
    mutationFn: () => completeVisit(visit.id),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: poliklinikKeys.all(branchId) });
      nav.replace(paths.hospital(orgSlug).branch(branchSlug).poliklinik.queue());
    },
  });

  return (
    <div className="flex flex-wrap items-start justify-between gap-3 border-b border-border pb-3">
      <div>
        <h1 className="page-title flex items-center gap-2">
          <Stethoscope className="h-5 w-5" />
          Muayene · {visit.patient_first_name ?? "Hasta"} {visit.patient_last_name ?? ""}
        </h1>
        <p className="page-subtitle">
          Başladı: {new Date(visit.started_at).toLocaleString("tr-TR")} ·
          {" "}
          {visit.status === "in_progress" ? "Devam ediyor" : visit.status === "completed" ? "Tamamlandı" : "İptal"}
        </p>
      </div>
      {visit.status === "in_progress" && (
        <PrimaryButton
          type="button"
          onClick={() => complete.mutate()}
          disabled={complete.isPending}
        >
          <span className="inline-flex items-center gap-1.5">
            <CheckCircle className="h-4 w-4" />
            {complete.isPending ? "Tamamlanıyor..." : "Muayeneyi Tamamla"}
          </span>
        </PrimaryButton>
      )}
    </div>
  );
}

function PatientPanel({ patient, visit }: { patient: Patient | undefined; visit: Visit }) {
  return (
    <aside className="space-y-3 rounded-lg border border-border bg-card p-4 text-sm">
      <div>
        <div className="text-xs uppercase tracking-wide text-muted-foreground">Hasta</div>
        <div className="mt-1 text-base font-semibold">
          {patient ? `${patient.first_name} ${patient.last_name}` : `${visit.patient_first_name} ${visit.patient_last_name}`}
        </div>
        {patient?.mrn && (
          <div className="text-xs text-muted-foreground">MRN {patient.mrn}</div>
        )}
      </div>

      {patient && (
        <dl className="grid grid-cols-2 gap-y-1.5 text-sm">
          {patient.birth_date && (
            <>
              <dt className="text-muted-foreground">Doğum</dt>
              <dd>{formatDateTr(patient.birth_date)}</dd>
            </>
          )}
          {patient.gender && patient.gender !== "unknown" && (
            <>
              <dt className="text-muted-foreground">Cinsiyet</dt>
              <dd>{patient.gender === "male" ? "Erkek" : "Kadın"}</dd>
            </>
          )}
          {patient.blood_type && patient.blood_type !== "unknown" && (
            <>
              <dt className="text-muted-foreground">Kan</dt>
              <dd className="font-mono">{patient.blood_type.replace("_", " ")}</dd>
            </>
          )}
          {patient.identifier_masked && (
            <>
              <dt className="text-muted-foreground">{patient.identifier_kind === "tc" ? "TC" : "Kimlik"}</dt>
              <dd className="font-mono">{patient.identifier_masked}</dd>
            </>
          )}
          {patient.phone && (
            <>
              <dt className="text-muted-foreground">Telefon</dt>
              <dd>{patient.phone}</dd>
            </>
          )}
        </dl>
      )}

      {visit.doctor_first_name && (
        <div className="border-t border-border pt-3">
          <div className="text-xs uppercase tracking-wide text-muted-foreground">Doktor</div>
          <div className="mt-1 font-medium">
            {visit.doctor_title ? visit.doctor_title + " " : ""}
            {visit.doctor_first_name} {visit.doctor_last_name}
          </div>
        </div>
      )}
    </aside>
  );
}

// ---------- ANAMNEZ TAB ----------

function AnamnezTab({ visit, branchId }: { visit: Visit; branchId: string }) {
  const qc = useQueryClient();
  const [form, setForm] = useState({
    chief_complaint: visit.chief_complaint ?? "",
    history_of_present_illness: visit.history_of_present_illness ?? "",
    examination_findings: visit.examination_findings ?? "",
    treatment_plan: visit.treatment_plan ?? "",
  });

  // Sync local form when the underlying visit changes (e.g. after a mutation).
  useEffect(() => {
    setForm({
      chief_complaint: visit.chief_complaint ?? "",
      history_of_present_illness: visit.history_of_present_illness ?? "",
      examination_findings: visit.examination_findings ?? "",
      treatment_plan: visit.treatment_plan ?? "",
    });
  }, [visit.id, visit.updated_at, visit.chief_complaint, visit.history_of_present_illness, visit.examination_findings, visit.treatment_plan]);

  const save = useMutation({
    mutationFn: () => updateVisitNotes(visit.id, form),
    onSuccess: () => qc.invalidateQueries({ queryKey: poliklinikKeys.detail(branchId, visit.id) }),
  });
  const readOnly = visit.status !== "in_progress";

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (!readOnly) save.mutate();
      }}
    >
      <Field id="cc" label="Şikayet (chief complaint)">
        <Textarea
          id="cc" rows={2} readOnly={readOnly}
          value={form.chief_complaint}
          onChange={(e) => setForm({ ...form, chief_complaint: e.target.value })}
          placeholder="Hasta neden geldi? (1-2 satır)"
        />
      </Field>
      <Field id="hpi" label="Mevcut hastalık hikayesi">
        <Textarea
          id="hpi" rows={5} readOnly={readOnly}
          value={form.history_of_present_illness}
          onChange={(e) => setForm({ ...form, history_of_present_illness: e.target.value })}
          placeholder="Başlangıç, süresi, eşlik eden semptomlar..."
        />
      </Field>
      <Field id="exam" label="Fizik muayene bulguları">
        <Textarea
          id="exam" rows={4} readOnly={readOnly}
          value={form.examination_findings}
          onChange={(e) => setForm({ ...form, examination_findings: e.target.value })}
          placeholder="Sistemlere göre bulgular..."
        />
      </Field>
      <Field id="plan" label="Tedavi planı">
        <Textarea
          id="plan" rows={3} readOnly={readOnly}
          value={form.treatment_plan}
          onChange={(e) => setForm({ ...form, treatment_plan: e.target.value })}
        />
      </Field>

      {!readOnly && (
        <div className="flex justify-end gap-2">
          {save.isSuccess && <span className="self-center text-xs text-muted-foreground">Kaydedildi.</span>}
          <PrimaryButton type="submit" disabled={save.isPending}>
            {save.isPending ? "Kaydediliyor..." : "Notları kaydet"}
          </PrimaryButton>
        </div>
      )}
    </form>
  );
}

// ---------- VITAL TAB ----------

function VitalTab({ visitId }: { visitId: string }) {
  const qc = useQueryClient();
  const list = useQuery(vitalsListOptions(visitId));
  const [form, setForm] = useState<{ [k: string]: string }>({});

  const add = useMutation({
    mutationFn: () =>
      addVitals(visitId, {
        systolic_bp: num(form.systolic_bp),
        diastolic_bp: num(form.diastolic_bp),
        pulse: num(form.pulse),
        temperature_c: numF(form.temperature_c),
        spo2_percent: num(form.spo2_percent),
        respiration: num(form.respiration),
        weight_kg: numF(form.weight_kg),
        height_cm: numF(form.height_cm),
        pain_score: num(form.pain_score),
        notes: form.notes?.trim() || undefined,
      }),
    onSuccess: () => {
      setForm({});
      qc.invalidateQueries({ queryKey: poliklinikKeys.vitals(visitId) });
    },
  });

  return (
    <div className="space-y-5">
      <form
        onSubmit={(e) => { e.preventDefault(); add.mutate(); }}
        className="grid grid-cols-2 gap-3 sm:grid-cols-4"
      >
        <VInput label="TA Sistolik" name="systolic_bp" suffix="mmHg" form={form} setForm={setForm} />
        <VInput label="TA Diastolik" name="diastolic_bp" suffix="mmHg" form={form} setForm={setForm} />
        <VInput label="Nabız" name="pulse" suffix="bpm" form={form} setForm={setForm} />
        <VInput label="Ateş" name="temperature_c" suffix="°C" decimal form={form} setForm={setForm} />
        <VInput label="SpO₂" name="spo2_percent" suffix="%" form={form} setForm={setForm} />
        <VInput label="Solunum" name="respiration" suffix="/dk" form={form} setForm={setForm} />
        <VInput label="Kilo" name="weight_kg" suffix="kg" decimal form={form} setForm={setForm} />
        <VInput label="Boy" name="height_cm" suffix="cm" decimal form={form} setForm={setForm} />
        <VInput label="Ağrı (0-10)" name="pain_score" form={form} setForm={setForm} />
        <div className="col-span-2 sm:col-span-4">
          <Field id="vnotes" label="Not">
            <TextInput
              id="vnotes"
              value={form.notes ?? ""}
              onChange={(e) => setForm({ ...form, notes: e.target.value })}
            />
          </Field>
        </div>
        <div className="col-span-2 sm:col-span-4 flex justify-end">
          <PrimaryButton type="submit" disabled={add.isPending}>
            <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Ölçüm ekle</span>
          </PrimaryButton>
        </div>
      </form>

      <div className="border-t border-border pt-4">
        <h3 className="mb-2 text-sm font-semibold">Bu muayenedeki ölçümler</h3>
        {list.isLoading ? (
          <p className="text-sm text-muted-foreground">Yükleniyor...</p>
        ) : (list.data ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">Henüz ölçüm yok.</p>
        ) : (
          <ul className="space-y-2">
            {(list.data ?? []).map((v) => (
              <li key={v.id} className="rounded-md border border-border p-3 text-sm">
                <div className="text-xs text-muted-foreground">
                  {new Date(v.measured_at).toLocaleString("tr-TR")}
                </div>
                <div className="mt-1 grid grid-cols-2 gap-x-4 gap-y-1 sm:grid-cols-4">
                  {showField("TA", v.systolic_bp != null && v.diastolic_bp != null ? `${v.systolic_bp}/${v.diastolic_bp} mmHg` : null)}
                  {showField("Nabız", v.pulse ? `${v.pulse} bpm` : null)}
                  {showField("Ateş", v.temperature_c ? `${v.temperature_c.toFixed(1)} °C` : null)}
                  {showField("SpO₂", v.spo2_percent ? `%${v.spo2_percent}` : null)}
                  {showField("Solunum", v.respiration ? `${v.respiration}/dk` : null)}
                  {showField("Kilo", v.weight_kg ? `${v.weight_kg} kg` : null)}
                  {showField("Boy", v.height_cm ? `${v.height_cm} cm` : null)}
                  {showField("Ağrı", v.pain_score != null ? `${v.pain_score}/10` : null)}
                </div>
                {v.notes && <div className="mt-1 text-xs italic text-muted-foreground">{v.notes}</div>}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

function showField(label: string, val: string | null) {
  if (!val) return null;
  return (
    <div key={label}>
      <span className="text-muted-foreground">{label}:</span> <span className="font-medium">{val}</span>
    </div>
  );
}

function VInput({
  label, name, suffix, decimal, form, setForm,
}: {
  label: string;
  name: string;
  suffix?: string;
  decimal?: boolean;
  form: Record<string, string>;
  setForm: (f: Record<string, string>) => void;
}) {
  return (
    <Field id={`v-${name}`} label={label}>
      <div className="flex items-center gap-1">
        <TextInput
          id={`v-${name}`}
          type="number"
          step={decimal ? "0.1" : "1"}
          value={form[name] ?? ""}
          onChange={(e) => setForm({ ...form, [name]: e.target.value })}
        />
        {suffix && <span className="text-xs text-muted-foreground">{suffix}</span>}
      </div>
    </Field>
  );
}

function num(s: string | undefined): number | undefined {
  if (!s) return undefined;
  const n = parseInt(s, 10);
  return Number.isFinite(n) ? n : undefined;
}
function numF(s: string | undefined): number | undefined {
  if (!s) return undefined;
  const n = parseFloat(s);
  return Number.isFinite(n) ? n : undefined;
}

// ---------- TANI TAB ----------

function TaniTab({ visitId }: { visitId: string }) {
  const qc = useQueryClient();
  const list = useQuery(diagnosisListOptions(visitId));
  const [picked, setPicked] = useState<Icd10Code | null>(null);
  const [kind, setKind] = useState<DiagnosisKind>("primary");
  const [notes, setNotes] = useState("");

  const add = useMutation({
    mutationFn: () =>
      addDiagnosis(visitId, {
        icd10_code: picked!.code,
        icd10_title: picked!.title_tr,
        kind,
        notes: notes.trim() || undefined,
      }),
    onSuccess: () => {
      setPicked(null); setKind("primary"); setNotes("");
      qc.invalidateQueries({ queryKey: poliklinikKeys.diagnoses(visitId) });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteDiagnosis(visitId, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: poliklinikKeys.diagnoses(visitId) }),
  });

  return (
    <div className="space-y-4">
      <div className="space-y-2 rounded-md border border-border bg-muted/30 p-3">
        <Field id="dx-pick" label="ICD-10 kodu ara">
          <Icd10Picker onPick={setPicked} />
        </Field>
        {picked && (
          <div className="flex items-center justify-between rounded-md border border-border bg-card px-3 py-2">
            <div className="text-sm">
              <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{picked.code}</code>{" "}
              <span className="font-medium">{picked.title_tr}</span>
            </div>
            <button type="button" onClick={() => setPicked(null)} className="text-xs text-muted-foreground hover:underline">
              Değiştir
            </button>
          </div>
        )}
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <Field id="dx-kind" label="Tür">
            <SelectInput id="dx-kind" value={kind} onChange={(e) => setKind(e.target.value as DiagnosisKind)}>
              {Object.entries(DX_KIND_LABELS).map(([k, label]) => (
                <option key={k} value={k}>{label}</option>
              ))}
            </SelectInput>
          </Field>
          <Field id="dx-notes" label="Not">
            <TextInput id="dx-notes" value={notes} onChange={(e) => setNotes(e.target.value)} className="sm:col-span-2" />
          </Field>
          <div className="flex items-end">
            <PrimaryButton type="button" onClick={() => add.mutate()} disabled={!picked || add.isPending} className="w-full">
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Tanıyı ekle</span>
            </PrimaryButton>
          </div>
        </div>
      </div>

      <div>
        <h3 className="mb-2 text-sm font-semibold">Konulan tanılar</h3>
        {list.isLoading ? (
          <p className="text-sm text-muted-foreground">Yükleniyor...</p>
        ) : (list.data ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">Henüz tanı eklenmemiş.</p>
        ) : (
          <ul className="space-y-1">
            {(list.data ?? []).map((d: Diagnosis) => (
              <li key={d.id} className="flex items-center justify-between gap-3 rounded-md border border-border px-3 py-2 text-sm">
                <div className="min-w-0 flex-1">
                  <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{d.icd10_code}</code>{" "}
                  <span className="font-medium">{d.icd10_title}</span>{" "}
                  <span className="text-xs text-muted-foreground">· {DX_KIND_LABELS[d.kind]}</span>
                  {d.notes && <div className="mt-0.5 text-xs italic text-muted-foreground">{d.notes}</div>}
                </div>
                <button
                  type="button"
                  onClick={() => remove.mutate(d.id)}
                  className="text-muted-foreground hover:text-[var(--critical)]"
                  title="Sil"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

// ---------- REÇETE TAB ----------

function ReceteTab({ visit }: { visit: Visit }) {
  const qc = useQueryClient();
  const list = useQuery(prescriptionListOptions(visit.id));
  const [items, setItems] = useState<CreatePrescriptionItemInput[]>([
    { medication_name: "", dosage: "", frequency: "", quantity: "", duration_days: undefined, instructions: "" },
  ]);
  const [notes, setNotes] = useState("");

  const create = useMutation({
    mutationFn: () =>
      createPrescription(visit.id, {
        notes: notes.trim() || undefined,
        items: items
          .filter((i) => i.medication_name.trim())
          .map((i) => ({
            ...i,
            medication_name: i.medication_name.trim(),
            dosage: i.dosage?.trim() || undefined,
            frequency: i.frequency?.trim() || undefined,
            quantity: i.quantity?.trim() || undefined,
            instructions: i.instructions?.trim() || undefined,
          })),
      }),
    onSuccess: () => {
      setItems([{ medication_name: "", dosage: "", frequency: "", quantity: "", duration_days: undefined, instructions: "" }]);
      setNotes("");
      qc.invalidateQueries({ queryKey: poliklinikKeys.prescriptions(visit.id) });
    },
  });

  const sign = useMutation({
    mutationFn: (args: { rxId: string; signatureId?: string }) =>
      signPrescription(args.rxId, args.signatureId),
    onSuccess: () => qc.invalidateQueries({ queryKey: poliklinikKeys.prescriptions(visit.id) }),
  });

  // e-İmza flow state: when user picks "e-İmza ile İmzala" we (1) compute
  // a SHA-256 of the rx's canonical JSON, (2) call initSignature, (3) open
  // SignDrawer with the returned signature_id. On signed: call sign() with
  // the signature id; backend cross-checks and links.
  const [pendingSigId, setPendingSigId] = useState<string | null>(null);
  const [pendingRxId, setPendingRxId] = useState<string | null>(null);
  const initEImza = useMutation({
    mutationFn: async (rx: Prescription) => {
      const canonical = JSON.stringify({
        prescription_no: rx.prescription_no,
        patient_id: rx.patient_id,
        items: rx.items.map((i) => ({
          medication: i.medication_name,
          dosage: i.dosage,
          frequency: i.frequency,
          duration_days: i.duration_days,
          quantity: i.quantity,
        })),
      });
      const hash = await sha256Hex(canonical);
      const sig = await initSignature({
        target_table: "prescription",
        target_id: rx.id,
        document_kind: "prescription",
        document_hash: hash,
      });
      return { sig, rxId: rx.id };
    },
    onSuccess: ({ sig, rxId }) => {
      setPendingSigId(sig.id);
      setPendingRxId(rxId);
    },
  });

  return (
    <div className="space-y-5">
      <div className="space-y-3 rounded-md border border-border bg-muted/30 p-3">
        <h3 className="text-sm font-semibold">Yeni reçete</h3>
        {items.map((it, idx) => (
          <div key={idx} className="space-y-2 rounded-md border border-border bg-card p-3">
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <Field id={`m-${idx}`} label={`İlaç ${idx + 1}`} required>
                <TextInput
                  id={`m-${idx}`}
                  required={idx === 0}
                  value={it.medication_name}
                  onChange={(e) => updateItem(setItems, idx, { medication_name: e.target.value })}
                  placeholder="Örn. Parol 500 mg"
                />
              </Field>
              <Field id={`d-${idx}`} label="Doz">
                <TextInput
                  id={`d-${idx}`} value={it.dosage ?? ""}
                  onChange={(e) => updateItem(setItems, idx, { dosage: e.target.value })}
                  placeholder="500 mg"
                />
              </Field>
              <Field id={`f-${idx}`} label="Sıklık">
                <TextInput
                  id={`f-${idx}`} value={it.frequency ?? ""}
                  onChange={(e) => updateItem(setItems, idx, { frequency: e.target.value })}
                  placeholder="günde 3 kez"
                />
              </Field>
              <Field id={`du-${idx}`} label="Süre (gün)">
                <TextInput
                  id={`du-${idx}`} type="number" min={1}
                  value={it.duration_days ? String(it.duration_days) : ""}
                  onChange={(e) => updateItem(setItems, idx, { duration_days: e.target.value ? parseInt(e.target.value, 10) : undefined })}
                />
              </Field>
              <Field id={`q-${idx}`} label="Miktar">
                <TextInput
                  id={`q-${idx}`} value={it.quantity ?? ""}
                  onChange={(e) => updateItem(setItems, idx, { quantity: e.target.value })}
                  placeholder="1 kutu / 30 tablet"
                />
              </Field>
              <Field id={`i-${idx}`} label="Talimat">
                <TextInput
                  id={`i-${idx}`} value={it.instructions ?? ""}
                  onChange={(e) => updateItem(setItems, idx, { instructions: e.target.value })}
                  placeholder="yemekten sonra"
                />
              </Field>
            </div>
            {items.length > 1 && (
              <div className="flex justify-end">
                <button
                  type="button"
                  onClick={() => setItems(items.filter((_, i) => i !== idx))}
                  className="text-xs text-muted-foreground hover:text-[var(--critical)]"
                >
                  İlacı kaldır
                </button>
              </div>
            )}
          </div>
        ))}
        <div className="flex flex-wrap items-end gap-3">
          <SecondaryButton
            type="button"
            onClick={() =>
              setItems([...items, { medication_name: "", dosage: "", frequency: "", quantity: "", duration_days: undefined, instructions: "" }])
            }
          >
            + Yeni ilaç satırı
          </SecondaryButton>
          <Field id="rx-notes" label="Reçete notu">
            <TextInput id="rx-notes" value={notes} onChange={(e) => setNotes(e.target.value)} />
          </Field>
          <PrimaryButton
            type="button"
            onClick={() => create.mutate()}
            disabled={create.isPending || items.filter((i) => i.medication_name.trim()).length === 0}
          >
            <span className="inline-flex items-center gap-1"><Pill className="h-4 w-4" /> Taslak reçete</span>
          </PrimaryButton>
        </div>
      </div>

      <div>
        <h3 className="mb-2 text-sm font-semibold">Bu muayenedeki reçeteler</h3>
        {list.isLoading ? (
          <p className="text-sm text-muted-foreground">Yükleniyor...</p>
        ) : (list.data ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">Henüz reçete yok.</p>
        ) : (
          <ul className="space-y-3">
            {(list.data ?? []).map((rx: Prescription) => (
              <li key={rx.id} className="rounded-md border border-border p-3">
                <div className="flex items-center justify-between">
                  <div>
                    <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{rx.prescription_no}</code>
                    <span className="ml-2 text-xs text-muted-foreground">
                      {rx.status === "draft"
                        ? "Taslak"
                        : rx.status === "signed"
                        ? `İmzalandı · ${rx.signed_at ? new Date(rx.signed_at).toLocaleString("tr-TR") : ""}`
                        : rx.status}
                    </span>
                  </div>
                  {rx.status === "draft" && (
                    <div className="flex gap-1">
                      <SecondaryButton
                        type="button"
                        onClick={() => sign.mutate({ rxId: rx.id })}
                        disabled={sign.isPending}
                        className="px-2 py-1 text-xs"
                      >
                        <span className="inline-flex items-center gap-1"><FileText className="h-3.5 w-3.5" /> İmzala</span>
                      </SecondaryButton>
                      <PrimaryButton
                        type="button"
                        onClick={() => initEImza.mutate(rx)}
                        disabled={initEImza.isPending}
                        className="px-2 py-1 text-xs"
                      >
                        <span className="inline-flex items-center gap-1"><ShieldCheck className="h-3.5 w-3.5" /> e-İmza</span>
                      </PrimaryButton>
                    </div>
                  )}
                </div>
                <ul className="mt-2 space-y-1 text-sm">
                  {rx.items.map((it) => (
                    <li key={it.id}>
                      <span className="font-medium">{it.medication_name}</span>
                      {it.dosage && ` · ${it.dosage}`}
                      {it.frequency && ` · ${it.frequency}`}
                      {it.duration_days && ` · ${it.duration_days} gün`}
                      {it.quantity && ` · ${it.quantity}`}
                      {it.instructions && <span className="text-xs italic text-muted-foreground"> ({it.instructions})</span>}
                    </li>
                  ))}
                </ul>
                {rx.notes && <p className="mt-1 text-xs italic text-muted-foreground">{rx.notes}</p>}
              </li>
            ))}
          </ul>
        )}
      </div>

      <SignDrawer
        signatureId={pendingSigId}
        title="e-İmza ile Reçeteyi Onayla"
        hint="TURKKEP mobil uygulamasında gelen onay isteğini onayladığınızda reçete imzalanır ve Sağlık Bakanlığı e-Reçete sistemine otomatik olarak gönderilir."
        onClose={() => { setPendingSigId(null); setPendingRxId(null); }}
        onSigned={(sig) => {
          if (pendingRxId) {
            sign.mutate({ rxId: pendingRxId, signatureId: sig.id });
          }
        }}
      />
    </div>
  );
}

function updateItem(
  setItems: (fn: (prev: CreatePrescriptionItemInput[]) => CreatePrescriptionItemInput[]) => void,
  idx: number,
  patch: Partial<CreatePrescriptionItemInput>,
) {
  setItems((prev) => prev.map((it, i) => (i === idx ? { ...it, ...patch } : it)));
}

// ---------- LAB TAB ----------

function LabTab({
  visitId,
  branchId,
  orgSlug,
  branchSlug,
}: {
  visitId: string;
  branchId: string;
  orgSlug: string;
  branchSlug: string;
}) {
  const nav = useNavigation();
  const orders = useQuery(labOrderListOptions(branchId, { visitId }));

  return (
    <div className="space-y-5">
      <LabOrderDrawer visitId={visitId} />

      <div className="border-t border-border pt-4">
        <h3 className="mb-2 text-sm font-semibold">Bu muayenedeki lab istekleri</h3>
        {orders.isLoading ? (
          <p className="text-sm text-muted-foreground">Yükleniyor...</p>
        ) : (orders.data ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">Henüz lab isteği yok.</p>
        ) : (
          <ul className="space-y-2">
            {(orders.data ?? []).map((o) => (
              <li
                key={o.id}
                className="flex items-center justify-between gap-3 rounded-md border border-border p-3 text-sm"
              >
                <div className="min-w-0 flex-1">
                  <div>
                    <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{o.order_no}</code>
                    <span className="ml-2 font-medium">{o.items.length} test</span>
                    <span className="ml-2 text-xs text-muted-foreground">· {o.status}</span>
                    {o.priority === "stat" && (
                      <span className="ml-2 text-xs font-semibold text-[var(--critical)]">STAT</span>
                    )}
                  </div>
                  <div className="mt-0.5 text-xs text-muted-foreground">
                    {o.items.slice(0, 6).map((it) => it.test_code).join(", ")}
                    {o.items.length > 6 && ` +${o.items.length - 6}`}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() =>
                    nav.push(paths.hospital(orgSlug).branch(branchSlug).laboratuvar.order(o.id))
                  }
                  className="rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
                >
                  Aç
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

// ---------- RAD TAB ----------

function RadTab({
  visitId,
  branchId,
  orgSlug,
  branchSlug,
}: {
  visitId: string;
  branchId: string;
  orgSlug: string;
  branchSlug: string;
}) {
  const nav = useNavigation();
  const orders = useQuery(radOrderListOptions(branchId, { visitId }));

  return (
    <div className="space-y-5">
      <RadOrderDrawer visitId={visitId} />

      <div className="border-t border-border pt-4">
        <h3 className="mb-2 text-sm font-semibold">Bu muayenedeki görüntüleme istekleri</h3>
        {orders.isLoading ? (
          <p className="text-sm text-muted-foreground">Yükleniyor...</p>
        ) : (orders.data ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">Henüz görüntüleme isteği yok.</p>
        ) : (
          <ul className="space-y-2">
            {(orders.data ?? []).map((o) => (
              <li
                key={o.id}
                className="flex items-center justify-between gap-3 rounded-md border border-border p-3 text-sm"
              >
                <div className="min-w-0 flex-1">
                  <div>
                    <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{o.order_no}</code>
                    <span className="ml-2 font-medium">{o.procedure_name}</span>
                    <span className="ml-2 text-xs text-muted-foreground">
                      {o.modality}{o.body_region && ` · ${o.body_region}`} · {o.status}
                    </span>
                    {o.priority === "stat" && (
                      <span className="ml-2 text-xs font-semibold text-[var(--critical)]">STAT</span>
                    )}
                  </div>
                  {(o.impression || o.findings) && (
                    <div className="mt-0.5 text-xs text-muted-foreground">
                      {o.impression
                        ? `Sonuç: ${o.impression.slice(0, 120)}${o.impression.length > 120 ? "..." : ""}`
                        : `Bulgular: ${o.findings!.slice(0, 120)}${o.findings!.length > 120 ? "..." : ""}`}
                    </div>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() =>
                    nav.push(paths.hospital(orgSlug).branch(branchSlug).radyoloji.exam(o.id))
                  }
                  className="rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
                >
                  Aç
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
