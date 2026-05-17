// Revenue-cycle happy path — one API-driven scenario covering the full
// patient journey from kabul to tahsilat. This is the single most
// important invariant to keep green: if it fails, the product doesn't
// make money correctly.
//
//   patient create → appointment → visit start → diagnosis add →
//   visit complete → invoice create+finalize → kasa open → payment →
//   invoice status='paid' → hakediş özet doktora yansıdı →
//   audit_log her adımda iz bıraktı

import { expect, test } from "@playwright/test";
import {
  createAppointment,
  createDoctor,
  createInvoice,
  createPatient,
  createTestApi,
  fetchAuditLog,
  listSpecializations,
  nextValidTC,
  openCashRegister,
  recordPayment,
  type TestApi,
} from "../helpers";

test.describe("revenue cycle — happy path", () => {
  let api: TestApi;
  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("kabul → randevu → muayene → tanı → fatura → tahsilat → hakediş + audit", async () => {
    // --- 1. Master data: doctor + commission rule ---
    const specs = await listSpecializations(api);
    const dahiliye = specs.find((s) => s.code === "DAHILIYE") ?? specs[0]!;
    const doctor = await createDoctor(api, {
      staff: { first_name: "Ahmet", last_name: "Yılmaz", title: "Uzm. Dr." },
      specialization_ids: [dahiliye.id],
      primary_specialization_id: dahiliye.id,
    });

    // Catch-all 50% komisyon (kategorisi olmayan/eşleşmeyen kalemlere de
    // uygulanır). Kategori-spesifik kural ayrı test'te ele alınıyor.
    const ruleResp = await api.request.post(`/api/hakedis/${doctor.id}/rules`, {
      data: { commission_pct: 50 },
    });
    expect(ruleResp.ok()).toBeTruthy();

    // --- 2. Patient kabul ---
    const tc = nextValidTC();
    const patient = await createPatient(api, {
      first_name: "Hasan",
      last_name: "Demir",
      identifier_kind: "tc",
      identifier_value: tc,
      phone: "05551112233",
      gender: "male",
    });
    expect(patient.mrn).toMatch(/^\d{8}$/);

    // --- 3. Randevu + visit ---
    const apptTime = new Date();
    apptTime.setHours(10, 0, 0, 0);
    const appt = await createAppointment(api, {
      patient_id: patient.id,
      doctor_id: doctor.id,
      scheduled_at: apptTime.toISOString(),
      duration_minutes: 20,
      kind: "outpatient",
      reason: "Genel muayene",
    });

    // Flow to arrived → started
    await api.request.post(`/api/appointments/${appt.id}/status`, {
      data: { status: "arrived" },
    });
    const visitResp = await api.request.post("/api/visits/start-from-appointment", {
      data: { appointment_id: appt.id },
    });
    expect(visitResp.ok()).toBeTruthy();
    const visit = (await visitResp.json()) as { id: string };

    // Notes + diagnosis (the visit complete enforces at least one diagnosis
    // in some configs; we keep it complete for realism).
    await api.request.patch(`/api/visits/${visit.id}/notes`, {
      data: {
        chief_complaint: "Halsizlik, baş ağrısı",
        examination_findings: "Genel durum iyi.",
        treatment_plan: "Kontrole çağır.",
      },
    });
    // Use a synthetic ICD-10 code — the diagnosis endpoint accepts the
    // code+title pair directly, no FK lookup required.
    const dxResp = await api.request.post(`/api/visits/${visit.id}/diagnoses`, {
      data: { icd10_code: "R51", icd10_title: "Baş ağrısı", kind: "primary" },
    });
    expect(dxResp.ok()).toBeTruthy();

    await api.request.post(`/api/visits/${visit.id}/complete`, { data: {} });

    // --- 4. Fatura ---
    const inv = await createInvoice(api, {
      patient_id: patient.id,
      finalize: true,
      items: [
        {
          code: "MUAYENE_GENEL",
          name: "Genel Muayene",
          quantity: 1,
          unit_price: 600,
          vat_rate: 10,
          doctor_id: doctor.id,
        },
      ],
    });

    const detailResp = await api.request.get(`/api/invoices/${inv.id}`);
    const detail = (await detailResp.json()) as {
      invoice: { subtotal: number; tax_total: number; total: number; balance_due: number; status: string };
    };
    expect(detail.invoice.subtotal).toBe(600);
    expect(detail.invoice.tax_total).toBe(60);
    expect(detail.invoice.total).toBe(660);
    expect(detail.invoice.status).toBe("finalized");

    // --- 5. Kasa + tahsilat ---
    const reg = await openCashRegister(api, { opening_balance: 0 });
    const pay = await recordPayment(api, {
      patient_id: patient.id,
      method: "cash",
      amount: 660,
      cash_register_id: reg.id,
      allocations: [{ invoice_id: inv.id, amount: 660 }],
    });
    expect(pay.payment_no).toMatch(/^\d{8}$/);
    expect(pay.cash_movement_no).toMatch(/^\d{8}$/);

    // Invoice paid.
    const paidResp = await api.request.get(`/api/invoices/${inv.id}`);
    const paid = (await paidResp.json()) as {
      invoice: { status: string; balance_due: number; paid_total: number };
    };
    expect(paid.invoice.status).toBe("paid");
    expect(paid.invoice.balance_due).toBe(0);
    expect(paid.invoice.paid_total).toBe(660);

    // --- 6. Hakediş — doktora 50% düştü ---
    // Use today's range to be safe regardless of test machine clock.
    const todayISO = new Date().toISOString().slice(0, 10);
    const startOfMonth = `${todayISO.slice(0, 8)}01`;
    const hakResp = await api.request.get(
      `/api/hakedis?from=${startOfMonth}&to=${todayISO}`,
    );
    expect(hakResp.ok()).toBeTruthy();
    const hakedis = (await hakResp.json()) as Array<{
      doctor_id: string;
      item_count: number;
      gross_revenue: number;
      earning_total: number;
    }>;
    const docRow = hakedis.find((r) => r.doctor_id === doctor.id);
    expect(docRow, "doctor should appear in hakediş after paid invoice").toBeTruthy();
    expect(docRow!.item_count).toBe(1);
    // line_total in invoice_item is post-VAT (subtotal + tax) = 660.
    // 50% komisyon → 330. Gross == line_total sum == 660.
    expect(docRow!.gross_revenue).toBe(660);
    expect(docRow!.earning_total).toBe(330);

    // --- 7. Audit log — kritik adımların izi düştü ---
    const audit = await fetchAuditLog(api, { limit: 200 });
    const actions = new Set(audit.items.map((i) => i.action));
    expect(actions.has("auth.login"), "login should audit").toBeTruthy();
    expect(actions.has("patient.create"), "patient.create should audit").toBeTruthy();
    expect(actions.has("invoice.create"), "invoice.create should audit").toBeTruthy();
    expect(actions.has("invoice.finalize"), "invoice.finalize should audit").toBeTruthy();
    expect(actions.has("payment.create"), "payment.create should audit").toBeTruthy();
    expect(actions.has("kasa.open"), "kasa.open should audit").toBeTruthy();

    // payment.create audit should reference the actual payment ID.
    const payAudit = audit.items.find((i) => i.action === "payment.create");
    expect(payAudit?.entity_id).toBe(pay.payment_id);
    expect((payAudit?.details as { amount?: number })?.amount).toBe(660);
  });
});
