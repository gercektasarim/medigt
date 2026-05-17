// EPIC-6 — pharmacy + warehouse + dispense.
//
// Invariants under test:
//   - Stock totals decrement atomically with dispense.
//   - FEFO order: lot with the earliest expiry is offered first.
//   - prescription_dispense + stock_movement are inserted in the same tx
//     (we observe both via API after a successful dispense).

import { expect, test } from "@playwright/test";
import {
  createDoctor,
  createMedication,
  createPatient,
  createTestApi,
  createWarehouse,
  listSpecializations,
  nextValidTC,
  receiveStock,
  startVisitFor,
  type TestApi,
} from "../helpers";

test.describe("eczane — catalog + lot stock + FEFO dispense", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("medication catalog round-trips", async () => {
    const med = await createMedication(api, {
      name: "Parol 500 mg tablet",
      generic_name: "Parasetamol",
      atc_code: "N02BE01",
      form: "tablet",
      strength: "500 mg",
      prescription_class: "normal",
    });
    expect(med.id).toBeTruthy();

    const list = await api.request.get("/api/medications?q=parol");
    expect(list.ok()).toBeTruthy();
    const rows = (await list.json()) as Array<{ id: string; name: string }>;
    expect(rows.find((r) => r.id === med.id)).toBeTruthy();
  });

  test("receive 2 lots then list shows them in FEFO order", async () => {
    const wh = await createWarehouse(api, { code: "ECZ1", name: "Eczane Deposu", kind: "pharmacy" });
    const med = await createMedication(api, {
      name: "Heparin 5000 IU/ml",
      generic_name: "Heparin sodyum",
      atc_code: "B01AB01",
      form: "injection",
      strength: "5000 IU/ml",
      requires_cold_chain: true,
    } as any);

    // Late expiry first
    await receiveStock(api, {
      warehouse_id: wh.id,
      medication_id: med.id,
      lot_no: "LOT-LATE",
      expiry_date: "2027-12-31",
      quantity: 10,
      unit_price: 25.5,
      counterparty: "Lab A.Ş.",
    });
    // Early expiry second — FEFO should still surface this one first.
    await receiveStock(api, {
      warehouse_id: wh.id,
      medication_id: med.id,
      lot_no: "LOT-EARLY",
      expiry_date: "2026-06-30",
      quantity: 5,
      unit_price: 25.5,
    });

    const fefoResp = await api.request.get(
      `/api/eczane/fefo?warehouse_id=${wh.id}&medication_id=${med.id}`,
    );
    expect(fefoResp.ok()).toBeTruthy();
    const lots = (await fefoResp.json()) as Array<{ lot_no: string; quantity: number }>;
    expect(lots.length).toBe(2);
    expect(lots[0]!.lot_no).toBe("LOT-EARLY"); // FEFO winner
    expect(lots[1]!.lot_no).toBe("LOT-LATE");

    // Stock list also surfaces both lots, q > 0 by default.
    const stock = await api.request.get(`/api/stock?warehouse_id=${wh.id}`);
    const stockRows = (await stock.json()) as Array<{ lot_no: string; quantity: number }>;
    const earlyRow = stockRows.find((r) => r.lot_no === "LOT-EARLY");
    expect(earlyRow!.quantity).toBe(5);
  });

  test("dispense decrements stock + writes audit rows", async () => {
    // Set up: patient + doctor + visit + signed prescription with one item.
    const wh = await createWarehouse(api, { code: "ECZ2", name: "Eczane", kind: "pharmacy" });
    const med = await createMedication(api, {
      name: "Augmentin 1g tablet",
      generic_name: "Amoksisilin/Klavulanat",
      atc_code: "J01CR02",
      form: "tablet",
      strength: "1 g",
    });
    await receiveStock(api, {
      warehouse_id: wh.id,
      medication_id: med.id,
      lot_no: "L240312",
      expiry_date: "2027-03-31",
      quantity: 20,
      unit_price: 80.0,
    });

    const patient = await createPatient(api, {
      first_name: "Disp", last_name: "Hasta",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const specs = await listSpecializations(api);
    const doctor = await createDoctor(api, {
      staff: { first_name: "Disp", last_name: "Dr", title: "Dr." },
      specialization_ids: [specs[0]!.id],
      primary_specialization_id: specs[0]!.id,
    });
    const { visitId } = await startVisitFor(api, patient.id, doctor.id);

    // Create the prescription with one item (free-text drug name; dispense
    // time the eczane will link it to the catalog row).
    const rxResp = await api.request.post(`/api/visits/${visitId}/prescriptions`, {
      data: {
        items: [
          {
            medication_name: "Augmentin 1g 2x1",
            dosage: "1 g",
            frequency: "günde 2 kez",
            duration_days: 7,
            quantity: "14 tablet",
            instructions: "yemekten sonra",
          },
        ],
      },
    });
    expect(rxResp.ok()).toBeTruthy();
    const rx = (await rxResp.json()) as { id: string; items: Array<{ id: string }> };
    const rxItemId = rx.items[0]!.id;

    // Sign the prescription (now eligible for dispense).
    const signResp = await api.request.post(`/api/prescriptions/${rx.id}/sign`, { data: {} });
    expect(signResp.ok()).toBeTruthy();

    // Pending queue surfaces it.
    const pending = await api.request.get("/api/eczane/pending");
    expect(pending.ok()).toBeTruthy();
    const pendingRows = (await pending.json()) as Array<{ id: string }>;
    expect(pendingRows.find((r) => r.id === rx.id)).toBeTruthy();

    // Dispense 14 tablets.
    const dispResp = await api.request.post(
      `/api/prescription-items/${rxItemId}/dispense`,
      {
        data: {
          medication_id: med.id,
          warehouse_id: wh.id,
          lot_no: "L240312",
          expiry_date: "2027-03-31",
          quantity: 14,
        },
      },
    );
    expect(dispResp.status()).toBe(201);
    const dispResult = (await dispResp.json()) as { movement_no: string };
    expect(dispResult.movement_no).toMatch(/^\d{8}$/);

    // Stock decremented: 20 - 14 = 6.
    const stock = await api.request.get(`/api/stock?warehouse_id=${wh.id}`);
    const stockRows = (await stock.json()) as Array<{ lot_no: string; quantity: number }>;
    const lot = stockRows.find((r) => r.lot_no === "L240312");
    expect(lot!.quantity).toBe(6);

    // Movement audit row exists with kind=issue.
    const mvts = await api.request.get(`/api/stock-movements?warehouse_id=${wh.id}&kind=issue`);
    const mvtRows = (await mvts.json()) as Array<{ kind: string; quantity: number; movement_no: string }>;
    const issueRow = mvtRows.find((m) => m.movement_no === dispResult.movement_no);
    expect(issueRow).toBeTruthy();
    expect(issueRow!.quantity).toBe(14);

    // Dispense history shows the row.
    const hist = await api.request.get("/api/eczane/history");
    const histRows = (await hist.json()) as Array<{ movement_no: string; quantity: number }>;
    expect(histRows.find((h) => h.movement_no === dispResult.movement_no)).toBeTruthy();
  });

  test("dispense refuses insufficient stock", async () => {
    const wh = await createWarehouse(api, { code: "ECZ3", name: "Eczane 3", kind: "pharmacy" });
    const med = await createMedication(api, {
      name: "Aspirin 100 mg",
      generic_name: "Asetilsalisilik asit",
      form: "tablet",
    });
    await receiveStock(api, {
      warehouse_id: wh.id,
      medication_id: med.id,
      lot_no: "L1",
      quantity: 3,
    });

    const patient = await createPatient(api, {
      first_name: "Short", last_name: "Stock",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const specs = await listSpecializations(api);
    const doctor = await createDoctor(api, {
      staff: { first_name: "S", last_name: "S" },
      specialization_ids: [specs[0]!.id],
      primary_specialization_id: specs[0]!.id,
    });
    const { visitId } = await startVisitFor(api, patient.id, doctor.id);
    const rxResp = await api.request.post(`/api/visits/${visitId}/prescriptions`, {
      data: { items: [{ medication_name: "Aspirin" }] },
    });
    const rx = await rxResp.json();
    await api.request.post(`/api/prescriptions/${rx.id}/sign`, { data: {} });

    // Try to dispense 10 from a lot that only has 3.
    const bad = await api.request.post(
      `/api/prescription-items/${rx.items[0].id}/dispense`,
      {
        data: { medication_id: med.id, warehouse_id: wh.id, lot_no: "L1", quantity: 10 },
      },
    );
    expect(bad.status()).toBe(409);
    const body = await bad.json();
    expect(body.error.code).toBe("insufficient_stock");
  });
});
