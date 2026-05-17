// HL7 ORU^R01 lab autoanalizör inbound.
//
// Acceptance:
//   1. Doktor lab istek oluşturur (HGB + GLU) → order_no + items present.
//   2. Lab middleware bir ORU^R01 mesajı POST eder → branch-scoped ingest.
//   3. matched_observations == 2, lab_order_item.value_numeric güncellendi,
//      flag enum'a maplenirdi (H → high), order status → 'resulted'.
//   4. Unknown test_code → unmatched listesinde geri döner.

import { expect, test } from "@playwright/test";
import {
  createDoctor,
  createPatient,
  createTestApi,
  listSpecializations,
  nextValidTC,
  postHL7Result,
  startVisitFor,
  type TestApi,
} from "../helpers";

test.describe("HL7 ORU^R01 lab inbound", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("autoanalizör mesajı → 2/2 sonuç eşleşir, order 'resulted'", async () => {
    // ---- Setup: patient + doctor + visit + lab order (HGB + GLU) ----
    const patient = await createPatient(api, {
      first_name: "HL7", last_name: "Hasta",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const specs = await listSpecializations(api);
    const doctor = await createDoctor(api, {
      staff: { first_name: "Lab", last_name: "Dr" },
      specialization_ids: [specs[0]!.id],
      primary_specialization_id: specs[0]!.id,
    });
    const { visitId } = await startVisitFor(api, patient.id, doctor.id);

    // Look up HGB and GLU from seeded lab catalog.
    const findTest = async (q: string) => {
      const r = await api.request.get(`/api/lab-tests?q=${encodeURIComponent(q)}`);
      expect(r.ok()).toBeTruthy();
      const rows = (await r.json()) as Array<{ id: string; code: string; name: string }>;
      return rows[0];
    };
    const hgb = await findTest("HGB");
    const glu = await findTest("GLU");
    expect(hgb).toBeTruthy();
    expect(glu).toBeTruthy();

    const orderResp = await api.request.post("/api/lab-orders", {
      data: {
        visit_id: visitId,
        patient_id: patient.id,
        ordering_doctor_id: doctor.id,
        sample_type: "blood",
        items: [
          { lab_test_catalog_id: hgb!.id },
          { lab_test_catalog_id: glu!.id },
        ],
      },
    });
    expect(orderResp.status()).toBe(201);
    const order = (await orderResp.json()) as { id: string; order_no: string };

    // ---- POST HL7 message ----
    const msg = [
      `MSH|^~\\&|LIS-Roche|HOSPITAL|MediGt|HOSPITAL|20260517141522||ORU^R01|MSG-E2E-001|P|2.5`,
      `PID|||${patient.id}||DOE^JOHN||19850101|M`,
      `PV1||O`,
      `ORC|RE|${order.order_no}`,
      `OBR|1|${order.order_no}|ROCHE-99001|HEMOGRAM^Hemogram|||20260517140000`,
      `OBX|1|NM|HGB^Hemoglobin||14.2|g/dL|13.0-17.0|N|||F`,
      `OBX|2|NM|GLU^Glukoz||145|mg/dL|70-110|H|||F`,
    ].join("\r");

    const result = await postHL7Result(api, msg);
    expect(result.matched_observations).toBe(2);
    expect(result.order_no).toBe(order.order_no);

    // ---- Verify lab_order_item values updated ----
    const detail = await api.request.get(`/api/lab-orders/${order.id}`);
    expect(detail.ok()).toBeTruthy();
    const detailBody = (await detail.json()) as {
      status: string;
      items: Array<{ test_code: string; value_numeric: number | null; flag: string | null; status: string }>;
    };
    const hgbItem = detailBody.items.find((i) => i.test_code === "HGB")!;
    const gluItem = detailBody.items.find((i) => i.test_code === "GLU")!;
    expect(hgbItem.value_numeric).toBe(14.2);
    expect(hgbItem.flag).toBe("normal");
    expect(hgbItem.status).toBe("resulted");
    expect(gluItem.value_numeric).toBe(145);
    expect(gluItem.flag).toBe("high");
    expect(detailBody.status).toBe("resulted"); // parent flipped
  });

  test("unknown test_code → unmatched, hata yok", async () => {
    const patient = await createPatient(api, {
      first_name: "Skip", last_name: "Test",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const specs = await listSpecializations(api);
    const doctor = await createDoctor(api, {
      staff: { first_name: "S", last_name: "S" },
      specialization_ids: [specs[0]!.id],
      primary_specialization_id: specs[0]!.id,
    });
    const { visitId } = await startVisitFor(api, patient.id, doctor.id);

    const r = await api.request.get("/api/lab-tests?q=HGB");
    const hgb = (await r.json())[0];

    const orderResp = await api.request.post("/api/lab-orders", {
      data: {
        visit_id: visitId,
        patient_id: patient.id,
        ordering_doctor_id: doctor.id,
        sample_type: "blood",
        items: [{ lab_test_catalog_id: hgb.id }],
      },
    });
    const order = await orderResp.json();

    const msg = [
      `MSH|^~\\&|LIS|HOSPITAL|MediGt|HOSPITAL|20260517141522||ORU^R01|UN-001|P|2.5`,
      `PID|||${patient.id}||X^Y||19850101|M`,
      `OBR|1|${order.order_no}|ACC|HEMOGRAM`,
      `OBX|1|NM|HGB^Hemoglobin||14.0|g/dL|13-17|N|||F`,
      `OBX|2|NM|FOO^Bilinmeyen||99|||N|||F`,
    ].join("\r");

    const out = await postHL7Result(api, msg);
    expect(out.matched_observations).toBe(1);
    expect(out.unmatched_observations?.[0]?.test_code).toBe("FOO");
  });
});
