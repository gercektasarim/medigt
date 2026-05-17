"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, Fingerprint, History, XCircle } from "lucide-react";
import { useHospitalStore } from "@medigt/core/hospital";
import { mernisKeys, mernisLogsOptions, verifyMernis } from "@medigt/core/mernis";
import type { MernisLogRow, MernisVerifyResult } from "@medigt/core/types";
import { DashboardLayout, PageHeader } from "../../layout";
import { DataTable, type Column } from "../../common/data-table";
import { Field, PrimaryButton, TextInput } from "../../common/form-fields";

export function MernisPage() {
  const org = useHospitalStore((s) => s.organization);
  const orgId = org?.id ?? "";
  const qc = useQueryClient();
  const logs = useQuery(mernisLogsOptions(orgId));

  const [tc, setTC] = useState("");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [birthYear, setBirthYear] = useState("");
  const [last, setLast] = useState<MernisVerifyResult | null>(null);

  const verify = useMutation({
    mutationFn: () =>
      verifyMernis({
        tc_kimlik_no: tc,
        first_name: firstName,
        last_name: lastName,
        birth_year: Number(birthYear),
      }),
    onSuccess: (res) => {
      setLast(res);
      qc.invalidateQueries({ queryKey: mernisKeys.all(orgId) });
    },
  });

  const canSubmit =
    tc.length === 11 && /^\d+$/.test(tc) &&
    firstName.trim().length > 0 && lastName.trim().length > 0 &&
    Number(birthYear) > 1900 && !verify.isPending;

  return (
    <DashboardLayout>
      <div className="page-shell">
        <PageHeader
          title="MERNIS"
          subtitle="NVI KPSPublicV2 entegrasyonu — TC kimlik no doğrulaması. (Bugün simülasyon modunda; gerçek sertifika gelince swap edilir.)"
        />

        <section className="rounded-lg border border-border bg-card p-4">
          <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold">
            <Fingerprint className="h-4 w-4" /> Hızlı Doğrulama
          </h2>
          <form
            className="grid grid-cols-1 gap-3 sm:grid-cols-2"
            onSubmit={(e) => { e.preventDefault(); verify.mutate(); }}
          >
            <Field id="m-tc" label="TC Kimlik No" required>
              <TextInput
                id="m-tc"
                required
                value={tc}
                onChange={(e) => setTC(e.target.value.replace(/\D/g, "").slice(0, 11))}
                placeholder="11 haneli"
              />
            </Field>
            <Field id="m-year" label="Doğum Yılı" required>
              <TextInput
                id="m-year"
                type="number"
                min="1900"
                max="2100"
                required
                value={birthYear}
                onChange={(e) => setBirthYear(e.target.value)}
              />
            </Field>
            <Field id="m-first" label="Ad" required>
              <TextInput
                id="m-first"
                required
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
              />
            </Field>
            <Field id="m-last" label="Soyad" required>
              <TextInput
                id="m-last"
                required
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
              />
            </Field>
            <div className="sm:col-span-2 flex items-center gap-3">
              <PrimaryButton type="submit" disabled={!canSubmit}>
                <span className="inline-flex items-center gap-1">
                  <Fingerprint className="h-4 w-4" /> {verify.isPending ? "Doğrulanıyor..." : "Doğrula"}
                </span>
              </PrimaryButton>
              {last && (
                <ResultBadge result={last} />
              )}
              {verify.isError && (
                <span className="text-sm text-[var(--critical)]">
                  Hata: {(verify.error as Error)?.message}
                </span>
              )}
            </div>
          </form>
          <p className="mt-3 text-xs text-muted-foreground">
            Simülasyon kuralı: son hanesi <code className="rounded bg-muted px-1">0</code> olan TC'ler reddedilir.
            Geçerli bir TC örneği üretip test edebilirsiniz.
          </p>
        </section>

        <section>
          <h2 className="mb-2 flex items-center gap-2 text-sm font-semibold">
            <History className="h-4 w-4" /> Son Doğrulamalar
          </h2>
          {logs.isLoading ? (
            <div className="empty-state">Yükleniyor...</div>
          ) : (logs.data ?? []).length === 0 ? (
            <div className="empty-state">Henüz doğrulama yapılmamış.</div>
          ) : (
            <DataTable<MernisLogRow>
              rows={logs.data ?? []}
              rowKey={(r) => r.id}
              columns={logColumns}
            />
          )}
        </section>
      </div>
    </DashboardLayout>
  );
}

function ResultBadge({ result }: { result: MernisVerifyResult }) {
  if (result.verified) {
    return (
      <span className="inline-flex items-center gap-1 rounded-md bg-emerald-100 px-3 py-1 text-sm font-medium text-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200">
        <CheckCircle2 className="h-4 w-4" /> Doğrulandı ({result.response_code})
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1 rounded-md bg-rose-100 px-3 py-1 text-sm font-medium text-rose-900 dark:bg-rose-950/40 dark:text-rose-200">
      <XCircle className="h-4 w-4" /> Reddedildi {result.response_code && `(${result.response_code})`}
    </span>
  );
}

const logColumns: Column<MernisLogRow>[] = [
  {
    key: "at",
    header: "Tarih",
    cell: (r) => new Date(r.requested_at).toLocaleString("tr-TR"),
  },
  {
    key: "verified",
    header: "Sonuç",
    cell: (r) =>
      r.verified ? (
        <span className="inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-300">
          <CheckCircle2 className="h-3.5 w-3.5" /> Doğru
        </span>
      ) : (
        <span className="inline-flex items-center gap-1 text-rose-700 dark:text-rose-300">
          <XCircle className="h-3.5 w-3.5" /> Yanlış
        </span>
      ),
  },
  {
    key: "name",
    header: "Kişi",
    cell: (r) => (
      <div>
        <div className="font-medium">{r.first_name} {r.last_name}</div>
        <div className="text-xs text-muted-foreground">
          TC ****{r.tc_last4} · Doğum {r.birth_year}
        </div>
      </div>
    ),
  },
  {
    key: "code",
    header: "Yanıt",
    cell: (r) => (
      <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
        {r.response_code ?? r.error_message ?? "—"}
      </code>
    ),
  },
];
