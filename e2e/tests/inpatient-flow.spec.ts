// EPIC-5 slice 1 — inpatient workflow.
//
//   1. Set up a ward + 2 free beds via the API.
//   2. UI: open the ward management page → see the colour-coded grid.
//   3. UI: open the yatış page → admit a patient to bed 101.
//   4. UI: transfer the same admission to bed 102; bed 101 flips to
//      cleaning, 102 becomes occupied, audit row appears.
//   5. UI: discharge the patient; bed 102 flips to cleaning.

import { expect, test } from "@playwright/test";
import {
  createPatient,
  createTestApi,
  gotoDashboard,
  nextValidTC,
  type TestApi,
} from "../helpers";

test.describe("yatış (inpatient) flow", () => {
  let api: TestApi;
  let wardId: string;
  let bed101Id: string;
  let bed102Id: string;
  let patientId: string;

  test.beforeEach(async () => {
    api = await createTestApi();

    const wardResp = await api.request.post("/api/wards", {
      data: { code: "DAH3", name: "Dahiliye Servisi 3", kind: "general", floor: "3" },
    });
    expect(wardResp.ok()).toBeTruthy();
    wardId = (await wardResp.json()).id;

    const bed101 = await api.request.post(`/api/wards/${wardId}/beds`, {
      data: { code: "101", kind: "standard" },
    });
    expect(bed101.ok()).toBeTruthy();
    bed101Id = (await bed101.json()).id;

    const bed102 = await api.request.post(`/api/wards/${wardId}/beds`, {
      data: { code: "102", kind: "standard" },
    });
    expect(bed102.ok()).toBeTruthy();
    bed102Id = (await bed102.json()).id;

    const patient = await createPatient(api, {
      first_name: "Yatış",
      last_name: "Hastası",
      identifier_kind: "tc",
      identifier_value: nextValidTC(),
    });
    patientId = patient.id;
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("admit → transfer → discharge end-to-end", async ({ page }) => {
    // Sanity check: ward management page shows both beds.
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/yatis/oda-yonetimi`);
    await expect(page.getByText("Dahiliye Servisi 3")).toBeVisible();
    await expect(page.getByText("101")).toBeVisible();
    await expect(page.getByText("102")).toBeVisible();

    // --- Admit via UI ---
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/yatis`);
    await page.getByRole("button", { name: /yeni yatış/i }).click();

    // Hasta typeahead inside the drawer
    await page.getByPlaceholder(/hasta ara/i).fill("Yatış");
    await page.getByText(/Yatış Hastası/).first().click();

    // Resolve <option> values by visible text (selectOption needs strings).
    const wardSelect = page.getByLabel("Servis");
    const wardValue = await wardSelect.locator("option", { hasText: "Dahiliye Servisi 3" }).first().getAttribute("value");
    await wardSelect.selectOption(wardValue!);
    const bedSelect = page.getByLabel("Yatak");
    const bedValue = await bedSelect.locator("option", { hasText: /^101/ }).first().getAttribute("value");
    await bedSelect.selectOption(bedValue!);
    await page.getByLabel("Yatış türü").selectOption({ label: "Planlı" });
    await page.getByLabel(/yatış tanısı/i).fill("Pnömoni");
    await page.getByRole("button", { name: /yatışı oluştur/i }).click();

    // Admission appears in the active list.
    await expect(page.getByText("Yatış Hastası")).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/Yatak 101/)).toBeVisible();
    await expect(page.getByText("Yatıyor")).toBeVisible();

    // --- Open the admission detail + transfer to 102 ---
    await page.getByRole("button", { name: /^aç$/i }).first().click();
    await page.waitForURL(/\/yatis\/[a-f0-9-]+/, { timeout: 10_000 });

    await page.getByRole("button", { name: /transfer/i }).click();
    const transferTo = page.getByLabel("Hedef yatak");
    const targetVal = await transferTo.locator("option", { hasText: "102" }).first().getAttribute("value");
    await transferTo.selectOption(targetVal!);
    await page.getByLabel("Sebep").fill("Kohort değişikliği");
    await page.getByRole("button", { name: /transfer et/i }).click();

    // Transfer audit row.
    await expect(page.getByText(/Kohort değişikliği/)).toBeVisible({ timeout: 5_000 });

    // Verify via API that bed 101 is now cleaning, 102 is occupied.
    const bedMapResp = await api.request.get("/api/bed-map");
    expect(bedMapResp.ok()).toBeTruthy();
    const bedMap = (await bedMapResp.json()) as Array<{ bed: { id: string; status: string } }>;
    const bed101 = bedMap.find((e) => e.bed.id === bed101Id)!;
    const bed102 = bedMap.find((e) => e.bed.id === bed102Id)!;
    expect(bed101.bed.status).toBe("cleaning");
    expect(bed102.bed.status).toBe("occupied");

    // --- Discharge ---
    await page.getByRole("button", { name: /^taburcu$/i }).click();
    await page.getByLabel("Taburcu türü").selectOption({ label: "Evde takip" });
    await page.getByLabel(/taburcu özeti/i).fill("İyileşti. Çıkış reçetesi düzenlendi.");
    await page.getByRole("button", { name: /taburcu et/i }).click();

    // Patient now in discharged state.
    await expect(page.getByText(/İyileşti\./)).toBeVisible({ timeout: 5_000 });

    // Bed 102 flipped to cleaning.
    const after = await api.request.get("/api/bed-map");
    const afterMap = (await after.json()) as Array<{ bed: { id: string; status: string } }>;
    const bed102After = afterMap.find((e) => e.bed.id === bed102Id)!;
    expect(bed102After.bed.status).toBe("cleaning");
  });

  test("admit refuses an already-occupied bed", async ({ page: _page }) => {
    // Admit the same patient via API.
    const ok = await api.request.post("/api/admissions", {
      data: { patient_id: patientId, ward_id: wardId, bed_id: bed101Id, kind: "planned" },
    });
    expect(ok.ok()).toBeTruthy();

    // Try a second admission for a second patient into the same bed.
    const second = await createPatient(api, {
      first_name: "İkinci",
      last_name: "Hasta",
      identifier_kind: "tc",
      identifier_value: nextValidTC(),
    });

    const conflict = await api.request.post("/api/admissions", {
      data: { patient_id: second.id, ward_id: wardId, bed_id: bed101Id, kind: "planned" },
    });
    expect(conflict.status()).toBe(409);
    const body = await conflict.json();
    expect(body.error.code).toBe("bed_unavailable");
  });
});
