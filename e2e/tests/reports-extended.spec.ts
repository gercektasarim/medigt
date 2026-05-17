// Extended reports — smoke-test the 11 new report ids added on top of
// the original 10. Each report must:
//   - return a non-empty `columns` array with the expected key set
//   - never crash on an empty dataset (fresh org, no domain data yet)

import { expect, test } from "@playwright/test";
import { createTestApi, runReport, type TestApi } from "../helpers";

test.describe("rapor — extended catalog smoke", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  const today = new Date().toISOString().slice(0, 10);
  const monthStart = today.slice(0, 8) + "01";

  const cases: Array<{
    id: string;
    params: Record<string, string>;
    requiredColumns: string[];
  }> = [
    {
      id: "hourly-collection",
      params: { from: today, to: today },
      requiredColumns: ["hour", "count", "total"],
    },
    {
      id: "cashier-collection",
      params: { from: monthStart, to: today },
      requiredColumns: ["cashier_name", "sessions", "income_total", "net"],
    },
    {
      id: "open-advances",
      params: {},
      requiredColumns: ["patient", "balance", "last_at"],
    },
    {
      id: "diagnosis-distribution",
      params: { from: monthStart, to: today },
      requiredColumns: ["icd10", "title", "count"],
    },
    {
      id: "polyclinic-by-hour",
      params: { from: today, to: today },
      requiredColumns: ["hour", "visits", "completed", "dropped"],
    },
    {
      id: "lab-test-volume",
      params: { from: monthStart, to: today },
      requiredColumns: ["test_code", "ordered", "resulted", "critical"],
    },
    {
      id: "ward-admission-stats",
      params: { from: monthStart, to: today },
      requiredColumns: ["ward", "admissions", "discharges", "active"],
    },
    {
      id: "top-medications",
      params: { from: monthStart, to: today },
      requiredColumns: ["medication", "dispense_count", "total_qty"],
    },
    {
      id: "surgeon-performance",
      params: { from: monthStart, to: today },
      requiredColumns: ["surgeon", "total", "completed", "cancelled"],
    },
    {
      id: "medula-success-rate",
      params: { from: monthStart, to: today },
      requiredColumns: ["metric", "value"],
    },
  ];

  for (const c of cases) {
    test(`${c.id} returns column metadata + doesn't crash on empty data`, async () => {
      const res = await runReport(api, c.id, c.params);
      const keys = res.columns.map((col) => col.key);
      for (const required of c.requiredColumns) {
        expect(keys).toContain(required);
      }
      // Rows may be empty for a fresh org — that's the point.
      expect(Array.isArray(res.rows)).toBeTruthy();
    });
  }

  test("unknown report id → 404", async () => {
    const r = await api.request.get("/api/reports/this-does-not-exist");
    expect(r.status()).toBe(404);
  });
});
