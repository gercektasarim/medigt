// Branş + Personel + Doktor + Kurum modules.
//
// Each test creates a fresh org via the API helper, then drives the UI to
// confirm the CRUD round-trip lands rows on the page.

import { expect, test } from "@playwright/test";
import {
  createSpecialization,
  createTestApi,
  gotoDashboard,
  type TestApi,
} from "../helpers";

test.describe("master data — people & institutions", () => {
  let api: TestApi;

  test.beforeEach(async () => {
    api = await createTestApi();
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("specialization list shows the 30 system specialties", async ({ page }) => {
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/brans`);

    // The 30 system specializations seeded in migration 002 should all be here.
    await expect(page.getByText("Kardiyoloji")).toBeVisible();
    await expect(page.getByText("Genel Cerrahi")).toBeVisible();
    await expect(page.getByText("İç Hastalıkları")).toBeVisible();

    // At least 30 rows.
    const rowCount = await page.locator("tbody tr").count();
    expect(rowCount).toBeGreaterThanOrEqual(30);
  });

  test("create a new specialization", async ({ page }) => {
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/brans`);
    await page.getByRole("button", { name: /yeni branş/i }).click();

    await page.getByLabel("Branş adı").fill("E2E Test Branşı");
    await page.getByLabel("Kod", { exact: true }).fill("E2E_BRANS");
    await page.getByRole("button", { name: "Kaydet" }).click();

    await expect(page.getByText("E2E Test Branşı")).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText("E2E_BRANS")).toBeVisible();
  });

  test("create a new staff member", async ({ page }) => {
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/personel`);

    await page.getByRole("button", { name: /yeni personel/i }).click();
    await page.getByLabel("Ad").fill("Mehmet");
    await page.getByLabel("Soyad").fill("Yılmaz");
    await page.getByLabel("Unvan").fill("Sekreter");
    await page.getByLabel("Sicil no").fill("P-E2E-001");
    await page.getByRole("button", { name: "Kaydet" }).click();

    await expect(page.getByText("Mehmet")).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText("Yılmaz")).toBeVisible();
    await expect(page.getByText("P-E2E-001")).toBeVisible();
  });

  test("create a doctor with an inline staff member + specialization", async ({ page }) => {
    // Use the existing Kardiyoloji from the seed so the form has something to pick.
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/doktor`);

    await page.getByRole("button", { name: /yeni doktor/i }).click();
    await page.getByLabel("Ad").fill("Ayşe");
    await page.getByLabel("Soyad").fill("Demir");
    await page.getByLabel("Unvan").fill("Uzm. Dr.");
    // Primary branş select — picks Kardiyoloji by visible label.
    await page.getByLabel("Birincil branş").selectOption({ label: "Kardiyoloji" });
    await page.getByLabel("Diploma no").fill("DIP-E2E");
    await page.getByRole("button", { name: "Kaydet" }).click();

    await expect(page.getByText("Uzm. Dr. Ayşe Demir")).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText("Kardiyoloji").nth(0)).toBeVisible();
  });

  test("create an SGK institution", async ({ page }) => {
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/kurum`);

    await page.getByRole("button", { name: /yeni kurum/i }).click();
    await page.getByLabel("Kurum adı").fill("Sosyal Güvenlik Kurumu");
    await page.getByLabel("Kod", { exact: true }).fill("SGK");
    await page.getByLabel("Tür").selectOption({ label: "SGK" });
    await page.getByRole("button", { name: "Kaydet" }).click();

    await expect(page.getByText("Sosyal Güvenlik Kurumu")).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText("SGK").nth(0)).toBeVisible();
  });

  test("API-seeded specialization is visible in the UI after navigation", async ({ page }) => {
    // Sanity check: things created over the API show up in the UI.
    await createSpecialization(api, "API_SEEDED", "API ile Eklendi");
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/brans`);
    await expect(page.getByText("API ile Eklendi")).toBeVisible();
  });
});
