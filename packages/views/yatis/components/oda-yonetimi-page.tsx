"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  bedMapOptions,
  createBed,
  createWard,
  setBedStatus,
  wardListOptions,
  yatisKeys,
  type CreateBedInput,
  type CreateWardInput,
} from "@medigt/core/yatis";
import type { BedMapEntry, BedStatus, WardKind } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  PrimaryButton,
  SecondaryButton,
  SelectInput,
  Textarea,
  TextInput,
} from "../../common/form-fields";

const WARD_KIND_LABELS: Record<WardKind, string> = {
  general: "Genel Servis",
  icu: "Yoğun Bakım",
  ccu: "Koroner YB",
  pediatrics: "Çocuk",
  maternity: "Doğum",
  surgical: "Cerrahi",
  isolation: "İzolasyon",
  observation: "Gözlem",
};

const BED_STATUS_LABELS: Record<BedStatus, string> = {
  free: "Boş",
  occupied: "Dolu",
  reserved: "Rezerve",
  cleaning: "Temizleniyor",
  blocked: "Bakımda",
};

const BED_STATUS_COLORS: Record<BedStatus, string> = {
  free: "border-emerald-300 bg-emerald-50 dark:border-emerald-900 dark:bg-emerald-950/40",
  occupied: "border-rose-300 bg-rose-50 dark:border-rose-900 dark:bg-rose-950/40",
  reserved: "border-amber-300 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/40",
  cleaning: "border-sky-300 bg-sky-50 dark:border-sky-900 dark:bg-sky-950/40",
  blocked: "border-zinc-300 bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-900",
};

export function OdaYonetimiPage() {
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const wards = useQuery(wardListOptions(branchId));
  const bedMap = useQuery(bedMapOptions(branchId));
  const [wardOpen, setWardOpen] = useState(false);
  const [bedOpenFor, setBedOpenFor] = useState<string | null>(null);

  // Group beds by ward.
  const grouped = useMemo(() => {
    const map = new Map<string, BedMapEntry[]>();
    for (const e of bedMap.data ?? []) {
      const arr = map.get(e.ward_id) ?? [];
      arr.push(e);
      map.set(e.ward_id, arr);
    }
    return map;
  }, [bedMap.data]);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Oda & Yatak Yönetimi"
          subtitle="Servisler, yataklar ve yatak durumları. Renkler: boş = yeşil, dolu = kırmızı, temizlik = mavi."
          actions={
            <PrimaryButton type="button" onClick={() => setWardOpen(true)}>
              <span className="inline-flex items-center gap-1"><Plus className="h-4 w-4" /> Yeni Servis</span>
            </PrimaryButton>
          }
        />

        {wards.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : (wards.data ?? []).length === 0 ? (
          <div className="empty-state">Henüz servis yok. İlk servisi ekleyin.</div>
        ) : (
          <div className="space-y-6">
            {(wards.data ?? []).map((w) => {
              const beds = grouped.get(w.id) ?? [];
              return (
                <section key={w.id} className="space-y-2">
                  <div className="flex items-center justify-between gap-2">
                    <div>
                      <h2 className="text-base font-semibold">
                        {w.name}{" "}
                        <span className="text-xs font-normal text-muted-foreground">
                          {w.code} · {WARD_KIND_LABELS[w.kind] ?? w.kind}
                          {w.floor && ` · Kat ${w.floor}`}
                        </span>
                      </h2>
                      <div className="text-xs text-muted-foreground">
                        {beds.length} yatak · {beds.filter((b) => b.bed.status === "free").length} boş
                      </div>
                    </div>
                    <SecondaryButton type="button" onClick={() => setBedOpenFor(w.id)} className="px-2 py-1 text-xs">
                      + Yatak ekle
                    </SecondaryButton>
                  </div>
                  {beds.length === 0 ? (
                    <div className="empty-state">Henüz yatak yok.</div>
                  ) : (
                    <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8">
                      {beds.map((e) => (
                        <BedCard key={e.bed.id} entry={e} branchId={branchId} />
                      ))}
                    </div>
                  )}
                </section>
              );
            })}
          </div>
        )}
      </div>

      <CreateWardSheet open={wardOpen} onClose={() => setWardOpen(false)} branchId={branchId} />
      {bedOpenFor && (
        <CreateBedSheet
          wardId={bedOpenFor}
          onClose={() => setBedOpenFor(null)}
          branchId={branchId}
        />
      )}
    </DashboardLayout>
  );
}

function BedCard({ entry, branchId }: { entry: BedMapEntry; branchId: string }) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const change = useMutation({
    mutationFn: (status: BedStatus) => setBedStatus(entry.bed.id, status),
    onSuccess: async () => {
      setOpen(false);
      await qc.invalidateQueries({ queryKey: yatisKeys.all(branchId) });
    },
  });

  return (
    <div
      className={
        "relative flex flex-col rounded-md border-2 p-2 text-sm transition-shadow " +
        BED_STATUS_COLORS[entry.bed.status] +
        " hover:shadow-sm"
      }
    >
      <div className="flex items-center justify-between">
        <span className="font-mono text-base font-semibold">{entry.bed.code}</span>
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className="text-xs text-muted-foreground hover:underline"
          aria-label="Durum değiştir"
        >
          ⋯
        </button>
      </div>
      <div className="text-xs text-muted-foreground">{BED_STATUS_LABELS[entry.bed.status]}</div>
      {entry.patient_first_name && (
        <div className="mt-1 text-xs">
          <div className="font-medium">{entry.patient_first_name} {entry.patient_last_name}</div>
          <div className="text-muted-foreground">MRN {entry.patient_mrn}</div>
        </div>
      )}
      {entry.bed.kind !== "standard" && (
        <div className="mt-0.5 text-[10px] uppercase text-muted-foreground">{entry.bed.kind}</div>
      )}

      {open && (
        <div className="absolute right-0 top-7 z-10 rounded-md border border-border bg-card shadow-lg">
          <ul className="py-1 text-xs">
            {(["free", "cleaning", "blocked"] as BedStatus[]).map((s) => (
              <li key={s}>
                <button
                  type="button"
                  onClick={() => change.mutate(s)}
                  disabled={change.isPending || entry.bed.status === s}
                  className="block w-full px-3 py-1.5 text-left hover:bg-muted disabled:opacity-50"
                >
                  → {BED_STATUS_LABELS[s]}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function CreateWardSheet({ open, onClose, branchId }: { open: boolean; onClose: () => void; branchId: string }) {
  const qc = useQueryClient();
  const [form, setForm] = useState<CreateWardInput>({
    code: "", name: "", kind: "general",
  });

  const create = useMutation({
    mutationFn: () => createWard(form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: yatisKeys.all(branchId) });
      setForm({ code: "", name: "", kind: "general" });
      onClose();
    },
  });

  return (
    <SideSheet open={open} onClose={onClose} title="Yeni Servis">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="w-name" label="Servis adı" required>
          <TextInput id="w-name" required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        </Field>
        <Field id="w-code" label="Kod" required hint="Örn. DAH3, ICU1">
          <TextInput
            id="w-code"
            required
            value={form.code}
            onChange={(e) => setForm({ ...form, code: e.target.value.toUpperCase().replace(/[^A-Z0-9_-]/g, "") })}
          />
        </Field>
        <Field id="w-kind" label="Tür" required>
          <SelectInput
            id="w-kind"
            value={form.kind}
            onChange={(e) => setForm({ ...form, kind: e.target.value as WardKind })}
          >
            {Object.entries(WARD_KIND_LABELS).map(([k, label]) => (
              <option key={k} value={k}>{label}</option>
            ))}
          </SelectInput>
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field id="w-floor" label="Kat">
            <TextInput
              id="w-floor" value={form.floor ?? ""}
              onChange={(e) => setForm({ ...form, floor: e.target.value })}
            />
          </Field>
          <Field id="w-cap" label="Kapasite">
            <TextInput
              id="w-cap" type="number" min={1}
              value={form.capacity ? String(form.capacity) : ""}
              onChange={(e) => setForm({ ...form, capacity: e.target.value ? Number(e.target.value) : undefined })}
            />
          </Field>
        </div>
        <Field id="w-notes" label="Notlar">
          <Textarea
            id="w-notes" rows={2}
            value={form.notes ?? ""}
            onChange={(e) => setForm({ ...form, notes: e.target.value })}
          />
        </Field>
        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız. Kod zaten kullanılıyor olabilir.</p>}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">İptal</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || !form.name || !form.code}>
            {create.isPending ? "Kaydediliyor..." : "Servisi oluştur"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}

function CreateBedSheet({
  wardId,
  onClose,
  branchId,
}: {
  wardId: string;
  onClose: () => void;
  branchId: string;
}) {
  const qc = useQueryClient();
  const [form, setForm] = useState<CreateBedInput>({ code: "", kind: "standard" });

  const create = useMutation({
    mutationFn: () => createBed(wardId, form),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: yatisKeys.all(branchId) });
      setForm({ code: "", kind: "standard" });
      // Don't close — allow adding multiple beds in a row.
    },
  });

  return (
    <SideSheet open onClose={onClose} title="Yatak Ekle">
      <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field id="b-code" label="Yatak no" required>
          <TextInput id="b-code" required value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} placeholder="Örn. 101, ICU-3" />
        </Field>
        <Field id="b-kind" label="Yatak türü">
          <SelectInput
            id="b-kind"
            value={form.kind}
            onChange={(e) => setForm({ ...form, kind: e.target.value as CreateBedInput["kind"] })}
          >
            <option value="standard">Standart</option>
            <option value="icu">Yoğun Bakım</option>
            <option value="isolation">İzolasyon</option>
            <option value="pediatric">Çocuk</option>
            <option value="vip">VIP</option>
            <option value="observation">Gözlem</option>
          </SelectInput>
        </Field>
        {create.isError && <p className="text-sm text-[var(--critical)]">Kayıt başarısız. Yatak kodu zaten kullanılıyor olabilir.</p>}
        {create.isSuccess && <p className="text-sm text-emerald-700">Yatak eklendi. Başka yatak için tekrar kaydedin.</p>}
        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">Kapat</SecondaryButton>
          <PrimaryButton type="submit" className="flex-1" disabled={create.isPending || !form.code}>
            {create.isPending ? "Ekleniyor..." : "Ekle"}
          </PrimaryButton>
        </div>
      </form>
    </SideSheet>
  );
}
