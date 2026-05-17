// Service catalog (+ per-institution prices) and ICD-10 search.

import { expect, test } from "@playwright/test";
import {
  createInstitution,
  createTestApi,
  gotoDashboard,
  type TestApi,
} from "../helpers";

test.describe("master data — services & ICD-10", () => {
  let api: TestApi;

  test.beforeEach(async () => {
    api = await createTestApi();
  });

  test.afterEach(async () => {
    await api.cleanup();
  });

  test("create a service then add a per-institution price", async ({ page }) => {
    // SGK needs to exist for the price drawer dropdown.
    await createInstitution(api, { code: "SGK", name: "Sosyal Güvenlik Kurumu", kind: "sgk" });

    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/hizmet`);

    // --- Create service ---
    await page.getByRole("button", { name: /yeni hizmet/i }).click();
    await page.getByLabel("Hizmet adı").fill("Genel Muayene");
    await page.getByLabel("Kod", { exact: true }).fill("GEN_MUAYENE");
    await page.getByLabel("Kategori").selectOption({ label: "Muayene" });
    await page.getByLabel("Etiket fiyat").fill("500");
    await page.getByRole("button", { name: "Kaydet" }).click();

    await expect(page.getByText("Genel Muayene")).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/₺\s?500/)).toBeVisible();

    // --- Open the row → price drawer ---
    await page.getByText("Genel Muayene").click();
    await expect(page.getByText(/fiyatlar:\s*Genel Muayene/i)).toBeVisible();

    // Add SGK-specific price
    await page.getByLabel("Kurum").selectOption({ label: "Sosyal Güvenlik Kurumu" });
    await page.getByLabel(/fiyat \(try\)/i).fill("350");
    await page.getByRole("button", { name: /fiyat ekle/i }).click();

    await expect(page.getByText("Sosyal Güvenlik Kurumu")).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText(/₺\s?350/)).toBeVisible();
  });

  test("ICD-10 search surfaces seeded codes", async ({ page }) => {
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/icd10`);

    // The page loads with the first batch already visible.
    await expect(page.locator("tbody tr").first()).toBeVisible({ timeout: 5_000 });

    // Search by code prefix.
    const search = page.getByPlaceholder(/ara:/i);
    await search.fill("I10");
    await expect(page.getByText(/hipertansiyon/i)).toBeVisible({ timeout: 5_000 });

    // Search by Turkish title.
    await search.fill("");
    await search.fill("diabetes");
    // E11 (Tip 2 DM) should land somewhere in the result list.
    await expect(page.getByText("E11", { exact: false }).first()).toBeVisible({ timeout: 5_000 });
  });

  test("rejects a duplicate service code with a friendly message", async ({ page }) => {
    await gotoDashboard(page, api);
    await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/hizmet`);

    // Create one.
    await page.getByRole("button", { name: /yeni hizmet/i }).click();
    await page.getByLabel("Hizmet adı").fill("Konsültasyon");
    await page.getByLabel("Kod", { exact: true }).fill("KONS");
    await page.getByLabel("Kategori").selectOption({ label: "Muayene" });
    await page.getByRole("button", { name: "Kaydet" }).click();
    await expect(page.getByText("Konsültasyon")).toBeVisible();

    // Try to create another with the same code.
    await page.getByRole("button", { name: /yeni hizmet/i }).click();
    await page.getByLabel("Hizmet adı").fill("Konsültasyon İkincil");
    await page.getByLabel("Kod", { exact: true }).fill("KONS");
    await page.getByLabel("Kategori").selectOption({ label: "Muayene" });
    await page.getByRole("button", { name: "Kaydet" }).click();

    await expect(page.getByText(/zaten kullan/i)).toBeVisible({ timeout: 5_000 });
  });
});
