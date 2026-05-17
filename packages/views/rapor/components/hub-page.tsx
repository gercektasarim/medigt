"use client";

import { ChevronRight } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { reportGroups } from "@medigt/core/rapor/registry";
import { useNavigation } from "@medigt/core/navigation";
import { paths } from "@medigt/core/paths";
import { DashboardLayout, PageHeader } from "../../layout";

export function RaporHubPage() {
  const org = useHospitalStore((s) => s.organization);
  const branch = useHospitalStore((s) => s.branch);
  const nav = useNavigation();
  const groups = reportGroups();

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="Raporlar"
          subtitle="Tüm modüllerden raporlar tek motorla çalışır. Parametreleri girin, sonuçları görüntüleyin, dışa aktarın."
        />

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {groups.map(({ group, reports }) => (
            <section key={group} className="space-y-2 rounded-lg border border-border bg-card p-4">
              <h2 className="text-sm font-semibold text-muted-foreground">{group}</h2>
              <ul className="space-y-1">
                {reports.map((r) => (
                  <li key={r.id}>
                    <button
                      type="button"
                      onClick={() =>
                        nav.push(paths.hospital(org?.slug ?? "").branch(branch?.slug ?? "").rapor.report(r.slug))
                      }
                      className="flex w-full items-start justify-between gap-3 rounded-md border border-transparent px-3 py-2 text-left hover:border-border hover:bg-muted"
                    >
                      <div className="min-w-0 flex-1">
                        <div className="text-sm font-medium">{r.title}</div>
                        {r.description && (
                          <div className="line-clamp-2 text-xs text-muted-foreground">{r.description}</div>
                        )}
                      </div>
                      <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
                    </button>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      </div>
    </DashboardLayout>
  );
}
