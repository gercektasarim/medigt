"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  AlertTriangle,
  CheckCircle2,
  FileSignature,
  FlaskConical,
  Inbox as InboxIcon,
  RefreshCw,
  Send,
} from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { inboxListOptions, type InboxItem, type InboxKind } from "@medigt/core/inbox";
import { useNavigation } from "@medigt/core/navigation";
import { DashboardLayout, PageHeader } from "../../layout";
import { SecondaryButton } from "../../common/form-fields";

// Inbox — "things you need to do right now". One uniform list backed
// by a single GET; backend aggregates from multiple source tables.
// Filter chips narrow by kind; clicking an item jumps to the relevant
// detail page (visit / lab order / Medula admin).

const KIND_META: Record<InboxKind, {
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  tone: "info" | "warning" | "critical";
}> = {
  "prescription.unsigned": {
    label: "İmza bekleyen reçeteler",
    icon: FileSignature,
    tone: "info",
  },
  "lab.critical": {
    label: "Kritik lab sonuçları",
    icon: FlaskConical,
    tone: "critical",
  },
  "medula.dead": {
    label: "Medula başarısızları",
    icon: Send,
    tone: "warning",
  },
};

const FILTERS: Array<{ key: "all" | InboxKind; label: string }> = [
  { key: "all", label: "Tümü" },
  { key: "prescription.unsigned", label: "İmza" },
  { key: "lab.critical", label: "Kritik lab" },
  { key: "medula.dead", label: "Medula" },
];

export function InboxPage() {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const branchId = branch?.id ?? "";
  const nav = useNavigation();

  const inbox = useQuery(inboxListOptions(branchId));
  const [filter, setFilter] = useState<"all" | InboxKind>("all");

  const allItems = inbox.data ?? [];
  const items = useMemo(
    () => (filter === "all" ? allItems : allItems.filter((i) => i.kind === filter)),
    [allItems, filter],
  );

  // Group counts per kind drive the filter pill badges.
  const counts = useMemo(() => {
    const c: Record<string, number> = { all: allItems.length };
    for (const it of allItems) c[it.kind] = (c[it.kind] ?? 0) + 1;
    return c;
  }, [allItems]);

  // Build the branch root path manually — paths.ts doesn't expose a
  // "root" helper, just per-module sub-paths.
  const branchRoot = `/h/${encodeURIComponent(org?.slug ?? "")}/${encodeURIComponent(branch?.slug ?? "")}`;

  const goTo = (it: InboxItem) => {
    // ref_url is a *relative* sub-path under the branch root.
    nav.push(`${branchRoot}${it.ref_url}`);
  };

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Gelen Kutusu"
          subtitle="Eyleme açık iş kalemleri — imza bekleyen reçeteler, kritik lab değerleri, Medula başarısızları."
          actions={
            <SecondaryButton
              type="button"
              onClick={() => inbox.refetch()}
              disabled={inbox.isFetching}
            >
              <span className="inline-flex items-center gap-1">
                <RefreshCw className={"h-3.5 w-3.5 " + (inbox.isFetching ? "anim-pulse-soft" : "")} />
                Yenile
              </span>
            </SecondaryButton>
          }
        />

        {/* Filter chips */}
        <div className="flex flex-wrap gap-2">
          {FILTERS.map((f) => {
            const active = f.key === filter;
            const c = counts[f.key] ?? 0;
            return (
              <button
                key={f.key}
                type="button"
                onClick={() => setFilter(f.key)}
                className={active ? "chip-accent" : "chip"}
                aria-pressed={active}
              >
                <span>{f.label}</span>
                <span className="font-mono text-xs opacity-70">{c}</span>
              </button>
            );
          })}
        </div>

        {inbox.isLoading ? (
          <div className="empty-state">Yükleniyor…</div>
        ) : items.length === 0 ? (
          <div className="empty-state">
            <CheckCircle2 className="h-8 w-8 text-emerald-600 dark:text-emerald-400" />
            <p>Şu an bekleyen bir iş yok. Güzel iş!</p>
          </div>
        ) : (
          <ul className="space-y-2">
            {items.map((it) => (
              <InboxRow key={`${it.kind}:${it.id}`} item={it} onOpen={() => goTo(it)} />
            ))}
          </ul>
        )}
      </div>
    </DashboardLayout>
  );
}

function InboxRow({ item, onOpen }: { item: InboxItem; onOpen: () => void }) {
  const meta = KIND_META[item.kind] ?? KIND_META["prescription.unsigned"];
  const Icon = meta.icon;

  const toneRing =
    meta.tone === "critical"
      ? "before:bg-[var(--critical)]"
      : meta.tone === "warning"
        ? "before:bg-[var(--warning)]"
        : "before:bg-[var(--brand)]";

  const occurred = new Date(item.occurred_at);
  const ago = humanAgo(occurred);

  return (
    <li>
      <button
        type="button"
        onClick={onOpen}
        className={
          "surface-card relative flex w-full items-start gap-3 p-4 text-left transition hover:bg-muted/40 " +
          "before:absolute before:left-0 before:top-0 before:h-full before:w-1 before:rounded-l-xl " +
          toneRing
        }
      >
        <div
          className={
            "mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg " +
            (meta.tone === "critical"
              ? "bg-[var(--critical)]/10 text-[var(--critical)]"
              : meta.tone === "warning"
                ? "bg-[var(--warning)]/10 text-[var(--warning)]"
                : "bg-[var(--accent-soft)] text-[var(--brand)]")
          }
        >
          {meta.tone === "critical" ? (
            <AlertTriangle className="h-4 w-4" />
          ) : (
            <Icon className="h-4 w-4" />
          )}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <div className="font-medium">{item.title}</div>
            <div className="text-xs text-muted-foreground">{ago}</div>
          </div>
          {item.subtitle && (
            <div className="mt-0.5 text-sm text-muted-foreground">{item.subtitle}</div>
          )}
        </div>
      </button>
    </li>
  );
}

// humanAgo returns "5 dk önce" / "2 sa önce" / "dün" / "12.05 14:30"
// depending on how stale the timestamp is. Keep it small + dep-free.
function humanAgo(d: Date): string {
  const now = Date.now();
  const diffSec = Math.max(0, Math.floor((now - d.getTime()) / 1000));
  if (diffSec < 60) return "az önce";
  const m = Math.floor(diffSec / 60);
  if (m < 60) return `${m} dk önce`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} sa önce`;
  if (h < 48) return "dün";
  return d.toLocaleString("tr-TR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
}

// Re-export the underlying icon for sidebar / module-id mapping callers.
export { InboxIcon };
