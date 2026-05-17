"use client";

import { useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Stethoscope } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { randevuDayOptions, randevuKeys } from "@medigt/core/randevu";
import {
  poliklinikKeys,
  startVisitFromAppointment,
  visitListOptions,
} from "@medigt/core/poliklinik";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import type { Appointment, Visit } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { PrimaryButton, SecondaryButton } from "../../common/form-fields";

function todayISO(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" });
}

export function PoliklinikQueuePage() {
  const branch = useHospitalStore((s) => s.branch);
  const org = useHospitalStore((s) => s.organization);
  const branchId = branch?.id ?? "";
  const today = todayISO();

  // Two queries: today's "arrived" appointments (waiting room) + today's
  // in-progress visits (already started, still open in a doctor's screen).
  const arrived = useQuery({
    ...randevuDayOptions(branchId, today),
    select: (rows) => rows.filter((a) => a.status === "arrived" || a.status === "scheduled"),
  });
  const inProgress = useQuery({
    ...visitListOptions(branchId, { status: "in_progress", from: today, to: today }),
  });

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Poliklinik"
          subtitle="Bekleyen hastalar ve devam eden muayeneler. Bir randevuyu muayeneye alın → doktor ekranı açılır."
        />

        <section className="space-y-3">
          <h2 className="text-sm font-semibold text-muted-foreground">
            Bekleyen ({arrived.data?.length ?? 0})
          </h2>
          {arrived.isLoading ? (
            <div className="empty-state">Yükleniyor...</div>
          ) : (arrived.data ?? []).length === 0 ? (
            <div className="empty-state">Bekleyen hasta yok.</div>
          ) : (
            <DataTable<Appointment>
              rows={arrived.data ?? []}
              rowKey={(r) => r.id}
              columns={waitingColumns(org?.slug ?? "", branch?.slug ?? "", branchId)}
            />
          )}
        </section>

        <section className="space-y-3">
          <h2 className="text-sm font-semibold text-muted-foreground">
            Devam Eden Muayeneler ({inProgress.data?.length ?? 0})
          </h2>
          {inProgress.isLoading ? (
            <div className="empty-state">Yükleniyor...</div>
          ) : (inProgress.data ?? []).length === 0 ? (
            <div className="empty-state">Açık muayene yok.</div>
          ) : (
            <DataTable<Visit>
              rows={inProgress.data ?? []}
              rowKey={(r) => r.id}
              columns={openVisitColumns(org?.slug ?? "", branch?.slug ?? "")}
            />
          )}
        </section>
      </div>
    </DashboardLayout>
  );
}

function waitingColumns(orgSlug: string, branchSlug: string, branchId: string): Column<Appointment>[] {
  return [
    {
      key: "time",
      header: "Saat",
      cell: (a) => <span className="font-mono text-sm">{formatTime(a.scheduled_at)}</span>,
    },
    {
      key: "patient",
      header: "Hasta",
      cell: (a) => (
        <div>
          <div className="font-medium">{a.patient_first_name} {a.patient_last_name}</div>
          <div className="text-xs text-muted-foreground">MRN {a.patient_mrn}</div>
        </div>
      ),
    },
    {
      key: "doctor",
      header: "Doktor",
      cell: (a) =>
        a.doctor_first_name ? (
          <span>
            {a.doctor_title ? a.doctor_title + " " : ""}
            {a.doctor_first_name} {a.doctor_last_name}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground">— atanmadı</span>
        ),
    },
    {
      key: "reason",
      header: "Şikayet",
      cell: (a) => a.reason ?? <span className="text-xs text-muted-foreground">—</span>,
    },
    {
      key: "actions",
      header: "",
      cell: (a) => <StartButton appt={a} orgSlug={orgSlug} branchSlug={branchSlug} branchId={branchId} />,
      className: "text-right",
    },
  ];
}

function openVisitColumns(orgSlug: string, branchSlug: string): Column<Visit>[] {
  return [
    {
      key: "since",
      header: "Açıldı",
      cell: (v) => <span className="font-mono text-sm">{formatTime(v.started_at)}</span>,
    },
    {
      key: "patient",
      header: "Hasta",
      cell: (v) => (
        <div>
          <div className="font-medium">{v.patient_first_name} {v.patient_last_name}</div>
          <div className="text-xs text-muted-foreground">MRN {v.patient_mrn}</div>
        </div>
      ),
    },
    {
      key: "doctor",
      header: "Doktor",
      cell: (v) =>
        v.doctor_first_name ? (
          <span>
            {v.doctor_title ? v.doctor_title + " " : ""}
            {v.doctor_first_name} {v.doctor_last_name}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        ),
    },
    {
      key: "complaint",
      header: "Şikayet",
      cell: (v) => v.chief_complaint ?? <span className="text-xs text-muted-foreground">—</span>,
    },
    {
      key: "actions",
      header: "",
      cell: (v) => (
        <OpenVisitLink visitId={v.id} orgSlug={orgSlug} branchSlug={branchSlug} />
      ),
      className: "text-right",
    },
  ];
}

function StartButton({
  appt,
  orgSlug,
  branchSlug,
  branchId,
}: {
  appt: Appointment;
  orgSlug: string;
  branchSlug: string;
  branchId: string;
}) {
  const qc = useQueryClient();
  const nav = useNavigation();
  const start = useMutation({
    mutationFn: () => startVisitFromAppointment({ appointment_id: appt.id }),
    onSuccess: async (visit) => {
      await qc.invalidateQueries({ queryKey: randevuKeys.all(branchId) });
      await qc.invalidateQueries({ queryKey: poliklinikKeys.all(branchId) });
      nav.push(paths.hospital(orgSlug).branch(branchSlug).poliklinik.visit(visit.id));
    },
  });

  return (
    <PrimaryButton
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        start.mutate();
      }}
      disabled={start.isPending}
      className="px-2 py-1 text-xs"
    >
      <span className="inline-flex items-center gap-1">
        <Stethoscope className="h-3.5 w-3.5" />
        {start.isPending ? "Açılıyor..." : "Muayeneye al"}
      </span>
    </PrimaryButton>
  );
}

function OpenVisitLink({
  visitId,
  orgSlug,
  branchSlug,
}: {
  visitId: string;
  orgSlug: string;
  branchSlug: string;
}) {
  const nav = useNavigation();
  const target = useMemo(
    () => paths.hospital(orgSlug).branch(branchSlug).poliklinik.visit(visitId),
    [orgSlug, branchSlug, visitId],
  );
  return (
    <SecondaryButton
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        nav.push(target);
      }}
      className="px-2 py-1 text-xs"
    >
      <span className="inline-flex items-center gap-1">
        Devam et <ArrowRight className="h-3.5 w-3.5" />
      </span>
    </SecondaryButton>
  );
}
