// PACS DICOM viewer integration.
//
// Acceptance:
//   - Radiology order created → background goroutine schedules a Study UID
//     in PACS (mock) and writes image_reference within ~1s.
//   - GET /api/radiology-orders/{id}/images returns the row with:
//     * study_instance_uid başlıyor "1.2.826.0.1.3680043.10.1234." (OrgRootOID)
//     * modality eşleşiyor order'ın modalitesi ile
//     * viewer_url OHIF demo formatı: viewer.ohif.org/viewer?StudyInstanceUIDs=...
//   - radiology_order.pacs_study_uid back-fill edildi.

import { expect, test } from "@playwright/test";
import {
  createDoctor,
  createPatient,
  createTestApi,
  listSpecializations,
  nextValidTC,
  startVisitFor,
  type TestApi,
} from "../helpers";

test.describe("PACS — radiology order auto-schedules study UID", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("order create → image_reference + study UID appear", async () => {
    const patient = await createPatient(api, {
      first_name: "PACS", last_name: "Test",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const specs = await listSpecializations(api);
    const doctor = await createDoctor(api, {
      staff: { first_name: "Rad", last_name: "Dr" },
      specialization_ids: [specs[0]!.id],
      primary_specialization_id: specs[0]!.id,
    });
    const { visitId } = await startVisitFor(api, patient.id, doctor.id);

    // Look up a CR (akciğer grafisi) procedure from the seeded catalog.
    const procResp = await api.request.get("/api/radiology-procedures?q=akciğer&modality=CR");
    expect(procResp.ok()).toBeTruthy();
    const procedures = (await procResp.json()) as Array<{ id: string; code: string; name: string; modality: string }>;
    expect(procedures.length).toBeGreaterThan(0);
    const procedure = procedures[0]!;

    const orderResp = await api.request.post("/api/radiology-orders", {
      data: {
        patient_id: patient.id,
        visit_id: visitId,
        ordering_doctor_id: doctor.id,
        procedure_id: procedure.id,
        priority: "routine",
        clinical_indication: "kontrol",
      },
    });
    expect(orderResp.status()).toBe(201);
    const order = await orderResp.json();

    // Poll the images endpoint — backend hooks PACS in a goroutine.
    let images: Array<{ study_instance_uid: string; modality: string; viewer_url: string }> = [];
    for (let i = 0; i < 20; i++) {
      const r = await api.request.get(`/api/radiology-orders/${order.id}/images`);
      expect(r.ok()).toBeTruthy();
      images = await r.json();
      if (images.length > 0) break;
      await new Promise((res) => setTimeout(res, 300));
    }

    expect(images.length).toBeGreaterThan(0);
    const img = images[0]!;
    expect(img.study_instance_uid).toMatch(/^1\.2\.826\.0\.1\.3680043\.10\.1234\./);
    expect(img.modality).toBe(procedure.modality);
    expect(img.viewer_url).toContain("viewer.ohif.org/viewer");
    expect(img.viewer_url).toContain(`StudyInstanceUIDs=${img.study_instance_uid}`);

    // Order itself should have the UID back-filled.
    const orderDetail = await api.request.get(`/api/radiology-orders/${order.id}`);
    const detail = await orderDetail.json();
    expect(detail.pacs_study_uid).toBe(img.study_instance_uid);
  });
});
