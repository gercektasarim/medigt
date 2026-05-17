"use client";

import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, Printer } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { paths } from "@medigt/core/paths";
import { AppLink } from "@medigt/core/navigation";
import { zReportOptions } from "@medigt/core/vezne";
import { formatTl } from "@medigt/core/utils";
import type {
  CashMovement,
  CashMovementKind,
  PaymentMethod,
  ZReport,
} from "@medigt/core/types";
import { DashboardLayout } from "../../layout";
import { PrimaryButton } from "../../common/form-fields";

// Printable Z-Report — gün sonu kasa raporu. The page renders the full
// session breakdown in a print-friendly layout. The cashier closes the
// register first (`/vezne` → "Kasayı Kapat"), then opens this page to
// print and sign. Variance, expected vs declared balance, all kind/method
// rollups, and the full movement list are included.

const KIND_LABELS: Record<CashMovementKind, string> = {
  opening: "Açılış",
  income: "Tahsilat",
  expense: "Gider",
  refund: "İade",
  closing: "Kapanış",
  transfer_in: "Transfer Giriş",
  transfer_out: "Transfer Çıkış",
};

const METHOD_LABELS: Record<PaymentMethod, string> = {
  cash: "Nakit",
  card: "Kart",
  transfer: "Havale",
  mobile: "Mobil",
  other: "Diğer",
};

const METHOD_ORDER: PaymentMethod[] = ["cash", "card", "transfer", "mobile", "other"];

export function ZReportPage({ registerId }: { registerId: string }) {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const z = useQuery(zReportOptions(branchId, registerId));

  if (z.isLoading) {
    return (
      <DashboardLayout>
        <div className="page-shell">Yükleniyor...</div>
      </DashboardLayout>
    );
  }
  if (!z.data) {
    return (
      <DashboardLayout>
        <div className="page-shell">
          <div className="empty-state">Rapor bulunamadı.</div>
        </div>
      </DashboardLayout>
    );
  }

  const report = z.data;
  const back = paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "").vezne.root();

  return (
    <DashboardLayout>
      <div className="page-shell">
        <header className="flex items-center justify-between print:hidden">
          <AppLink
            to={back}
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="h-4 w-4" /> Vezne'ye dön
          </AppLink>
          <PrimaryButton type="button" onClick={() => window.print()}>
            <span className="inline-flex items-center gap-1">
              <Printer className="h-4 w-4" /> Yazdır
            </span>
          </PrimaryButton>
        </header>

        <article className="z-report rounded-lg border border-border bg-card p-6 print:border-0 print:p-0 print:shadow-none">
          <ReportHeader
            report={report}
            orgName={org?.name ?? ""}
            branchName={branch?.name ?? ""}
          />
          <BalanceSummary report={report} />
          <ByKindMethod report={report} />
          <MovementList movements={report.movements} />
          <Signature report={report} />
        </article>
      </div>

      {/* Print-only stylesheet — single page, no nav, no chrome */}
      <style>{`
        @media print {
          body { background: white; }
          aside, header.app-header, nav, .print\\:hidden { display: none !important; }
          .z-report { color: #111; font-size: 11pt; }
          .z-report h1, .z-report h2 { color: #111; }
        }
      `}</style>
    </DashboardLayout>
  );
}

function ReportHeader({
  report,
  orgName,
  branchName,
}: {
  report: ZReport;
  orgName: string;
  branchName: string;
}) {
  const reg = report.register;
  return (
    <header className="mb-6 border-b border-border pb-4">
      <div className="flex items-start justify-between">
        <div>
          <div className="eyebrow">Gün Sonu Kasa Raporu · Z</div>
          <h1 className="mt-1 text-2xl font-bold">{orgName}</h1>
          <div className="text-sm text-muted-foreground">{branchName}</div>
        </div>
        <div className="text-right text-xs">
          <div>
            <span className="text-muted-foreground">Kasa No:</span>{" "}
            <code className="rounded bg-muted px-1.5 py-0.5">{reg.register_no}</code>
          </div>
          <div className="mt-1 text-muted-foreground">
            Açılış: {new Date(reg.opened_at).toLocaleString("tr-TR")}
          </div>
          {reg.closed_at && (
            <div className="text-muted-foreground">
              Kapanış: {new Date(reg.closed_at).toLocaleString("tr-TR")}
            </div>
          )}
          <div className="mt-1">
            <span className="text-muted-foreground">Kasiyer:</span>{" "}
            <span className="font-medium">{reg.cashier_name}</span>
          </div>
        </div>
      </div>
    </header>
  );
}

function BalanceSummary({ report }: { report: ZReport }) {
  const reg = report.register;
  const variance = report.variance;
  return (
    <section className="mb-6">
      <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
        Nakit Bakiye Özeti
      </h2>
      <table className="w-full border-collapse text-sm">
        <tbody>
          <tr className="border-b border-border">
            <td className="py-2 text-muted-foreground">Açılış bakiyesi</td>
            <td className="py-2 text-right font-mono">{formatTl(reg.opening_balance)}</td>
          </tr>
          <tr className="border-b border-border">
            <td className="py-2 text-muted-foreground">Tahsilat (+)</td>
            <td className="py-2 text-right font-mono text-emerald-700 dark:text-emerald-300">
              +{formatTl(report.total_income)}
            </td>
          </tr>
          <tr className="border-b border-border">
            <td className="py-2 text-muted-foreground">Gider (−)</td>
            <td className="py-2 text-right font-mono text-rose-700 dark:text-rose-300">
              −{formatTl(report.total_expense)}
            </td>
          </tr>
          <tr className="border-b border-border">
            <td className="py-2 text-muted-foreground">İade (−)</td>
            <td className="py-2 text-right font-mono text-rose-700 dark:text-rose-300">
              −{formatTl(report.total_refund)}
            </td>
          </tr>
          <tr className="border-b border-border bg-muted/40">
            <td className="py-2 font-medium">Beklenen kasa</td>
            <td className="py-2 text-right font-mono font-semibold">
              {formatTl(report.expected_close)}
            </td>
          </tr>
          {reg.declared_balance != null && (
            <>
              <tr className="border-b border-border">
                <td className="py-2 font-medium">Sayım (beyan)</td>
                <td className="py-2 text-right font-mono font-medium">
                  {formatTl(reg.declared_balance)}
                </td>
              </tr>
              {variance != null && (
                <tr
                  className={
                    "border-b border-border " +
                    (variance === 0
                      ? ""
                      : variance > 0
                        ? "bg-emerald-50 dark:bg-emerald-950/30"
                        : "bg-rose-50 dark:bg-rose-950/30")
                  }
                >
                  <td className="py-2 font-semibold">Fark</td>
                  <td
                    className={
                      "py-2 text-right font-mono font-semibold " +
                      (variance === 0
                        ? ""
                        : variance > 0
                          ? "text-emerald-700 dark:text-emerald-300"
                          : "text-rose-700 dark:text-rose-300")
                    }
                  >
                    {variance > 0 ? "+" : ""}
                    {formatTl(variance)}
                  </td>
                </tr>
              )}
            </>
          )}
        </tbody>
      </table>
    </section>
  );
}

function ByKindMethod({ report }: { report: ZReport }) {
  // Group by method first (column), kind second (row).
  const grouped = new Map<PaymentMethod, Partial<Record<CashMovementKind, number>>>();
  for (const r of report.by_kind_method) {
    const slot = grouped.get(r.method) ?? {};
    slot[r.kind] = (slot[r.kind] ?? 0) + r.total;
    grouped.set(r.method, slot);
  }
  const kinds: CashMovementKind[] = ["income", "expense", "refund"];

  return (
    <section className="mb-6">
      <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
        Ödeme Yöntemine Göre Dağılım
      </h2>
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="border-b-2 border-border">
            <th className="py-2 text-left text-xs font-semibold uppercase text-muted-foreground">
              Yöntem
            </th>
            {kinds.map((k) => (
              <th
                key={k}
                className="py-2 text-right text-xs font-semibold uppercase text-muted-foreground"
              >
                {KIND_LABELS[k]}
              </th>
            ))}
            <th className="py-2 text-right text-xs font-semibold uppercase text-muted-foreground">
              Net
            </th>
          </tr>
        </thead>
        <tbody>
          {METHOD_ORDER.map((m) => {
            const slot = grouped.get(m) ?? {};
            const net = (slot.income ?? 0) - (slot.expense ?? 0) - (slot.refund ?? 0);
            const empty = !slot.income && !slot.expense && !slot.refund;
            if (empty) return null;
            return (
              <tr key={m} className="border-b border-border">
                <td className="py-2 font-medium">{METHOD_LABELS[m]}</td>
                {kinds.map((k) => (
                  <td key={k} className="py-2 text-right font-mono">
                    {slot[k] ? formatTl(slot[k] ?? 0) : "—"}
                  </td>
                ))}
                <td className="py-2 text-right font-mono font-medium">
                  {formatTl(net)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </section>
  );
}

function MovementList({ movements }: { movements: CashMovement[] }) {
  if (movements.length === 0) {
    return (
      <section className="mb-6">
        <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
          Hareketler
        </h2>
        <p className="text-sm text-muted-foreground">Hareket yok.</p>
      </section>
    );
  }
  return (
    <section className="mb-6">
      <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
        Hareketler ({movements.length})
      </h2>
      <table className="w-full border-collapse text-xs">
        <thead>
          <tr className="border-b-2 border-border">
            <th className="py-1.5 text-left">Saat</th>
            <th className="py-1.5 text-left">No</th>
            <th className="py-1.5 text-left">Tür</th>
            <th className="py-1.5 text-left">Yöntem</th>
            <th className="py-1.5 text-right">Tutar</th>
            <th className="py-1.5 text-left">Karşı taraf</th>
            <th className="py-1.5 text-left">Açıklama</th>
          </tr>
        </thead>
        <tbody>
          {movements.map((m) => (
            <tr key={m.id} className="border-b border-border/60">
              <td className="py-1 font-mono">
                {new Date(m.performed_at).toLocaleTimeString("tr-TR", {
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </td>
              <td className="py-1 font-mono">{m.movement_no}</td>
              <td className="py-1">{KIND_LABELS[m.kind] ?? m.kind}</td>
              <td className="py-1">{METHOD_LABELS[m.method] ?? m.method}</td>
              <td
                className={
                  "py-1 text-right font-mono " +
                  (m.kind === "expense" || m.kind === "refund" || m.kind === "transfer_out"
                    ? "text-rose-700 dark:text-rose-300"
                    : "")
                }
              >
                {formatTl(m.amount)}
              </td>
              <td className="py-1">{m.counterparty ?? ""}</td>
              <td className="py-1 text-muted-foreground">{m.description ?? ""}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function Signature({ report }: { report: ZReport }) {
  const reg = report.register;
  return (
    <footer className="mt-10 grid grid-cols-2 gap-12 border-t border-border pt-8 text-sm">
      <div>
        <div className="border-b border-border pb-12" />
        <div className="mt-2 text-xs">
          <div className="font-medium">{reg.cashier_name}</div>
          <div className="text-muted-foreground">Kasiyer (imza)</div>
        </div>
      </div>
      <div>
        <div className="border-b border-border pb-12" />
        <div className="mt-2 text-xs">
          <div className="font-medium">_________________________________</div>
          <div className="text-muted-foreground">Yetkili / Muhasebe (imza)</div>
        </div>
      </div>
    </footer>
  );
}
