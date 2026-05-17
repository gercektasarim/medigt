// Appointment scheduling + state machine: scheduled → arrived → in_progress
// → completed (via the inline row actions). Also exercises cancel.

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

test.describe("appointment", () => {
  let api: TestApi;
  let patientId: string;
  let doctorId: string;
  let doctorLabel: string;

  test.beforeEach(async () => {
    api = await createTestApi();

    // One patient + one doctor (with Kardiyoloji from the seed) per test.
    const patient = await createPatient(api, {
      first_name: "Ali",
      last_name: "Veli",
      identifier_kind: "tc",
      identifier_value: nextValidTC(),
      phone: "05550001122",
    });
    patientId = patient.id;

    const specs = await listSpecializations(api);
    const kardiyo = specs.find((s) => s.code === "KARDIYOLOJI")!;
    const doctor = await createDoctor(api, {
      staff: { first_name: "Ayşe", last_name: "Demir", title: "Uzm. Dr." },
      specialization_ids: [kardiyo.id],
      primary_specialization_id: kardiyo.id,
    });
    doctorId = doctor.id;
    doctorLabel = "Uzm. Dr. Ayşe Demir";
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("create an appointment via the UI", async ({ page }) => {
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/randevu`);

    await page.getByRole("button", { name: /yeni randevu/i }).click();

    // Hasta typeahead — type a unique fragment and pick the result.
    const patientSearch = page.getByPlaceholder(/hasta ara/i);
    await patientSearch.fill("Ali");
    await page.getByText(/Ali Veli/).first().click();

    // Time + duration + kind
    await page.getByLabel("Saat").fill("14:30");
    await page.getByLabel(/süre/i).fill("30");
    // selectOption requires a string label; resolve the option dynamically.
    const selectEl = page.getByLabel("Doktor");
    const optionValue = await selectEl.locator("option", { hasText: doctorLabel }).first().getAttribute("value");
    if (!optionValue) throw new Error(`doktor seçeneği bulunamadı: ${doctorLabel}`);
    await selectEl.selectOption(optionValue);
    await page.getByLabel(/şikayet/i).fill("Göğüs ağrısı");

    await page.getByRole("button", { name: /randevuyu oluştur/i }).click();

    // Appointment lands on the day's list at 14:30 with the patient + doctor.
    await expect(page.getByText("14:30")).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText("Ali Veli")).toBeVisible();
    await expect(page.getByText("Planlandı")).toBeVisible();
  });

  test("state machine walks scheduled → arrived → in_progress → completed", async ({ page }) => {
    // Seed via API for a deterministic starting state.
    const now = new Date();
    now.setHours(10, 0, 0, 0);
    const appt = await api.request.post("/api/appointments", {
      data: {
        patient_id: patientId,
        doctor_id: doctorId,
        scheduled_at: now.toISOString(),
        duration_minutes: 20,
        kind: "outpatient",
      },
    });
    expect(appt.ok()).toBeTruthy();

    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/randevu`);

    await expect(page.getByText("Planlandı")).toBeVisible({ timeout: 5_000 });

    // scheduled → arrived
    await page.getByRole("button", { name: "Geldi" }).click();
    await expect(page.getByText("Geldi")).toBeVisible({ timeout: 5_000 });

    // arrived → in_progress
    await page.getByRole("button", { name: /muayeneye al/i }).click();
    await expect(page.getByText("Muayenede")).toBeVisible({ timeout: 5_000 });

    // in_progress → completed
    await page.getByRole("button", { name: "Tamamla" }).click();
    await expect(page.getByText("Tamamlandı")).toBeVisible({ timeout: 5_000 });
  });

  test("cancel scheduled appointment with reason", async ({ page }) => {
    const now = new Date();
    now.setHours(11, 0, 0, 0);
    await api.request.post("/api/appointments", {
      data: {
        patient_id: patientId,
        doctor_id: doctorId,
        scheduled_at: now.toISOString(),
        kind: "outpatient",
        reason: "kontrol",
      },
    });

    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/randevu`);

    await expect(page.getByText("11:00")).toBeVisible({ timeout: 5_000 });

    await page.getByRole("button", { name: "İptal" }).first().click();

    // Cancel drawer
    await page.getByLabel(/iptal sebebi/i).fill("Hasta gelemiyor");
    await page.getByRole("button", { name: "İptal et" }).click();

    await expect(page.getByText("İptal", { exact: true })).toBeVisible({ timeout: 5_000 });
  });
});
