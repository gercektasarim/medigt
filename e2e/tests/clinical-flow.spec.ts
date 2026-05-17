// End-to-end happy path through the clinical encounter:
//
//   appointment (arrived) → "Muayeneye al" → visit detail opens →
//   vital ekle → anamnez yaz → ICD-10 tanı seç → reçete oluştur →
//   reçeteyi imzala → muayeneyi tamamla → randevu de "Tamamlandı".
//
// This is the most expensive spec — it covers the whole EPIC-3 surface.

import { expect, test } from "@playwright/test";
import {
  createAppointment,
  createDoctor,
  createPatient,
  createTestApi,
  gotoDashboard,
  listSpecializations,
  nextValidTC,
  type TestApi,
} from "../helpers";

test.describe("clinical flow — end to end", () => {
  let api: TestApi;

  test.beforeEach(async () => {
    api = await createTestApi();
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("randevu → muayene → vital → tanı → reçete → imza → tamamla", async ({ page }) => {
    // --- Setup over the API ---
    const patient = await createPatient(api, {
      first_name: "Mehmet",
      last_name: "Kaya",
      identifier_kind: "tc",
      identifier_value: nextValidTC(),
      phone: "05559998877",
      gender: "male",
      blood_type: "A_pos",
    });
    const specs = await listSpecializations(api);
    const kardiyo = specs.find((s) => s.code === "KARDIYOLOJI")!;
    const doctor = await createDoctor(api, {
      staff: { first_name: "Selin", last_name: "Yıldız", title: "Uzm. Dr." },
      specialization_ids: [kardiyo.id],
      primary_specialization_id: kardiyo.id,
    });
    const now = new Date();
    now.setHours(9, 0, 0, 0);
    const appt = await createAppointment(api, {
      patient_id: patient.id,
      doctor_id: doctor.id,
      scheduled_at: now.toISOString(),
      duration_minutes: 20,
      kind: "outpatient",
      reason: "Göğüs ağrısı",
    });
    // Flip to 'arrived' so it shows up in the waiting room.
    await api.request.post(`/api/appointments/${appt.id}/status`, {
      data: { status: "arrived" },
    });

    // --- Poliklinik queue ---
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/poliklinik`);

    await expect(page.getByText("Mehmet Kaya")).toBeVisible({ timeout: 5_000 });

    // "Muayeneye al" → visit oluşur, doktor ekranı açılır.
    await page.getByRole("button", { name: /muayeneye al/i }).click();
    await page.waitForURL(/\/poliklinik\/[a-f0-9-]+/, { timeout: 10_000 });

    // --- Patient panel sanity check ---
    await expect(page.getByText("Mehmet Kaya")).toBeVisible();
    await expect(page.getByText(/MRN\s+\d{8}/)).toBeVisible();

    // --- Anamnez tab (default) ---
    await page.getByLabel(/chief complaint/i).fill("3 gündür göğüs ağrısı.");
    await page.getByLabel(/mevcut hastalık hikayesi/i).fill("Aralıklı, eforla artan.");
    await page.getByLabel(/fizik muayene/i).fill("Genel durum iyi. TA: 130/85.");
    await page.getByLabel(/tedavi planı/i).fill("EKG + troponin → kontrole çağır.");
    await page.getByRole("button", { name: /notları kaydet/i }).click();
    await expect(page.getByText(/kaydedildi/i)).toBeVisible({ timeout: 5_000 });

    // --- Vital tab ---
    await page.getByRole("button", { name: /vital bulgular/i }).click();
    await page.getByLabel("TA Sistolik").fill("130");
    await page.getByLabel("TA Diastolik").fill("85");
    await page.getByLabel("Nabız").fill("78");
    await page.getByLabel("Ateş").fill("36.7");
    await page.getByLabel("SpO₂").fill("98");
    await page.getByRole("button", { name: /ölçüm ekle/i }).click();
    await expect(page.getByText(/130\/85 mmHg/)).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/78 bpm/)).toBeVisible();

    // --- Tanı tab — pick I10 (Hipertansiyon) from the ICD-10 picker ---
    await page.getByRole("button", { name: /tanılar/i }).click();
    const icdSearch = page.getByPlaceholder(/icd-10 ara/i);
    await icdSearch.fill("I10");
    await page.getByText(/hipertansiyon/i).first().click();
    await page.getByRole("button", { name: /tanıyı ekle/i }).click();

    await expect(page.getByText("I10").first()).toBeVisible();
    await expect(page.getByText(/hipertansiyon/i)).toBeVisible();

    // --- Reçete tab — single drug, draft → sign ---
    await page.getByRole("button", { name: /reçeteler/i }).click();
    await page.getByLabel(/ilaç 1/i).fill("Norvasc 5 mg");
    await page.getByLabel(/^doz$/i).first().fill("5 mg");
    await page.getByLabel(/sıklık/i).first().fill("günde 1 kez");
    await page.getByLabel(/süre \(gün\)/i).first().fill("30");
    await page.getByLabel(/miktar/i).first().fill("1 kutu");
    await page.getByRole("button", { name: /taslak reçete/i }).click();

    // Prescription number visible (8-digit) and draft badge.
    await expect(page.locator("code").filter({ hasText: /^\d{8}$/ }).first()).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/^taslak$/i)).toBeVisible();

    // Sign
    await page.getByRole("button", { name: /^imzala$/i }).click();
    await expect(page.getByText(/imzalandı/i)).toBeVisible({ timeout: 5_000 });

    // --- Complete the visit ---
    await page.getByRole("button", { name: /muayeneyi tamamla/i }).click();

    // Back on the queue page.
    await page.waitForURL(/\/poliklinik$/, { timeout: 10_000 });

    // The appointment now shows "Tamamlandı" on the randevu page (completeVisit
    // cascades the appointment too).
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/randevu`);
    await expect(page.getByText("Tamamlandı")).toBeVisible({ timeout: 5_000 });
  });
});
