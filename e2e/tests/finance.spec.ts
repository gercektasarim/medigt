// EPIC-7 — vezne + fatura + hakediş.
//
// Invariants under test:
//   - Sum of payment_allocation amounts always equals the payment amount.
//   - Invoice.balance_due never goes negative; over-allocation is rejected.
//   - Cash payment requires an open register AND creates a cash_movement
//     in the same tx as the payment.
//   - Z-report's expected_close == opening + cash_income - cash_expense -
//     cash_refund.
//   - Hakediş summary applies the commission rule's percentage to the
//     paid invoice_item.line_total.

import { expect, test } from "@playwright/test";
import {
  createDoctor,
  createInvoice,
  createPatient,
  createTestApi,
  listSpecializations,
  nextValidTC,
  openCashRegister,
  type TestApi,
} from "../helpers";

test.describe("vezne — cash register session", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("open → record cash movement → Z report → close", async () => {
    const reg = await openCashRegister(api, { opening_balance: 100, notes: "Açılış sayımı" });
    expect(reg.register_no).toMatch(/^K-\d{8}$/);

    // Record walk-in tahsilat.
    const mvtResp = await api.request.post(`/api/cash-registers/${reg.id}/movements`, {
      data: { kind: "income", method: "cash", amount: 250, counterparty: "Bay X" },
    });
    expect(mvtResp.status()).toBe(201);

    // Record gider.
    const expResp = await api.request.post(`/api/cash-registers/${reg.id}/movements`, {
      data: { kind: "expense", method: "cash", amount: 30, description: "Su gideri" },
    });
    expect(expResp.ok()).toBeTruthy();

    // Z report: expected = 100 + 250 - 30 = 320.
    const z = await api.request.get(`/api/cash-registers/${reg.id}/z-report`);
    expect(z.ok()).toBeTruthy();
    const zReport = await z.json();
    expect(zReport.total_income).toBe(250);
    expect(zReport.total_expense).toBe(30);
    expect(zReport.expected_close).toBe(320);

    // Close with declared = expected (zero variance).
    const close = await api.request.post(`/api/cash-registers/${reg.id}/close`, {
      data: { declared_balance: 320 },
    });
    expect(close.ok()).toBeTruthy();

    // Z report after close has zero variance.
    const z2 = await api.request.get(`/api/cash-registers/${reg.id}/z-report`);
    const zAfter = await z2.json();
    expect(zAfter.variance).toBe(0);

    // Cannot open a second register while one's still considered open for
    // the user — but ours is closed now, so a fresh open works.
    const reg2 = await openCashRegister(api, { opening_balance: 50 });
    expect(reg2.id).not.toBe(reg.id);
  });

  test("cannot open two registers for the same cashier", async () => {
    await openCashRegister(api, { opening_balance: 0 });
    const second = await api.request.post("/api/cash-registers", {
      data: { opening_balance: 0 },
    });
    expect(second.status()).toBe(409);
    expect((await second.json()).error.code).toBe("already_open");
  });
});

test.describe("fatura + ödeme — full money loop", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("create invoice → finalize → cash payment → status flips to paid", async () => {
    const patient = await createPatient(api, {
      first_name: "Inv", last_name: "Hasta",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });

    // Doctor for line attribution (used in hakediş test below).
    const specs = await listSpecializations(api);
    const doctor = await createDoctor(api, {
      staff: { first_name: "Hesap", last_name: "Doktor", title: "Dr." },
      specialization_ids: [specs[0]!.id],
      primary_specialization_id: specs[0]!.id,
    });

    const inv = await createInvoice(api, {
      patient_id: patient.id,
      finalize: true,
      items: [
        { code: "MUAYENE", name: "Muayene", quantity: 1, unit_price: 500, vat_rate: 10, doctor_id: doctor.id },
        { code: "TAHLIL",  name: "Tahlil",  quantity: 2, unit_price: 100, vat_rate: 10 },
      ],
    });
    expect(inv.invoice_no).toMatch(/^\d{8}$/);

    // Fetch detail; totals should be: subtotal = 500 + 200 = 700,
    // tax = 70, total = 770.
    const detailResp = await api.request.get(`/api/invoices/${inv.id}`);
    expect(detailResp.ok()).toBeTruthy();
    const detail = await detailResp.json();
    expect(detail.invoice.subtotal).toBe(700);
    expect(detail.invoice.tax_total).toBe(70);
    expect(detail.invoice.total).toBe(770);
    expect(detail.invoice.balance_due).toBe(770);
    expect(detail.invoice.status).toBe("finalized");

    // Cash payment requires an open kasa.
    const reg = await openCashRegister(api, { opening_balance: 0 });

    const payResp = await api.request.post("/api/payments", {
      data: {
        patient_id: patient.id,
        method: "cash",
        amount: 770,
        cash_register_id: reg.id,
        allocations: [{ invoice_id: inv.id, amount: 770 }],
      },
    });
    expect(payResp.status()).toBe(201);
    const pay = await payResp.json();
    expect(pay.payment_no).toMatch(/^\d{8}$/);
    expect(pay.cash_movement_no).toMatch(/^\d{8}$/);

    // Invoice now paid.
    const after = await api.request.get(`/api/invoices/${inv.id}`);
    const afterBody = await after.json();
    expect(afterBody.invoice.status).toBe("paid");
    expect(afterBody.invoice.paid_total).toBe(770);
    expect(afterBody.invoice.balance_due).toBe(0);

    // Kasa hareket listesinde income görünür.
    const mvts = await api.request.get(`/api/cash-registers/${reg.id}/movements`);
    const mvtRows = (await mvts.json()) as Array<{ kind: string; amount: number; reference_type: string | null }>;
    const income = mvtRows.find((m) => m.kind === "income");
    expect(income).toBeTruthy();
    expect(income!.amount).toBe(770);
    expect(income!.reference_type).toBe("payment");
  });

  test("over-allocation is rejected; partial payment is accepted", async () => {
    const patient = await createPatient(api, {
      first_name: "Partial", last_name: "Hasta",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const inv = await createInvoice(api, {
      patient_id: patient.id,
      finalize: true,
      items: [{ code: "X", name: "Service", quantity: 1, unit_price: 100, vat_rate: 0 }],
    });

    // Over-allocate (200 > 100 total).
    const over = await api.request.post("/api/payments", {
      data: {
        patient_id: patient.id, method: "card", amount: 200,
        allocations: [{ invoice_id: inv.id, amount: 200 }],
      },
    });
    expect(over.status()).toBe(409);

    // Partial payment: 40/100 → status stays finalized, balance = 60.
    const partial = await api.request.post("/api/payments", {
      data: {
        patient_id: patient.id, method: "card", amount: 40,
        allocations: [{ invoice_id: inv.id, amount: 40 }],
      },
    });
    expect(partial.status()).toBe(201);
    const detail = await (await api.request.get(`/api/invoices/${inv.id}`)).json();
    expect(detail.invoice.status).toBe("finalized");
    expect(detail.invoice.paid_total).toBe(40);
    expect(detail.invoice.balance_due).toBe(60);

    // Settle the rest.
    await api.request.post("/api/payments", {
      data: {
        patient_id: patient.id, method: "transfer", amount: 60,
        allocations: [{ invoice_id: inv.id, amount: 60 }],
      },
    });
    const final = await (await api.request.get(`/api/invoices/${inv.id}`)).json();
    expect(final.invoice.status).toBe("paid");
    expect(final.invoice.balance_due).toBe(0);
  });

  test("cash payment without open register is rejected", async () => {
    const patient = await createPatient(api, {
      first_name: "NoKasa", last_name: "Hasta",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const inv = await createInvoice(api, {
      patient_id: patient.id,
      finalize: true,
      items: [{ code: "X", name: "Service", quantity: 1, unit_price: 100, vat_rate: 0 }],
    });

    // No register passed for cash → error.
    const r = await api.request.post("/api/payments", {
      data: {
        patient_id: patient.id, method: "cash", amount: 100,
        allocations: [{ invoice_id: inv.id, amount: 100 }],
      },
    });
    expect(r.status()).toBe(409);
  });
});

test.describe("hakediş — commission rules drive earnings", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("rule applies %30; earnings = paid line_total * 30%", async () => {
    const patient = await createPatient(api, {
      first_name: "Hak", last_name: "Hasta",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const specs = await listSpecializations(api);
    const doctor = await createDoctor(api, {
      staff: { first_name: "Kazanç", last_name: "Doktor", title: "Dr." },
      specialization_ids: [specs[0]!.id],
      primary_specialization_id: specs[0]!.id,
    });

    // Set a 30% rule for this doctor (valid from yesterday so any invoice
    // created today is within range).
    const yesterday = new Date(Date.now() - 24 * 3600 * 1000).toISOString().slice(0, 10);
    const ruleResp = await api.request.post(`/api/hakedis/${doctor.id}/rules`, {
      data: { commission_pct: 30, valid_from: yesterday },
    });
    expect(ruleResp.status()).toBe(201);

    // Invoice with one doctor-attributed line worth 1000 + 100 VAT = 1100.
    const inv = await createInvoice(api, {
      patient_id: patient.id,
      finalize: true,
      items: [{ code: "MUAYENE", name: "Muayene", quantity: 1, unit_price: 1000, vat_rate: 10, doctor_id: doctor.id }],
    });

    // Pay it in full so invoice flips to 'paid'.
    await api.request.post("/api/payments", {
      data: {
        patient_id: patient.id, method: "card", amount: 1100,
        allocations: [{ invoice_id: inv.id, amount: 1100 }],
      },
    });

    // Pull hakediş summary (default = current month).
    const sum = await api.request.get("/api/hakedis");
    expect(sum.ok()).toBeTruthy();
    const rows = (await sum.json()) as Array<{
      doctor_id: string; gross_revenue: number; earning_total: number; item_count: number;
    }>;
    const row = rows.find((r) => r.doctor_id === doctor.id);
    expect(row).toBeTruthy();
    // Gross uses line_total (1100), commission applies to that → 1100 * 0.30 = 330.
    expect(row!.gross_revenue).toBe(1100);
    expect(row!.item_count).toBe(1);
    expect(row!.earning_total).toBeCloseTo(330, 2);
  });
});
