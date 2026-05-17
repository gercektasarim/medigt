import { api } from "../api/client";

export type Icd10ImportResult = {
  processed: number;
  inserted: number;
  updated: number;
};

// importIcd10TSV uploads a raw TSV body. Body is sent as text (rawBody)
// so the API client doesn't JSON-stringify it. Max 5MB enforced server-side.
export function importIcd10TSV(tsv: string): Promise<Icd10ImportResult> {
  return api().request<Icd10ImportResult>("/api/icd10/import", {
    method: "POST",
    body: tsv,
    rawBody: true,
    headers: { "Content-Type": "text/tab-separated-values" },
  });
}
