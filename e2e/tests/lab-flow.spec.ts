// EPIC-4 slice 1 — lab workflow:
//   doctor visit → "Lab İstek" tab → pick tests → order created →
//   /laboratuvar queue shows the order → lab tech enters results →
//   order status walks ordered → resulted → verified → results
//   surface back in the visit's Lab tab.

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

test.describe("lab — order + result entry", () => {
  let api: TestApi;

  test.beforeEach(async () => {
    api = await createTestApi();
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("doctor orders a hemogram, lab fills the result, doctor sees it", async ({ page }) => {
    // --- Setup: patient + doctor + open visit via API ---
    const patient = await createPatient(api, {
      first_name: "Lab",
      last_name: "Hastası",
      identifier_kind: "tc",
      identifier_value: nextValidTC(),
    });
    const specs = await listSpecializations(api);
    const dahiliye = specs.find((s) => s.code === "IC_HASTALIKLARI")!;
    const doctor = await createDoctor(api, {
      staff: { first_name: "Lab", last_name: "Doktoru", title: "Uzm. Dr." },
      specialization_ids: [dahiliye.id],
      primary_specialization_id: dahiliye.id,
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
    await api.request.post(`/api/appointments/${appt.id}/status`, {
      data: { status: "arrived" },
    });
    const visitResp = await api.request.post("/api/visits/start-from-appointment", {
      data: { appointment_id: appt.id },
    });
    const visit = await visitResp.json();

    // --- Doctor: open visit, switch to Lab tab, pick HGB + GLU, submit ---
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/poliklinik/${visit.id}`);

    await page.getByRole("button", { name: /lab istek/i }).click();
    const testSearch = page.getByPlaceholder(/test ara/i);

    await testSearch.fill("HGB");
    await page.getByText("Hemoglobin").first().click();

    await testSearch.fill("GLU");
    await page.getByText(/Glukoz/i).first().click();

    await page.getByRole("button", { name: /lab istek oluştur/i }).click();

    // The freshly created order appears in the visit's Lab tab list.
    await expect(page.locator("code").filter({ hasText: /^\d{8}$/ }).first()).toBeVisible({
      timeout: 5_000,
    });
    await expect(page.getByText(/2 test/)).toBeVisible();

    // --- Lab queue picks the order up ---
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/laboratuvar`);
    await expect(page.getByText("Lab Hastası")).toBeVisible();
    await expect(page.getByText("İstendi")).toBeVisible();

    // Open the order detail.
    await page.getByRole("button", { name: /^aç$/i }).first().click();
    await page.waitForURL(/\/laboratuvar\/[a-f0-9-]+/, { timeout: 10_000 });

    // Numune Alındı → status flips.
    await page.getByRole("button", { name: /numune alındı/i }).click();
    await expect(page.getByText(/sampled/i)).toBeVisible({ timeout: 5_000 });

    // Enter HGB result.
    const hgbRow = page.locator("div").filter({ hasText: /^HGB.*Hemoglobin/ }).first();
    await hgbRow.getByRole("button", { name: /sonuç gir/i }).click();
    await hgbRow.getByLabel("Değer (sayısal)").fill("13.5");
    await hgbRow.getByLabel("Bayrak").selectOption({ label: "Normal" });
    await hgbRow.getByRole("button", { name: /^kaydet$/i }).click();
    await expect(hgbRow.getByText(/13\.5/)).toBeVisible({ timeout: 5_000 });

    // Enter GLU result with a flag.
    const gluRow = page.locator("div").filter({ hasText: /^GLU.*Glukoz/ }).first();
    await gluRow.getByRole("button", { name: /sonuç gir/i }).click();
    await gluRow.getByLabel("Değer (sayısal)").fill("145");
    await gluRow.getByLabel("Bayrak").selectOption({ label: "Yüksek" });
    await gluRow.getByRole("button", { name: /^kaydet$/i }).click();
    await expect(gluRow.getByText(/145/)).toBeVisible({ timeout: 5_000 });
    await expect(gluRow.getByText(/yüksek/i)).toBeVisible();

    // Sorumlu onayla.
    await page.getByRole("button", { name: /sonuçları onayla/i }).click();

    // --- Doctor's Lab tab now shows the result is in. ---
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/poliklinik/${visit.id}`);
    await page.getByRole("button", { name: /lab istek/i }).click();
    // Order summary still shows; status word will be "verified" or "resulted"
    // depending on when we look — both states are fine here.
    await expect(page.getByText(/(verified|resulted)/i)).toBeVisible({ timeout: 5_000 });
  });
});
