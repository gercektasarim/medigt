// e-İmza (TURKKEP cloud) — Init → Poll loop → terminal.
//
// Mock client deterministik:
//   - signer TC sonu '0' → failed
//   - challenge_code SHA-256(doc_hash) ilk 6 hex'inden türetilir (sabit)
//   - Init sonrası 2. poll'da (≥2s) signed
//
// E2E senaryolar:
//   1. Init → poll → signed; envelope alındı, signer TC saklandı.
//   2. Init → cancel → status='cancelled'.
//   3. Re-poll terminal row idempotent (no-op).

import { expect, test } from "@playwright/test";
import {
  createPatient,
  createTestApi,
  initSignature,
  nextValidTC,
  pollUntilTerminal,
  type TestApi,
} from "../helpers";

test.describe("e-imza — TURKKEP mock", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("init → poll → signed (happy path)", async () => {
    // Use a prescription target — we just need ANY valid uuid for target_id;
    // the signature itself doesn't enforce existence in the target table.
    // For a realistic target we link to a patient row.
    const patient = await createPatient(api, {
      first_name: "Test", last_name: "Sign",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });

    const sig = await initSignature(api, {
      target_table: "prescription",
      target_id: patient.id, // valid uuid; not a real prescription row but fine for sig itself
      document_kind: "prescription",
      document_hash: "abc123def456abc123def456abc123def456abc123def456abc123def456abcd",
    });
    expect(sig.status).toBe("pending");
    expect(sig.challenge_code).toMatch(/^\d{6}$/);

    const final = await pollUntilTerminal(api, sig.id);
    expect(final.status).toBe("signed");

    // Fetch detail and verify cert subject populated.
    const detail = await api.request.get(`/api/signatures/${sig.id}`);
    const body = await detail.json();
    expect(body.signed_at).toBeTruthy();
    expect(body.certificate_subject).toMatch(/CN=/);
  });

  test("cancel mid-flow → status=cancelled", async () => {
    const patient = await createPatient(api, {
      first_name: "Cancel", last_name: "Test",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const sig = await initSignature(api, {
      target_table: "prescription",
      target_id: patient.id,
      document_kind: "prescription",
      document_hash: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
    });
    expect(sig.status).toBe("pending");

    const cancel = await api.request.post(`/api/signatures/${sig.id}/cancel`, { data: {} });
    expect(cancel.ok()).toBeTruthy();

    const detail = await api.request.get(`/api/signatures/${sig.id}`);
    const body = await detail.json();
    expect(body.status).toBe("cancelled");

    // Polling a terminal row is idempotent — no state change.
    const r = await api.request.post(`/api/signatures/${sig.id}/poll`, { data: {} });
    expect(r.ok()).toBeTruthy();
    const after = await r.json();
    expect(after.status).toBe("cancelled");
  });

  test("active sessions endpoint returns user's open ones", async () => {
    const patient = await createPatient(api, {
      first_name: "Active", last_name: "List",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const sig = await initSignature(api, {
      target_table: "prescription",
      target_id: patient.id,
      document_kind: "prescription",
      document_hash: "1111222233334444555566667777888899990000111122223333444455556666",
    });

    const mineResp = await api.request.get("/api/signatures/mine");
    expect(mineResp.ok()).toBeTruthy();
    const mine = (await mineResp.json()) as Array<{ id: string; status: string }>;
    expect(mine.find((m) => m.id === sig.id)).toBeTruthy();
  });
});
