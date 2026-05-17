// Patient creation, duplicate-identifier guard, TC checksum, search.

import { expect, test } from "@playwright/test";
import { createTestApi, gotoDashboard, nextValidTC, VALID_TEST_TCS, type TestApi } from "../helpers";

test.describe("patient", () => {
  let api: TestApi;

  test.beforeEach(async () => {
    api = await createTestApi();
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("create a patient with a valid TC", async ({ page }) => {
    const tc = nextValidTC();
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/hasta`);

    await page.getByRole("button", { name: /yeni hasta/i }).click();

    // Identity (TC default)
    await page.getByLabel(/T\.?C\.? Kimlik No/i).fill(tc);
    // Demographics
    await page.getByLabel("Ad", { exact: true }).fill("Ahmet");
    await page.getByLabel("Soyad").fill("Yılmaz");
    await page.getByLabel("Cinsiyet").selectOption({ label: "Erkek" });
    await page.getByLabel("Kan grubu").selectOption({ label: "A Rh+" });
    // Contact
    await page.getByLabel("Telefon").fill("0555 111 22 33");

    await page.getByRole("button", { name: /hastayı kaydet/i }).click();

    // Lands back on the list — search for the new row.
    await expect(page.getByText("Ahmet")).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText("Yılmaz")).toBeVisible();
    // Masked identifier (last 4 visible)
    const masked = "*******" + tc.slice(-4);
    await expect(page.getByText(masked)).toBeVisible();
    // MRN format: 8-digit zero-pad
    await expect(page.locator("tbody tr").first().getByText(/^\d{8}$/)).toBeVisible();
  });

  test("invalid TC fails the local checksum and disables submit", async ({ page }) => {
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/hasta`);

    await page.getByRole("button", { name: /yeni hasta/i }).click();
    await page.getByLabel(/T\.?C\.? Kimlik No/i).fill("12345678901"); // length OK, checksum bad
    await page.getByLabel("Ad", { exact: true }).fill("Hatalı");
    await page.getByLabel("Soyad").fill("TC");

    await expect(page.getByText(/algoritmaya göre geçersiz/i)).toBeVisible();
    await expect(page.getByRole("button", { name: /hastayı kaydet/i })).toBeDisabled();
  });

  test("duplicate TC is rejected with a friendly error", async ({ page }) => {
    const tc = VALID_TEST_TCS[0]!;

    // Seed one patient via API.
    const seedResp = await api.request.post("/api/patients", {
      data: {
        first_name: "İlk",
        last_name: "Kayıt",
        identifier_kind: "tc",
        identifier_value: tc,
      },
    });
    expect(seedResp.ok()).toBeTruthy();

    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/hasta`);

    await page.getByRole("button", { name: /yeni hasta/i }).click();
    await page.getByLabel(/T\.?C\.? Kimlik No/i).fill(tc);
    await page.getByLabel("Ad", { exact: true }).fill("İkinci");
    await page.getByLabel("Soyad").fill("Kayıt");
    await page.getByRole("button", { name: /hastayı kaydet/i }).click();

    await expect(page.getByText(/zaten kayıtlı/i)).toBeVisible({ timeout: 5_000 });
  });

  test("debounced search filters the list", async ({ page }) => {
    // Seed two patients with distinct names.
    await api.request.post("/api/patients", {
      data: { first_name: "Zeynep", last_name: "Aydın", phone: "05551111111" },
    });
    await api.request.post("/api/patients", {
      data: { first_name: "Kemal", last_name: "Öztürk", phone: "05552222222" },
    });

    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/hasta`);

    // Both visible initially.
    await expect(page.getByText("Zeynep")).toBeVisible();
    await expect(page.getByText("Kemal")).toBeVisible();

    // Search "Zeyn" — only Zeynep visible.
    await page.getByPlaceholder(/ara:/i).fill("Zeyn");
    await expect(page.getByText("Zeynep")).toBeVisible({ timeout: 3_000 });
    await expect(page.getByText("Kemal")).not.toBeVisible();
  });
});
