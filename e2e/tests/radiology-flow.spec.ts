// EPIC-4 slice 2 — radiology workflow:
//   doctor visit → "Görüntüleme" tab → pick a procedure (XR Akciğer) →
//   order created → /radyoloji queue → tech flips to Çekildi →
//   radiologist writes findings + impression + saves → Raporu Onayla →
//   the report surfaces back on the visit's Görüntüleme tab.

import { expect, test } from "@playwright/test";
import {
  createDoctor,
  createPatient,
  createTestApi,
  gotoDashboard,
  listSpecializations,
  nextValidTC,
  type TestApi,
} from "../helpers";

test.describe("radyoloji — order + acquire + report", () => {
  let api: TestApi;

  test.beforeEach(async () => {
    api = await createTestApi();
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("akciğer grafisi: doctor orders → tech acquires → radiologist reports", async ({ page }) => {
    // --- Setup: patient + doctor + open visit (API) ---
    const patient = await createPatient(api, {
      first_name: "Rad",
      last_name: "Hasta",
      identifier_kind: "tc",
      identifier_value: nextValidTC(),
    });
    const specs = await listSpecializations(api);
    const acil = specs.find((s) => s.code === "ACIL_TIP")!;
    const doctor = await createDoctor(api, {
      staff: { first_name: "Rad", last_name: "Doktor", title: "Uzm. Dr." },
      specialization_ids: [acil.id],
      primary_specialization_id: acil.id,
    });
    const now = new Date();
    const apptResp = await api.request.post("/api/appointments", {
      data: {
        patient_id: patient.id,
        doctor_id: doctor.id,
        scheduled_at: now.toISOString(),
        duration_minutes: 20,
        kind: "outpatient",
      },
    });
    const appt = await apptResp.json();
    await api.request.post(`/api/appointments/${appt.id}/status`, { data: { status: "arrived" } });
    const visitResp = await api.request.post("/api/visits/start-from-appointment", {
      data: { appointment_id: appt.id },
    });
    const visit = await visitResp.json();

    // --- Doctor orders an akciğer grafisi from the visit's Görüntüleme tab ---
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/poliklinik/${visit.id}`);

    await page.getByRole("button", { name: /görüntüleme/i }).click();
    await page.getByPlaceholder(/tetkik ara/i).fill("Akciğer");
    await page.getByText(/Akciğer Grafisi \(PA\)/i).first().click();

    await page.getByLabel(/klinik gerekçe/i).fill("Öksürük 3 gündür");
    await page.getByLabel(/klinik soru/i).fill("Pnömoni var mı?");
    await page.getByRole("button", { name: /görüntüleme isteği oluştur/i }).click();

    await expect(page.locator("code").filter({ hasText: /^\d{8}$/ }).first()).toBeVisible({
      timeout: 5_000,
    });
    await expect(page.getByText(/Akciğer Grafisi/i).first()).toBeVisible();

    // --- /radyoloji kuyruğu picks the order up ---
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/radyoloji`);
    await expect(page.getByText("Rad Hasta")).toBeVisible();
    await expect(page.getByText("İstendi")).toBeVisible();

    // Open and acquire.
    await page.getByRole("button", { name: /^aç$/i }).first().click();
    await page.waitForURL(/\/radyoloji\/[a-f0-9-]+/, { timeout: 10_000 });

    await page.getByRole("button", { name: /çekim tamamlandı/i }).click();
    await expect(page.getByText(/acquired/i).first()).toBeVisible({ timeout: 5_000 });

    // Write the report.
    await page.getByLabel(/bulgular/i).fill(
      "Akciğer parankim alanları havalı. Sinüs frenik açıklar serbest. Kalp gölgesi normal.",
    );
    await page.getByLabel(/sonuç/i).fill("Normal akciğer grafisi.");
    await page.getByLabel(/öneriler/i).fill("İleri tetkik gerekmez.");
    await page.getByRole("button", { name: /raporu kaydet/i }).click();

    await expect(page.getByText(/kaydedildi/i)).toBeVisible({ timeout: 5_000 });

    // Verify the report.
    await page.getByRole("button", { name: /raporu onayla/i }).click();
    await expect(page.getByText(/onaylandı/i)).toBeVisible({ timeout: 5_000 });

    // --- Back on the visit → Görüntüleme tab shows the impression summary ---
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/poliklinik/${visit.id}`);
    await page.getByRole("button", { name: /görüntüleme/i }).click();
    await expect(page.getByText(/Normal akciğer grafisi/i)).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/verified/i)).toBeVisible();
  });
});
