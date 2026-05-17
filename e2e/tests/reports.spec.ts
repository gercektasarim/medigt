// EPIC-8 — generic report runner.
//
//   - Hub returns a non-empty list of report ids (catalog discovery).
//   - daily-cash report runs without params (defaults to today) and
//     returns the expected column shape.
//   - low-expiry-stock honours the `days` parameter.
//   - Unknown report id returns 404.

import { expect, test } from "@playwright/test";
import {
  createMedication,
  createTestApi,
  createWarehouse,
  openCashRegister,
  receiveStock,
  type TestApi,
} from "../helpers";

test.describe("rapor runner", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("daily-cash returns column metadata + session row", async () => {
    const reg = await openCashRegister(api, { opening_balance: 200 });
    // Stamp two cash movements so totals are non-zero.
    await api.request.post(`/api/cash-registers/${reg.id}/movements`, {
      data: { kind: "income", method: "cash", amount: 100 },
    });
    await api.request.post(`/api/cash-registers/${reg.id}/movements`, {
      data: { kind: "expense", method: "cash", amount: 40 },
    });

    const today = new Date().toISOString().slice(0, 10);
    const r = await api.request.get(`/api/reports/daily-cash?from=${today}&to=${today}`);
    expect(r.ok()).toBeTruthy();
    const body = await r.json();
    expect(body.columns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ key: "register_no", type: "text" }),
        expect.objectContaining({ key: "cash_income", type: "currency" }),
        expect.objectContaining({ key: "expected", type: "currency" }),
      ]),
    );
    const ourRow = (body.rows as Array<{ register_no: string; cash_income: number; cash_expense: number; expected: number }>).find(
      (row) => row.register_no === reg.register_no,
    );
    expect(ourRow).toBeTruthy();
    expect(ourRow!.cash_income).toBe(100);
    expect(ourRow!.cash_expense).toBe(40);
    expect(ourRow!.expected).toBe(260); // 200 + 100 - 40
    expect(body.summary.total_income).toBeGreaterThanOrEqual(100);
  });

  test("low-expiry-stock honours `days` window", async () => {
    const wh = await createWarehouse(api, { code: "STK1", name: "Stok 1" });
    const med = await createMedication(api, {
      name: "Soon",
      generic_name: "Soon-generic",
      form: "tablet",
    });
    // SKT 60 days away.
    const expiry = new Date(Date.now() + 60 * 24 * 3600 * 1000)
      .toISOString()
      .slice(0, 10);
    await receiveStock(api, {
      warehouse_id: wh.id,
      medication_id: med.id,
      lot_no: "EXP60",
      expiry_date: expiry,
      quantity: 10,
    });

    // 30-day window: should NOT include this lot.
    const r30 = await api.request.get("/api/reports/low-expiry-stock?days=30");
    const rows30 = (await r30.json()).rows as Array<{ lot_no: string }>;
    expect(rows30.find((row) => row.lot_no === "EXP60")).toBeUndefined();

    // 90-day window: should include it.
    const r90 = await api.request.get("/api/reports/low-expiry-stock?days=90");
    const rows90 = (await r90.json()).rows as Array<{ lot_no: string; days_left: number }>;
    const found = rows90.find((row) => row.lot_no === "EXP60");
    expect(found).toBeTruthy();
    expect(found!.days_left).toBeLessThanOrEqual(90);
  });

  test("unknown report id returns 404", async () => {
    const r = await api.request.get("/api/reports/does-not-exist");
    expect(r.status()).toBe(404);
    const body = await r.json();
    expect(body.error.code).toBe("report_not_found");
  });
});
