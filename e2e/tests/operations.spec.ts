// EPIC-5 slices 3 & 4 — surgery + dialysis state machines.

import { expect, test } from "@playwright/test";
import {
  createDialysisMachine,
  createDoctor,
  createOperatingRoom,
  createPatient,
  createTestApi,
  listSpecializations,
  nextValidTC,
  type TestApi,
} from "../helpers";

test.describe("ameliyat — state machine", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("schedule → start → save op-note → complete", async () => {
    const or = await createOperatingRoom(api, { code: "AM1", name: "Ameliyathane 1", floor: "2" });
    const patient = await createPatient(api, {
      first_name: "Op", last_name: "Hastası",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const specs = await listSpecializations(api);
    const gen = specs.find((s) => s.code === "GENEL_CERRAHI") ?? specs[0]!;
    const doctor = await createDoctor(api, {
      staff: { first_name: "Op", last_name: "Cerrah", title: "Op. Dr." },
      specialization_ids: [gen.id],
      primary_specialization_id: gen.id,
    });

    const scheduled = new Date(Date.now() + 2 * 60 * 60 * 1000).toISOString();
    const createResp = await api.request.post("/api/surgeries", {
      data: {
        patient_id: patient.id,
        operating_room_id: or.id,
        primary_surgeon_id: doctor.id,
        procedure_name: "Laparoskopik kolesistektomi",
        priority: "elective",
        anesthesia_type: "general",
        scheduled_at: scheduled,
        estimated_minutes: 90,
      },
    });
    expect(createResp.status()).toBe(201);
    const surgery = (await createResp.json()) as { id: string; status: string; surgery_no: string };
    expect(surgery.status).toBe("scheduled");
    expect(surgery.surgery_no).toMatch(/^\d{8}$/);

    // List endpoint surfaces it.
    const list = await api.request.get(
      `/api/surgeries?from=${scheduled.slice(0, 10)}&to=${scheduled.slice(0, 10)}`,
    );
    expect(list.ok()).toBeTruthy();
    const rows = (await list.json()) as Array<{ id: string }>;
    expect(rows.find((r) => r.id === surgery.id)).toBeTruthy();

    // Start.
    const start = await api.request.post(`/api/surgeries/${surgery.id}/status`, {
      data: { status: "in_progress" },
    });
    expect(start.ok()).toBeTruthy();
    expect((await start.json()).status).toBe("in_progress");

    // Save op-note + blood loss + specimen flag.
    const opNote = await api.request.patch(`/api/surgeries/${surgery.id}/op-note`, {
      data: {
        op_note: "Standart laparoskopi. Klipsleme tamam.",
        blood_loss_ml: 50,
        specimen_sent: true,
      },
    });
    expect(opNote.ok()).toBeTruthy();
    const noted = await opNote.json();
    expect(noted.blood_loss_ml).toBe(50);
    expect(noted.specimen_sent).toBe(true);

    // Complete.
    const done = await api.request.post(`/api/surgeries/${surgery.id}/status`, {
      data: { status: "completed" },
    });
    expect(done.ok()).toBeTruthy();
    const finalState = await done.json();
    expect(finalState.status).toBe("completed");
    expect(finalState.started_at).toBeTruthy();
    expect(finalState.ended_at).toBeTruthy();
  });

  test("invalid status transition is rejected", async () => {
    const or = await createOperatingRoom(api, { code: "AM2", name: "Ameliyathane 2" });
    const patient = await createPatient(api, {
      first_name: "Bad", last_name: "Trans",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });
    const r = await api.request.post("/api/surgeries", {
      data: {
        patient_id: patient.id,
        operating_room_id: or.id,
        procedure_name: "x",
        scheduled_at: new Date().toISOString(),
      },
    });
    const surgery = await r.json();

    // 'scheduled' is not a valid target via this endpoint (only in_progress,
    // completed, cancelled).
    const bad = await api.request.post(`/api/surgeries/${surgery.id}/status`, {
      data: { status: "scheduled" },
    });
    expect(bad.status()).toBe(400);
  });
});

test.describe("diyaliz — state machine", () => {
  let api: TestApi;

  test.beforeEach(async () => { api = await createTestApi(); });
  test.afterEach(async () => { await api.cleanup(); });

  test("schedule → save pre + post readings → complete", async () => {
    const machine = await createDialysisMachine(api, {
      code: "HD-01",
      name: "Fresenius 4008S #1",
      manufacturer: "Fresenius",
    });
    const patient = await createPatient(api, {
      first_name: "Dia", last_name: "Hasta",
      identifier_kind: "tc", identifier_value: nextValidTC(),
    });

    const createResp = await api.request.post("/api/dialysis-sessions", {
      data: {
        patient_id: patient.id,
        machine_id: machine.id,
        modality: "hemodialysis",
        vascular_access: "av_fistula",
        scheduled_at: new Date().toISOString(),
        duration_minutes: 240,
        dry_weight_kg: 72.5,
        ultrafiltration_target_ml: 2500,
        dialyzer_type: "Polyflux 17L",
      },
    });
    expect(createResp.status()).toBe(201);
    const session = (await createResp.json()) as { id: string; status: string; session_no: string };
    expect(session.status).toBe("scheduled");

    // Start session.
    const start = await api.request.post(`/api/dialysis-sessions/${session.id}/status`, {
      data: { status: "in_progress" },
    });
    expect(start.ok()).toBeTruthy();

    // Save pre-readings.
    const pre = await api.request.patch(`/api/dialysis-sessions/${session.id}/record`, {
      data: { pre_weight_kg: 75.0, pre_systolic_bp: 140, pre_diastolic_bp: 85 },
    });
    expect(pre.ok()).toBeTruthy();

    // Save post-readings (incremental update — pre stays intact via COALESCE).
    const post = await api.request.patch(`/api/dialysis-sessions/${session.id}/record`, {
      data: {
        post_weight_kg: 72.8,
        post_systolic_bp: 125,
        post_diastolic_bp: 78,
        actual_ultrafiltration_ml: 2200,
      },
    });
    expect(post.ok()).toBeTruthy();
    const after = await post.json();
    expect(after.pre_weight_kg).toBe(75.0);  // preserved from earlier write
    expect(after.post_weight_kg).toBe(72.8);
    expect(after.actual_ultrafiltration_ml).toBe(2200);

    // Complete.
    const done = await api.request.post(`/api/dialysis-sessions/${session.id}/status`, {
      data: { status: "completed" },
    });
    expect(done.ok()).toBeTruthy();
    expect((await done.json()).ended_at).toBeTruthy();
  });
});
