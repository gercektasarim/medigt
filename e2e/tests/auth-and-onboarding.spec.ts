// Real, full-fat onboarding flow:
//   /login → email → master code 888888 → onboarding wizard → first hospital
//   created → user lands on /h/:org/:branch/baslangic.
//
// Every other spec injects tokens directly to skip this happy path; this one
// proves the path itself works.

import { expect, test } from "@playwright/test";
import { uniqueSlug } from "../helpers";

test.describe("auth + onboarding", () => {
  test("first-time user signs in and creates their first hospital", async ({ page }) => {
    const email = `${uniqueSlug("user")}@medigt.test`;
    const orgSlug = uniqueSlug("hst");
    const branchSlug = "merkez";

    // --- /login → email step ---
    await page.goto("/login");
    await expect(page.getByText("MediGt", { exact: false })).toBeVisible();
    await page.getByLabel("E-posta").fill(email);
    await page.getByRole("button", { name: /kod gönder/i }).click();

    // --- code step → master code 888888 → auto-submit ---
    const codeInput = page.locator('input[autocomplete="one-time-code"]');
    await expect(codeInput).toBeVisible();
    await codeInput.fill("888888");

    // The form auto-submits when 6 digits are typed. Wait for navigation.
    await page.waitForURL((url) => !url.pathname.startsWith("/login"), { timeout: 15_000 });

    // --- onboarding wizard ---
    await expect(page.getByText(/hoş geldiniz/i)).toBeVisible({ timeout: 10_000 });
    await page.getByRole("button", { name: /başla/i }).click();

    // Org step
    await page.getByLabel("Hastane adı").fill("E2E Test Hastanesi");
    await page.getByLabel(/URL kısa adı/i).fill(orgSlug);
    await page.getByRole("button", { name: "Devam" }).click();

    // Branch step — defaults are fine, just update the slug to be deterministic.
    await page.getByLabel("Şube slug").fill(branchSlug);
    await page.getByRole("button", { name: /oluştur/i }).click();

    // --- landing on /h/:org/:branch/baslangic ---
    await page.waitForURL(new RegExp(`/h/${orgSlug}/${branchSlug}/baslangic`), { timeout: 15_000 });
    await expect(page).toHaveURL(new RegExp(`/h/${orgSlug}/${branchSlug}/baslangic`));
  });

  test("invalid code is rejected without crashing the form", async ({ page }) => {
    const email = `${uniqueSlug("user")}@medigt.test`;
    await page.goto("/login");
    await page.getByLabel("E-posta").fill(email);
    await page.getByRole("button", { name: /kod gönder/i }).click();

    const codeInput = page.locator('input[autocomplete="one-time-code"]');
    await codeInput.fill("000000");
    // Auto-submit fires; backend returns invalid_code. Form should stay on
    // the code step and surface an error.
    await expect(page.getByText(/hatalı|süresi geçti/i)).toBeVisible({ timeout: 10_000 });
    // We are still on /login (no redirect).
    expect(page.url()).toContain("/login");
  });
});
