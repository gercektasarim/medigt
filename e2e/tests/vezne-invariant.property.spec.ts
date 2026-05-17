// Property-based test for the vezne (cashier) money invariant.
//
// The non-negotiable accounting truth per patient:
//
//   billed_active  ==  paid_active + outstanding_active
//
// where _active scopes "finalized" and "paid" rows (cancelled invoices
// reverse out — they're tracked separately). Plus:
//
//   billed_total  ==  paid_active + outstanding_active + cancelled_total
//
// where billed_total includes ALL non-draft invoices (including cancelled).
//
// We drive a random sequence of operations (createInvoice, recordPayment
// partial / full, cancelInvoice) and assert the invariant after each step.
// Fast-check shrinks failing sequences to the smallest counter-example.

import { expect, test } from "@playwright/test";
import fc from "fast-check";
import {
  createInvoice,
  createPatient,
  createTestApi,
  nextValidTC,
  openCashRegister,
  recordPayment,
  type TestApi,
} from "../helpers";

type InvoiceState = {
  id: string;
  total: number;       // post-VAT
  paidTotal: number;   // running paid_total
  status: "finalized" | "paid" | "cancelled";
};

type WorldState = {
  patientId: string;
  cashRegisterId: string;
  invoices: Map<string, InvoiceState>;
};

// Per-line VAT rate is 10%. We pick whole-TL prices so the math stays
// exact (NUMERIC(14,2)) and assertions don't fight rounding noise.
const VAT = 10;

function postVat(unitPrice: number, qty: number): number {
  const sub = unitPrice * qty;
  const tax = Math.round(sub * VAT) / 100;
  return Math.round((sub + tax) * 100) / 100;
}

/** Drive one command against the backend, update the shadow state. */
async function step(
  api: TestApi,
  world: WorldState,
  cmd: Command,
): Promise<void> {
  if (cmd.kind === "create") {
    const total = postVat(cmd.unitPrice, cmd.qty);
    const inv = await createInvoice(api, {
      patient_id: world.patientId,
      finalize: true,
      items: [
        {
          code: "SVC",
          name: `Hizmet x${cmd.qty}`,
          quantity: cmd.qty,
          unit_price: cmd.unitPrice,
          vat_rate: VAT,
        },
      ],
    });
    world.invoices.set(inv.id, {
      id: inv.id,
      total,
      paidTotal: 0,
      status: "finalized",
    });
    return;
  }

  if (cmd.kind === "pay") {
    // Pick an invoice that still has balance > 0 and isn't cancelled.
    const candidates = [...world.invoices.values()].filter(
      (i) => i.status !== "cancelled" && i.paidTotal < i.total,
    );
    if (candidates.length === 0) return;
    const target = candidates[cmd.invoiceIdx % candidates.length]!;
    const remaining = round2(target.total - target.paidTotal);
    // Pay 25% / 50% / 100% of remaining, but at least 0.01.
    const ratio = cmd.fraction;
    const amount = Math.max(0.01, round2(remaining * ratio));
    const capped = Math.min(amount, remaining);

    await recordPayment(api, {
      patient_id: world.patientId,
      method: "cash",
      amount: capped,
      cash_register_id: world.cashRegisterId,
      allocations: [{ invoice_id: target.id, amount: capped }],
    });
    target.paidTotal = round2(target.paidTotal + capped);
    if (target.paidTotal >= target.total - 0.001) {
      target.status = "paid";
    }
    return;
  }

  if (cmd.kind === "cancel") {
    // Pick a non-paid, non-cancelled invoice with zero payments — backend
    // refuses to cancel anything that already has receipts on it (real-world
    // accounting rule). If none qualifies, no-op.
    const candidates = [...world.invoices.values()].filter(
      (i) => i.status === "finalized" && i.paidTotal === 0,
    );
    if (candidates.length === 0) return;
    const target = candidates[cmd.invoiceIdx % candidates.length]!;
    const resp = await api.request.post(`/api/invoices/${target.id}/cancel`, {
      data: { reason: "property test" },
    });
    if (resp.ok()) {
      target.status = "cancelled";
    }
    return;
  }
}

function round2(n: number): number {
  return Math.round(n * 100) / 100;
}

type Command =
  | { kind: "create"; unitPrice: number; qty: number }
  | { kind: "pay"; invoiceIdx: number; fraction: number }
  | { kind: "cancel"; invoiceIdx: number };

const commandArb: fc.Arbitrary<Command> = fc.oneof(
  fc.record({
    kind: fc.constant("create" as const),
    unitPrice: fc.integer({ min: 10, max: 5000 }),
    qty: fc.integer({ min: 1, max: 5 }),
  }),
  fc.record({
    kind: fc.constant("pay" as const),
    invoiceIdx: fc.integer({ min: 0, max: 9 }),
    fraction: fc.constantFrom(0.25, 0.5, 0.75, 1.0),
  }),
  fc.record({
    kind: fc.constant("cancel" as const),
    invoiceIdx: fc.integer({ min: 0, max: 9 }),
  }),
);

/** Pull the live aggregates straight from the backend and compare to
 *  the shadow state. Source of truth is the backend; we just check that
 *  per-invoice numbers match what we shadow-tracked. */
async function verifyInvariant(
  api: TestApi,
  world: WorldState,
): Promise<{ billed: number; paid: number; outstanding: number; cancelled: number }> {
  let billedActive = 0;
  let paidActive = 0;
  let outstanding = 0;
  let cancelledTotal = 0;

  for (const local of world.invoices.values()) {
    const resp = await api.request.get(`/api/invoices/${local.id}`);
    expect(resp.ok(), `fetch ${local.id}`).toBeTruthy();
    const body = (await resp.json()) as {
      invoice: {
        total: number;
        paid_total: number;
        balance_due: number;
        status: string;
      };
    };
    const remote = body.invoice;

    // Per-invoice round-trip: shadow state must match backend state.
    expect(remote.total, `total drift for ${local.id}`).toBeCloseTo(local.total, 2);
    expect(remote.status, `status drift for ${local.id}`).toBe(local.status);
    expect(
      remote.paid_total,
      `paid_total drift for ${local.id}`,
    ).toBeCloseTo(local.paidTotal, 2);

    // The core single-invoice invariant — applies always:
    //   total == paid_total + balance_due
    expect(
      round2(remote.paid_total + remote.balance_due),
      `single-invoice invariant for ${local.id}`,
    ).toBeCloseTo(remote.total, 2);

    if (remote.status === "cancelled") {
      cancelledTotal += remote.total;
    } else {
      billedActive += remote.total;
      paidActive += remote.paid_total;
      outstanding += remote.balance_due;
    }
  }

  // Aggregate invariant — must hold across all of this patient's invoices:
  expect(
    round2(paidActive + outstanding),
    "active invariant: billed = paid + outstanding",
  ).toBeCloseTo(round2(billedActive), 2);

  return {
    billed: round2(billedActive),
    paid: round2(paidActive),
    outstanding: round2(outstanding),
    cancelled: round2(cancelledTotal),
  };
}

test.describe("vezne invariant — property based", () => {
  let api: TestApi;
  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  // One full run = one new patient + one kasa + N random commands. We
  // run a handful of fast-check iterations; each one re-uses the same
  // patient inside the property body.
  test("random sequence preserves the money invariant", async () => {
    test.setTimeout(120_000); // hits the backend repeatedly

    // 5 iterations × ~12 ops each ≈ 60 backend calls. Tuned so the suite
    // stays under the per-test timeout while still being a real fuzz pass.
    await fc.assert(
      fc.asyncProperty(
        fc.array(commandArb, { minLength: 4, maxLength: 12 }),
        async (commands) => {
          const patient = await createPatient(api, {
            first_name: "Prop",
            last_name: "Tester",
            identifier_kind: "tc",
            identifier_value: nextValidTC(),
          });
          const reg = await openCashRegister(api, { opening_balance: 0 });
          const world: WorldState = {
            patientId: patient.id,
            cashRegisterId: reg.id,
            invoices: new Map(),
          };

          for (const cmd of commands) {
            await step(api, world, cmd);
            // Spot-check the invariant after every command so any bad
            // step pinpoints the exact culprit (rather than only failing
            // on the final state).
            await verifyInvariant(api, world);
          }

          // Final pass — also report the headline numbers for context
          // when fast-check shrinks a failing run.
          const totals = await verifyInvariant(api, world);
          expect(totals.billed).toBeGreaterThanOrEqual(0);
          expect(totals.paid).toBeLessThanOrEqual(totals.billed + 0.01);

          // Close kasa for the next iteration so a fresh one can open.
          await api.request.post(`/api/cash-registers/${reg.id}/close`, {
            data: { declared_balance: totals.paid },
          });
        },
      ),
      { numRuns: 3, verbose: false },
    );
  });
});
