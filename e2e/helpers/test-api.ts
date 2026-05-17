// MediGt e2e API helpers.
//
// Each spec creates a fresh user + organization + branch via the real backend
// API (using the dev master code 888888). The resulting TestApi gives:
//   - tokens (access + refresh) ready for localStorage injection
//   - orgId/orgSlug/branchId/branchSlug for URL navigation
//   - request() — a pre-authenticated, pre-tenant-headered API client
//
// Tests should call createTestApi() in beforeEach, then loginAs(page, api)
// to seed the browser session. cleanup() disposes the HTTP context.

import { request, type APIRequestContext } from "@playwright/test";

const API_BASE = process.env.E2E_API_URL ?? "http://localhost:8088";

export type TestApi = {
  request: APIRequestContext;
  accessToken: string;
  refreshToken: string;
  email: string;
  userId: string;
  userName: string;
  orgId: string;
  orgSlug: string;
  branchId: string;
  branchSlug: string;
  cleanup: () => Promise<void>;
};

let counter = 0;

/** Returns a slug-safe unique string. */
export function uniqueSlug(prefix = "e2e"): string {
  counter++;
  const ts = Date.now().toString(36);
  const rnd = Math.random().toString(36).slice(2, 6);
  return `${prefix}-${ts}-${counter}-${rnd}`;
}

/** Make sure the backend is up before kicking off the suite. */
async function pingBackend(ctx: APIRequestContext): Promise<void> {
  const resp = await ctx.get("/api/health");
  if (!resp.ok()) {
    throw new Error(
      `Backend not reachable at ${API_BASE}/api/health (status ${resp.status()}). Run \`make start\` first.`,
    );
  }
}

export async function createTestApi(): Promise<TestApi> {
  const ctx = await request.newContext({ baseURL: API_BASE });
  await pingBackend(ctx);

  const email = `${uniqueSlug("user")}@medigt.test`;

  // 1) send-code (dev shortcut: 888888 always works)
  const sendResp = await ctx.post("/api/auth/send-code", { data: { email } });
  if (!sendResp.ok()) {
    throw new Error(`send-code failed: ${sendResp.status()} ${await sendResp.text()}`);
  }

  // 2) verify-code — returns access + refresh + user
  const verifyResp = await ctx.post("/api/auth/verify-code", {
    data: { email, code: "888888" },
  });
  if (!verifyResp.ok()) {
    throw new Error(`verify-code failed: ${verifyResp.status()} ${await verifyResp.text()}`);
  }
  const verified = (await verifyResp.json()) as {
    access_token: string;
    refresh_token: string;
    user: { id: string; name: string };
  };

  // 3) create org + initial branch
  const orgSlug = uniqueSlug("hst");
  const branchSlug = "merkez";
  const orgResp = await ctx.post("/api/organizations", {
    data: {
      slug: orgSlug,
      name: `E2E Hastanesi ${orgSlug}`,
      kind: "single_hospital",
      initial_branch: {
        slug: branchSlug,
        name: "Merkez Şube",
        kind: "hospital",
      },
    },
    headers: { Authorization: `Bearer ${verified.access_token}` },
  });
  if (!orgResp.ok()) {
    throw new Error(`create org failed: ${orgResp.status()} ${await orgResp.text()}`);
  }
  const orgData = (await orgResp.json()) as {
    organization: { id: string; slug: string };
    branch: { id: string; slug: string };
  };

  // 4) From here on, requests are auto-authenticated + tenant-scoped.
  const authedCtx = await request.newContext({
    baseURL: API_BASE,
    extraHTTPHeaders: {
      Authorization: `Bearer ${verified.access_token}`,
      "X-Organization-ID": orgData.organization.id,
      "X-Branch-ID": orgData.branch.id,
      "Content-Type": "application/json",
    },
  });

  await ctx.dispose();

  return {
    request: authedCtx,
    accessToken: verified.access_token,
    refreshToken: verified.refresh_token,
    email,
    userId: verified.user.id,
    userName: verified.user.name,
    orgId: orgData.organization.id,
    orgSlug: orgData.organization.slug,
    branchId: orgData.branch.id,
    branchSlug: orgData.branch.slug,
    cleanup: async () => {
      await authedCtx.dispose();
    },
  };
}

// ---------- Domain helpers (compose flows quickly inside specs) ----------

export async function createSpecialization(api: TestApi, code: string, name: string) {
  const resp = await api.request.post("/api/specializations", { data: { code, name } });
  if (!resp.ok()) throw new Error(`createSpecialization: ${resp.status()} ${await resp.text()}`);
  return await resp.json();
}

export async function listSpecializations(api: TestApi) {
  const resp = await api.request.get("/api/specializations");
  if (!resp.ok()) throw new Error(`listSpecializations: ${resp.status()}`);
  return (await resp.json()) as Array<{ id: string; code: string; name: string; is_system: boolean }>;
}

export async function createStaff(
  api: TestApi,
  data: { first_name: string; last_name: string; title?: string; employment_type?: string },
) {
  const resp = await api.request.post("/api/staff", { data });
  if (!resp.ok()) throw new Error(`createStaff: ${resp.status()} ${await resp.text()}`);
  return await resp.json();
}

export async function createDoctor(
  api: TestApi,
  data: {
    staff: { first_name: string; last_name: string; title?: string; employment_type?: string };
    specialization_ids?: string[];
    primary_specialization_id?: string;
    diploma_no?: string;
    is_accepting_patients?: boolean;
  },
) {
  const resp = await api.request.post("/api/doctors", { data });
  if (!resp.ok()) throw new Error(`createDoctor: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as { id: string; staff_member_id: string };
}

export async function createInstitution(
  api: TestApi,
  data: { code: string; name: string; kind: string },
) {
  const resp = await api.request.post("/api/institutions", { data });
  if (!resp.ok()) throw new Error(`createInstitution: ${resp.status()} ${await resp.text()}`);
  return await resp.json();
}

export async function createPatient(
  api: TestApi,
  data: {
    first_name: string;
    last_name: string;
    identifier_kind?: string;
    identifier_value?: string;
    phone?: string;
    birth_date?: string;
    gender?: string;
    blood_type?: string;
  },
) {
  const resp = await api.request.post("/api/patients", { data });
  if (!resp.ok()) throw new Error(`createPatient: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as { id: string; mrn: string };
}

export async function createAppointment(
  api: TestApi,
  data: {
    patient_id: string;
    doctor_id?: string;
    scheduled_at: string;
    duration_minutes?: number;
    kind?: string;
    reason?: string;
  },
) {
  const resp = await api.request.post("/api/appointments", { data });
  if (!resp.ok()) throw new Error(`createAppointment: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as { id: string };
}

/** A list of T.C. kimlik numbers that pass the NVI checksum. Synthetic — they
 *  are not assigned to any real person and are safe to use in tests. */
export const VALID_TEST_TCS = [
  "10000000146",
  "19283746506",
  "13579086420",
  "24681357902",
  "98765432104",
];

let tcCursor = 0;
export function nextValidTC(): string {
  const tc = VALID_TEST_TCS[tcCursor % VALID_TEST_TCS.length]!;
  tcCursor++;
  return tc;
}

// ---------- EPIC-5: ameliyat + diyaliz ----------

export async function createOperatingRoom(
  api: TestApi,
  data: { code: string; name: string; floor?: string },
) {
  const resp = await api.request.post("/api/operating-rooms", { data });
  if (!resp.ok()) throw new Error(`createOperatingRoom: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as { id: string; code: string; name: string };
}

export async function createDialysisMachine(
  api: TestApi,
  data: { code: string; name: string; manufacturer?: string },
) {
  const resp = await api.request.post("/api/dialysis-machines", { data });
  if (!resp.ok()) throw new Error(`createDialysisMachine: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as { id: string; code: string; name: string };
}

// ---------- EPIC-6: ilaç + depo ----------

export async function createMedication(
  api: TestApi,
  data: {
    name: string;
    generic_name?: string;
    atc_code?: string;
    form?: string;
    strength?: string;
    prescription_class?: string;
  },
) {
  const resp = await api.request.post("/api/medications", { data });
  if (!resp.ok()) throw new Error(`createMedication: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as { id: string; name: string };
}

export async function createWarehouse(
  api: TestApi,
  data: { code: string; name: string; kind?: string },
) {
  const resp = await api.request.post("/api/warehouses", { data });
  if (!resp.ok()) throw new Error(`createWarehouse: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as { id: string; code: string; name: string };
}

export async function receiveStock(
  api: TestApi,
  data: {
    warehouse_id: string;
    medication_id: string;
    lot_no: string;
    expiry_date?: string;
    quantity: number;
    unit_price?: number;
    counterparty?: string;
  },
) {
  const resp = await api.request.post("/api/stock-movements/receive", { data });
  if (!resp.ok()) throw new Error(`receiveStock: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as { movement_no: string };
}

// ---------- EPIC-7: vezne + fatura ----------

export async function openCashRegister(
  api: TestApi,
  data: { opening_balance: number; notes?: string },
) {
  const resp = await api.request.post("/api/cash-registers", { data });
  if (!resp.ok()) throw new Error(`openCashRegister: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as { id: string; register_no: string };
}

export async function createInvoice(
  api: TestApi,
  data: {
    patient_id: string;
    institution_id?: string;
    finalize?: boolean;
    items: Array<{
      code: string;
      name: string;
      quantity: number;
      unit_price: number;
      discount_pct?: number;
      vat_rate?: number;
      doctor_id?: string;
      service_id?: string;
    }>;
  },
) {
  const resp = await api.request.post("/api/invoices", { data });
  if (!resp.ok()) throw new Error(`createInvoice: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as { id: string; invoice_no: string };
}

// ---------- Payment + audit ----------

export async function recordPayment(
  api: TestApi,
  data: {
    patient_id: string;
    method: string;
    amount: number;
    cash_register_id?: string;
    allocations: Array<{ invoice_id: string; amount: number }>;
    reference?: string;
    notes?: string;
  },
) {
  const resp = await api.request.post("/api/payments", { data });
  if (!resp.ok()) throw new Error(`recordPayment: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as {
    payment_id: string;
    payment_no: string;
    cash_movement_no?: string;
  };
}

export async function fetchAuditLog(
  api: TestApi,
  params: { action?: string; entity_type?: string; limit?: number } = {},
) {
  const qs = new URLSearchParams();
  if (params.action) qs.set("action", params.action);
  if (params.entity_type) qs.set("entity_type", params.entity_type);
  if (params.limit != null) qs.set("limit", String(params.limit));
  const resp = await api.request.get(
    `/api/audit-log${qs.toString() ? `?${qs}` : ""}`,
  );
  if (!resp.ok()) throw new Error(`fetchAuditLog: ${resp.status()}`);
  return (await resp.json()) as {
    total: number;
    items: Array<{
      id: number;
      action: string;
      entity_type: string;
      entity_id?: string;
      details: Record<string, unknown>;
    }>;
  };
}

// ---------- e-İmza (TURKKEP cloud) ----------

export async function initSignature(
  api: TestApi,
  data: {
    target_table: string;
    target_id: string;
    document_kind: string;
    document_hash?: string;
    document_bytes?: string;
  },
) {
  const resp = await api.request.post("/api/signatures", { data });
  if (!resp.ok()) throw new Error(`initSignature: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as {
    id: string;
    challenge_code: string;
    status: string;
  };
}

/** Drive the mock TURKKEP signing loop to terminal. Mock signs on the
 *  2nd poll (≥2s after init). Resolves with the final status. */
export async function pollUntilTerminal(
  api: TestApi,
  signatureId: string,
  maxAttempts = 30,
) {
  for (let i = 0; i < maxAttempts; i++) {
    const r = await api.request.post(`/api/signatures/${signatureId}/poll`, { data: {} });
    if (!r.ok() && r.status() !== 202) {
      throw new Error(`pollSignature: ${r.status()} ${await r.text()}`);
    }
    const body = (await r.json()) as { status: string };
    if (["signed", "cancelled", "failed", "expired"].includes(body.status)) {
      return body as { status: string };
    }
    await new Promise((res) => setTimeout(res, 1100));
  }
  throw new Error("e-imza polling timed out");
}

// ---------- HL7 ORU^R01 inbound ----------

/** POST a raw HL7 message to the lab inbound endpoint. */
export async function postHL7Result(api: TestApi, hl7Body: string) {
  const resp = await api.request.post("/api/integrations/hl7/lab-result", {
    data: hl7Body,
    headers: { "Content-Type": "text/plain" },
  });
  if (!resp.ok()) throw new Error(`postHL7Result: ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as {
    message_control_id: string;
    order_no: string;
    matched_observations: number;
    unmatched_observations?: Array<{ test_code: string; reason: string }>;
    errors?: string[];
  };
}

// ---------- Reports ----------

export async function runReport(
  api: TestApi,
  id: string,
  params: Record<string, string> = {},
) {
  const qs = new URLSearchParams(params).toString();
  const resp = await api.request.get(`/api/reports/${id}${qs ? `?${qs}` : ""}`);
  if (!resp.ok()) throw new Error(`runReport(${id}): ${resp.status()} ${await resp.text()}`);
  return (await resp.json()) as {
    columns: Array<{ key: string; label: string; type: string }>;
    rows: Record<string, unknown>[];
    summary?: Record<string, unknown>;
  };
}

// ---------- EPIC-3 reusable: start a visit ----------

export async function startVisitFor(
  api: TestApi,
  patientId: string,
  doctorId: string,
  options: { kind?: string; whenISO?: string } = {},
): Promise<{ visitId: string; appointmentId: string }> {
  const apptResp = await api.request.post("/api/appointments", {
    data: {
      patient_id: patientId,
      doctor_id: doctorId,
      scheduled_at: options.whenISO ?? new Date().toISOString(),
      duration_minutes: 20,
      kind: options.kind ?? "outpatient",
    },
  });
  if (!apptResp.ok()) throw new Error(`appt: ${apptResp.status()} ${await apptResp.text()}`);
  const appt = (await apptResp.json()) as { id: string };
  await api.request.post(`/api/appointments/${appt.id}/status`, { data: { status: "arrived" } });
  const vResp = await api.request.post("/api/visits/start-from-appointment", {
    data: { appointment_id: appt.id },
  });
  if (!vResp.ok()) throw new Error(`visit: ${vResp.status()} ${await vResp.text()}`);
  const v = (await vResp.json()) as { id: string };
  return { visitId: v.id, appointmentId: appt.id };
}
