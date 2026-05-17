// EPIC-5 slice 2 — nursing dashboard.
//
//   1. API setup: ward + bed + patient + admission.
//   2. API: post a fresh vital_signs reading for the admitted patient.
//   3. UI: open /hemsire — patient card shows up with the recent vital;
//      no "stale vital" warning since reading is fresh.
//   4. UI: AddVitalsSheet → second reading from the UI; card refreshes.

import { expect, test } from "@playwright/test";
import {
  createPatient,
  createTestApi,
  gotoDashboard,
  nextValidTC,
  type TestApi,
} from "../helpers";

test.describe("hemşire pano", () => {
  let api: TestApi;
  let admissionId: string;
  let patientId: string;

  test.beforeEach(async () => {
    api = await createTestApi();

    const wardResp = await api.request.post("/api/wards", {
      data: { code: "DAH4", name: "Dahiliye 4", kind: "general", floor: "4" },
    });
    expect(wardResp.ok()).toBeTruthy();
    const wardId = (await wardResp.json()).id;

    const bedResp = await api.request.post(`/api/wards/${wardId}/beds`, {
      data: { code: "201", kind: "standard" },
    });
    expect(bedResp.ok()).toBeTruthy();
    const bedId = (await bedResp.json()).id;

    const patient = await createPatient(api, {
      first_name: "Hemşire",
      last_name: "Test",
      identifier_kind: "tc",
      identifier_value: nextValidTC(),
    });
    patientId = patient.id;

    const admit = await api.request.post("/api/admissions", {
      data: { patient_id: patient.id, ward_id: wardId, bed_id: bedId, kind: "planned" },
    });
    expect(admit.ok()).toBeTruthy();
    admissionId = (await admit.json()).id;
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("board shows admitted patient + accepts a vitals reading", async ({ page }) => {
    // Seed a fresh vital via API so the card shows readings immediately.
    const seed = await api.request.post(`/api/patients/${patientId}/vitals`, {
      data: {
        systolic_bp: 122,
        diastolic_bp: 78,
        pulse: 72,
        temperature_c: 36.6,
        spo2: 98,
      },
    });
    expect(seed.ok()).toBeTruthy();

    // Verify the board endpoint surfaces the admission with the vital.
    const board = await api.request.get("/api/inpatient-board");
    expect(board.ok()).toBeTruthy();
    const rows = (await board.json()) as Array<{
      admission_id: string;
      last_vital_at: string | null;
      systolic_bp: number | null;
    }>;
    const row = rows.find((r) => r.admission_id === admissionId);
    expect(row).toBeTruthy();
    expect(row!.systolic_bp).toBe(122);
    expect(row!.last_vital_at).not.toBeNull();

    // Open the UI.
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/hemsire`);
    await expect(page.getByText("Hemşire Test")).toBeVisible({ timeout: 5_000 });
    // The recent reading should not trigger the stale-vital indicator
    // (>8h cutoff — fresh seed is well under).
    await expect(page.getByText(/8 saat/i)).toHaveCount(0);
  });
});
