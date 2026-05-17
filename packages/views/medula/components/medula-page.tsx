"use client";

import { useState } from "react";
import {
  ArrowRightLeft,
  FileSearch,
  FileSignature,
  Receipt,
  ShieldAlert,
  ShieldCheck,
} from "lucide-react";
import { DashboardLayout, PageHeader } from "../../layout";
import { MedulaProvisionsTab } from "./tab-provisions";
import { MedulaSubmissionsTab } from "./tab-submissions";
import { MedulaReferralsTab } from "./tab-referrals";
import { MedulaEraportsTab } from "./tab-eraports";
import { MedulaQueriesTab } from "./tab-queries";

type Tab = "provisions" | "submissions" | "referrals" | "eraports" | "queries";

const TABS: { id: Tab; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: "provisions", label: "Provizyon", icon: ShieldCheck },
  { id: "submissions", label: "Fatura Gönderim", icon: Receipt },
  { id: "referrals", label: "Sevk", icon: ArrowRightLeft },
  { id: "eraports", label: "e-Rapor", icon: FileSignature },
  { id: "queries", label: "Sorgular", icon: FileSearch },
];

export function MedulaPage() {
  const [tab, setTab] = useState<Tab>("provisions");

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Medula SGK"
          subtitle="14 SGK servisini kapsayan entegrasyon. Bugün simülasyon modunda; sertifika geldiğinde adapter swap edilecek."
        />

        <SimulationBanner />

        <div className="flex gap-1 border-b border-border overflow-x-auto">
          {TABS.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              type="button"
              onClick={() => setTab(id)}
              className={`inline-flex items-center gap-1 whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium transition-colors ${
                tab === id
                  ? "border-primary text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              }`}
            >
              <Icon className="h-4 w-4" />
              {label}
            </button>
          ))}
        </div>

        {tab === "provisions" && <MedulaProvisionsTab />}
        {tab === "submissions" && <MedulaSubmissionsTab />}
        {tab === "referrals" && <MedulaReferralsTab />}
        {tab === "eraports" && <MedulaEraportsTab />}
        {tab === "queries" && <MedulaQueriesTab />}
      </div>
    </DashboardLayout>
  );
}

function SimulationBanner() {
  return (
    <div className="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
      <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0" />
      <div>
        <strong>Simülasyon modu:</strong> Tüm yazma operasyonları (provizyon, fatura,
        sevk, e-rapor) outbox üzerinden mock client'a düşer. TC son hanesi{" "}
        <code className="rounded bg-amber-100 px-1 dark:bg-amber-900/60">0</code> →
        reddedilir; aksi halde deterministik bir SGK kimliği üretilir. Sertifika
        gelince yalnızca <code className="rounded bg-amber-100 px-1 dark:bg-amber-900/60">internal/integration/medula/client.go</code>{" "}
        değişir.
      </div>
    </div>
  );
}
