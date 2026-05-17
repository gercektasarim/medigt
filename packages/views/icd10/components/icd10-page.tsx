"use client";

import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Upload } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { icd10SearchOptions, icd10Keys, importIcd10TSV } from "@medigt/core/icd10";
import type { Icd10Code } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { PrimaryButton, SecondaryButton, TextInput } from "../../common/form-fields";
import { SideSheet } from "../../common/side-sheet";

export function Icd10Page() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const [q, setQ] = useState("");
  const search = useQuery(icd10SearchOptions(orgId, q, 100));
  const [importOpen, setImportOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="ICD-10 Tanı Kataloğu"
          subtitle="Uluslararası Hastalık Sınıflaması. Sistem kataloğu (~362 Türkçe kod) + size özel eklemeler. Org yöneticisi TSV yükleyerek tam ~14.000 satırlık Sağlık Bakanlığı listesini ekleyebilir."
          actions={
            <PrimaryButton type="button" onClick={() => setImportOpen(true)}>
              <span className="inline-flex items-center gap-1"><Upload className="h-4 w-4" /> TSV Yükle</span>
            </PrimaryButton>
          }
        />

        <TextInput
          autoFocus
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Ara: kod (I10) veya başlık (hipertansiyon)"
          className="max-w-md"
        />

        {search.isLoading ? (
          <div className="empty-state">Yükleniyor...</div>
        ) : search.isError ? (
          <div className="empty-state text-[var(--critical)]">Arama başarısız.</div>
        ) : (
          <DataTable<Icd10Code>
            rows={search.data ?? []}
            rowKey={(r) => r.id}
            columns={icdColumns}
          />
        )}
      </div>

      {importOpen && <ImportSheet orgId={orgId} onClose={() => setImportOpen(false)} />}
    </DashboardLayout>
  );
}

const icdColumns: Column<Icd10Code>[] = [
  {
    key: "code",
    header: "Kod",
    cell: (r) => <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{r.code}</code>,
  },
  { key: "title", header: "Başlık (TR)", cell: (r) => <span className="font-medium">{r.title_tr}</span> },
  {
    key: "chapter",
    header: "Bölüm",
    cell: (r) => (r.chapter ? <span className="text-xs text-muted-foreground">{r.chapter}</span> : "—"),
  },
  {
    key: "source",
    header: "Kaynak",
    cell: (r) =>
      r.is_system ? (
        <span className="text-xs text-muted-foreground">Sistem</span>
      ) : (
        <span className="text-xs text-[var(--brand)]">Özel</span>
      ),
  },
];

function ImportSheet({ orgId, onClose }: { orgId: string; onClose: () => void }) {
  const qc = useQueryClient();
  const fileRef = useRef<HTMLInputElement>(null);
  const [text, setText] = useState("");
  const [result, setResult] = useState<{ processed: number; inserted: number; updated: number } | null>(null);

  const doImport = useMutation({
    mutationFn: () => importIcd10TSV(text),
    onSuccess: async (data) => {
      setResult(data);
      await qc.invalidateQueries({ queryKey: icd10Keys.all(orgId) });
    },
  });

  function handleFile(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (!f) return;
    const reader = new FileReader();
    reader.onload = () => setText(String(reader.result ?? ""));
    reader.readAsText(f, "utf-8");
  }

  return (
    <SideSheet open onClose={onClose} title="ICD-10 TSV Yükle">
      <div className="space-y-4">
        <p className="text-sm text-muted-foreground">
          UTF-8 TSV dosyası. Sütunlar: <code className="rounded bg-muted px-1">code</code> ·{" "}
          <code className="rounded bg-muted px-1">title_tr</code> ·{" "}
          <code className="rounded bg-muted px-1">chapter</code> (opsiyonel) ·{" "}
          <code className="rounded bg-muted px-1">parent_code</code> (opsiyonel).
          İlk satır başlık olabilir (atlanır). Var olan kodlar başlık/bölüm bilgisi
          güncellenir.
        </p>

        <div>
          <input
            ref={fileRef}
            type="file"
            accept=".tsv,.txt,text/tab-separated-values,text/plain"
            onChange={handleFile}
            className="block w-full text-sm file:mr-3 file:rounded-md file:border file:border-input file:bg-background file:px-3 file:py-1.5 file:text-sm"
          />
        </div>

        {text && (
          <div className="space-y-1">
            <div className="text-xs text-muted-foreground">
              Önizleme — {text.split("\n").length} satır
            </div>
            <pre className="max-h-40 overflow-auto rounded-md border border-border bg-muted/40 p-2 text-xs">
              {text.split("\n").slice(0, 8).join("\n")}
              {text.split("\n").length > 8 && "\n…"}
            </pre>
          </div>
        )}

        {result && (
          <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200">
            ✓ {result.processed} kod işlendi · {result.inserted} yeni · {result.updated} güncellendi
          </div>
        )}

        {doImport.isError && (
          <p className="text-sm text-[var(--critical)]">
            Yükleme başarısız: {(doImport.error as Error)?.message}
          </p>
        )}

        <div className="flex gap-2">
          <SecondaryButton type="button" onClick={onClose} className="flex-1">Kapat</SecondaryButton>
          <PrimaryButton
            type="button"
            onClick={() => doImport.mutate()}
            disabled={!text || doImport.isPending}
            className="flex-1"
          >
            <span className="inline-flex items-center gap-1">
              <Upload className="h-4 w-4" /> {doImport.isPending ? "Yükleniyor..." : "Yükle"}
            </span>
          </PrimaryButton>
        </div>
      </div>
    </SideSheet>
  );
}
