// Seeds the browser session for a given TestApi.
//
// We don't replay the email-code login in every spec — that's tested once
// in auth-and-onboarding.spec.ts. For every other spec, we inject the
// tokens directly into localStorage via addInitScript, then let the
// AuthBootstrap component pick them up on first render.

import type { Page } from "@playwright/test";
import type { TestApi } from "./test-api";

const ACCESS_KEY = "medigt_access_token";
const REFRESH_KEY = "medigt_refresh_token";

/** Prime localStorage so AuthBootstrap finds an active session on next nav. */
export async function loginAs(page: Page, api: TestApi): Promise<void> {
  await page.addInitScript(
    ({ accessKey, refreshKey, access, refresh }) => {
      window.localStorage.setItem(accessKey, access);
      window.localStorage.setItem(refreshKey, refresh);
    },
    {
      accessKey: ACCESS_KEY,
      refreshKey: REFRESH_KEY,
      access: api.accessToken,
      refresh: api.refreshToken,
    },
  );
}

/** Convenience: log in + go to the org/branch dashboard. */
export async function gotoDashboard(page: Page, api: TestApi): Promise<void> {
  await loginAs(page, api);
  await page.goto(`/h/${api.orgSlug}/${api.branchSlug}/baslangic`);
  // RootDispatcher loads available orgs via /me before showing chrome —
  // wait for the sidebar to confirm we're past the bootstrap screen.
  await page.waitForSelector('[data-slot="sidebar-inset"], aside, nav', { timeout: 10_000 });
}
