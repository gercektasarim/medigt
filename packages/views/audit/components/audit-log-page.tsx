"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, Filter, ShieldCheck } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import {
  auditFacetsOptions,
  auditListOptions,
  type AuditEntry,
  type AuditFilter,
} from "@medigt/core/audit";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { SideSheet } from "../../common/side-sheet";
import {
  Field,
  SecondaryButton,
  SelectInput,
  TextInput,
} from "../../common/form-fields";

// KVKK compliance — org admins must be able to review who accessed what.
// This page lists audit_log rows filtered by org (header), with optional
// actor / action / entity / date-range narrowing. The details JSON is
// rendered raw in a side-sheet for forensic inspection.

const PAGE_SIZE = 50;

export function AuditLogPage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";

  const [filter, setFilter] = useState<AuditFilter>({
    limit: PAGE_SIZE,
    offset: 0,
  });
  const facets = useQuery(auditFacetsOptions(orgId));
  const list = useQuery(auditListOptions(orgId, filter));
  const [selected, setSelected] = useState<AuditEntry | null>(null);

  const total = list.data?.total ?? 0;
  const page = Math.floor((filter.offset ?? 0) / PAGE_SIZE) + 1;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const updateFilter = (patch: Partial<AuditFilter>) => {
    // Any filter change resets pagination.
    setFilter((f) => ({ ...f, ...patch, offset: 0 }));
  };

  const columns: Column<AuditEntry>[] = useMemo(
    () => [
      {
        key: "at",
        header: "Zaman",
        cell: (r) => (
          <span className="whitespace-nowrap font-mono text-xs">
            {new Date(r.created_at).toLocaleString("tr-TR", {
              dateStyle: "short",
              timeStyle: "medium",
            })}
          </span>
        ),
      },
      {
        key: "actor",
        header: "Kullanıcı",
        cell: (r) => (
          <div className="text-xs">
            <div className="font-medium">
              {r.actor_name || r.actor_email || "—"}
            </div>
            {r.actor_email && r.actor_name && (
              <div className="text-muted-foreground">{r.actor_email}</div>
            )}
          </div>
        ),
      },
      {
        key: "action",
        header: "Eylem",
        cell: (r) => (
          <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
            {r.action}
          </code>
        ),
      },
      {
        key: "entity",
        header: "Hedef",
        cell: (r) => (
          <div className="text-xs">
            <div className="font-medium">{r.entity_type}</div>
            {r.entity_id && (
              <div className="font-mono text-muted-foreground">
                {r.entity_id.length > 12
                  ? `${r.entity_id.slice(0, 8)}...`
                  : r.entity_id}
              </div>
            )}
          </div>
        ),
      },
      {
        key: "ip",
        header: "IP",
        cell: (r) => (
          <span className="font-mono text-xs text-muted-foreground">
            {r.ip_address ?? "—"}
          </span>
        ),
      },
      {
        key: "details",
        header: "",
        cell: (r) => (
          <button
            type="button"
            onClick={() => setSelected(r)}
            className="inline-flex items-center gap-1 rounded-md border border-input bg-background px-2 py-1 text-xs hover:bg-muted"
          >
            Detay <ChevronRight className="h-3.5 w-3.5" />
          </button>
        ),
        className: "text-right",
      },
    ],
    [],
  );

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Denetim Kayıtları"
          subtitle="KVKK gereği tüm kritik erişimler 10 yıl boyunca saklanır. Aşağıdan filtreyip dışa aktarabilirsiniz."
          actions={
            <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs text-muted-foreground">
              <ShieldCheck className="h-4 w-4" /> KVKK
            </span>
          }
        />

        <section className="rounded-lg border border-border bg-card p-3">
          <div className="mb-2 flex items-center gap-2 text-sm font-semibold">
            <Filter className="h-4 w-4" /> Filtreler
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Field id="f-action" label="Eylem">
              <SelectInput
                id="f-action"
                value={filter.action ?? ""}
                onChange={(e) =>
                  updateFilter({ action: e.target.value || undefined })
                }
              >
                <option value="">Tümü</option>
                {(facets.data?.actions ?? []).map((a) => (
                  <option key={a} value={a}>
                    {a}
                  </option>
                ))}
              </SelectInput>
            </Field>
            <Field id="f-entity" label="Hedef tipi">
              <SelectInput
                id="f-entity"
                value={filter.entity_type ?? ""}
                onChange={(e) =>
                  updateFilter({ entity_type: e.target.value || undefined })
                }
              >
                <option value="">Tümü</option>
                {(facets.data?.entity_types ?? []).map((a) => (
                  <option key={a} value={a}>
                    {a}
                  </option>
                ))}
              </SelectInput>
            </Field>
            <Field id="f-from" label="Başlangıç">
              <TextInput
                id="f-from"
                type="date"
                value={filter.from ?? ""}
                onChange={(e) =>
                  updateFilter({ from: e.target.value || undefined })
                }
              />
            </Field>
            <Field id="f-to" label="Bitiş">
              <TextInput
                id="f-to"
                type="date"
                value={filter.to ?? ""}
                onChange={(e) =>
                  updateFilter({ to: e.target.value || undefined })
                }
              />
            </Field>
            <Field id="f-entity-id" label="Hedef ID (UUID)">
              <TextInput
                id="f-entity-id"
                value={filter.entity_id ?? ""}
                onChange={(e) =>
                  updateFilter({ entity_id: e.target.value || undefined })
                }
                placeholder="örn. hasta veya kayıt UUID'si"
              />
            </Field>
            <Field id="f-actor" label="Kullanıcı ID (UUID)">
              <TextInput
                id="f-actor"
                value={filter.actor_user_id ?? ""}
                onChange={(e) =>
                  updateFilter({ actor_user_id: e.target.value || undefined })
                }
              />
            </Field>
            <div className="flex items-end">
              <SecondaryButton
                type="button"
                onClick={() =>
                  setFilter({ limit: PAGE_SIZE, offset: 0 })
                }
                className="w-full"
              >
                Temizle
              </SecondaryButton>
            </div>
          </div>
        </section>

        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>
            {total > 0
              ? `${total} kayıt · sayfa ${page}/${totalPages}`
              : list.isLoading
                ? "Yükleniyor..."
                : "Kayıt yok"}
          </span>
          <div className="flex gap-1">
            <SecondaryButton
              type="button"
              disabled={(filter.offset ?? 0) <= 0}
              onClick={() =>
                setFilter((f) => ({
                  ...f,
                  offset: Math.max(0, (f.offset ?? 0) - PAGE_SIZE),
                }))
              }
            >
              <ChevronLeft className="h-3.5 w-3.5" />
            </SecondaryButton>
            <SecondaryButton
              type="button"
              disabled={page >= totalPages}
              onClick={() =>
                setFilter((f) => ({
                  ...f,
                  offset: (f.offset ?? 0) + PAGE_SIZE,
                }))
              }
            >
              <ChevronRight className="h-3.5 w-3.5" />
            </SecondaryButton>
          </div>
        </div>

        {list.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : (list.data?.items ?? []).length === 0 ? (
          <div className="empty-state">Bu filtreye uyan kayıt yok.</div>
        ) : (
          <DataTable<AuditEntry>
            rows={list.data?.items ?? []}
            rowKey={(r) => String(r.id)}
            columns={columns}
          />
        )}
      </div>

      {selected && (
        <AuditDetailSheet
          entry={selected}
          onClose={() => setSelected(null)}
        />
      )}
    </DashboardLayout>
  );
}

function AuditDetailSheet({
  entry,
  onClose,
}: {
  entry: AuditEntry;
  onClose: () => void;
}) {
  return (
    <SideSheet open onClose={onClose} title="Kayıt Detayı">
      <div className="space-y-4 text-sm">
        <div className="grid grid-cols-[8rem_1fr] gap-2">
          <div className="text-muted-foreground">ID</div>
          <div className="font-mono">{entry.id}</div>
          <div className="text-muted-foreground">Zaman</div>
          <div>{new Date(entry.created_at).toLocaleString("tr-TR")}</div>
          <div className="text-muted-foreground">Eylem</div>
          <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
            {entry.action}
          </code>
          <div className="text-muted-foreground">Hedef tipi</div>
          <div>{entry.entity_type}</div>
          {entry.entity_id && (
            <>
              <div className="text-muted-foreground">Hedef ID</div>
              <div className="break-all font-mono text-xs">
                {entry.entity_id}
              </div>
            </>
          )}
          <div className="text-muted-foreground">Kullanıcı</div>
          <div>
            {entry.actor_name || entry.actor_email || "—"}
            {entry.actor_user_id && (
              <div className="break-all font-mono text-xs text-muted-foreground">
                {entry.actor_user_id}
              </div>
            )}
          </div>
          <div className="text-muted-foreground">IP</div>
          <div className="font-mono text-xs">{entry.ip_address ?? "—"}</div>
          <div className="text-muted-foreground">User-Agent</div>
          <div className="break-all text-xs">{entry.user_agent ?? "—"}</div>
        </div>

        <section>
          <h3 className="mb-1 text-sm font-semibold">Detaylar</h3>
          <pre className="max-h-72 overflow-auto rounded-md border border-border bg-muted/40 p-2 text-xs">
            {JSON.stringify(entry.details ?? {}, null, 2)}
          </pre>
        </section>
      </div>
    </SideSheet>
  );
}
