// Unit tests for the WebSocket client's reconnect + heartbeat machinery.
//
// We mock the global WebSocket constructor so each test drives the
// lifecycle (open → message → close, or open → idle → heartbeat trips)
// deterministically with fake timers. The point is to lock down the
// half-open mitigation we built specifically because Caliptic shipped
// without it and ate a P0 cache-staleness bug.

import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { WsClient } from "./ws-client";

// ---------- Fake WebSocket harness ----------
//
// One `FakeWebSocket` is created per `new WebSocket()` call. Tests pull
// the most-recent instance from the static `latest` reference and
// poke its handlers to simulate server-driven events.

class FakeWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances: FakeWebSocket[] = [];

  readyState: number = FakeWebSocket.CONNECTING;
  url: string;
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;
  closeCalls = 0;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  close() {
    this.closeCalls++;
    if (this.readyState === FakeWebSocket.CLOSED) return;
    this.readyState = FakeWebSocket.CLOSED;
    // Browsers always fire onclose after close(); mimic that.
    this.onclose?.();
  }

  // ---------- Test helpers (not part of the WS API surface) ----------
  fireOpen() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.();
  }
  fireMessage(payload: unknown) {
    this.onmessage?.({ data: JSON.stringify(payload) });
  }
  fireServerClose() {
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.();
  }
}

function lastWs(): FakeWebSocket {
  const ws = FakeWebSocket.instances[FakeWebSocket.instances.length - 1];
  if (!ws) throw new Error("no FakeWebSocket created yet");
  return ws;
}

// ---------- Setup ----------

const originalWebSocket = globalThis.WebSocket;

beforeEach(() => {
  vi.useFakeTimers();
  FakeWebSocket.instances = [];
  // `as unknown as` because FakeWebSocket only implements the shape we
  // touch — enough for runtime, not enough for the full DOM lib type.
  (globalThis as unknown as { WebSocket: unknown }).WebSocket = FakeWebSocket;
});

afterEach(() => {
  (globalThis as unknown as { WebSocket: typeof WebSocket }).WebSocket = originalWebSocket;
  vi.useRealTimers();
});

// ---------- Tests ----------

describe("WsClient — connection lifecycle", () => {
  test("fires onConnect on first OPEN, onReconnect on subsequent ones", () => {
    const onConnect = vi.fn();
    const onReconnect = vi.fn();
    const client = new WsClient({
      url: "ws://localhost/ws",
      getAccessToken: () => "tok",
      onMessage: vi.fn(),
      onConnect,
      onReconnect,
      reconnectDelayMs: 1000,
    });

    client.connect();
    lastWs().fireOpen();
    expect(onConnect).toHaveBeenCalledTimes(1);
    expect(onReconnect).not.toHaveBeenCalled();

    // Server drops the connection — client should auto-reconnect.
    lastWs().fireServerClose();
    vi.advanceTimersByTime(1000);
    expect(FakeWebSocket.instances).toHaveLength(2);

    lastWs().fireOpen();
    expect(onConnect).toHaveBeenCalledTimes(1); // still 1 — only first time
    expect(onReconnect).toHaveBeenCalledTimes(1);

    client.close();
  });

  test("appends the auth token to the URL", () => {
    const client = new WsClient({
      url: "ws://localhost/ws",
      getAccessToken: () => "abc xyz",
      onMessage: vi.fn(),
    });
    client.connect();
    expect(lastWs().url).toBe("ws://localhost/ws?token=abc%20xyz");
    client.close();
  });

  test("connect() while already open is a no-op", () => {
    const client = new WsClient({
      url: "ws://localhost/ws",
      getAccessToken: () => null,
      onMessage: vi.fn(),
    });
    client.connect();
    lastWs().fireOpen();
    const before = FakeWebSocket.instances.length;
    client.connect();
    expect(FakeWebSocket.instances).toHaveLength(before);
    client.close();
  });
});

describe("WsClient — half-open heartbeat detection", () => {
  test("force-closes the socket after heartbeatTimeoutMs of silence", () => {
    const client = new WsClient({
      url: "ws://localhost/ws",
      getAccessToken: () => null,
      onMessage: vi.fn(),
      heartbeatTimeoutMs: 60_000,
      reconnectDelayMs: 30_000, // big — keeps the test focused on the close
    });
    client.connect();
    const ws = lastWs();
    ws.fireOpen();
    expect(ws.closeCalls).toBe(0);

    // The heartbeat checker fires every min(15s, timeout/2). For a 60s
    // timeout that's 15s. We sit idle past the timeout.
    vi.advanceTimersByTime(75_000);
    expect(ws.closeCalls).toBeGreaterThanOrEqual(1);
    expect(ws.readyState).toBe(FakeWebSocket.CLOSED);

    client.close();
  });

  test("incoming messages bump lastMessageTime and keep the socket alive", () => {
    const onMessage = vi.fn();
    const client = new WsClient({
      url: "ws://localhost/ws",
      getAccessToken: () => null,
      onMessage,
      heartbeatTimeoutMs: 60_000,
      reconnectDelayMs: 999_999,
    });
    client.connect();
    const ws = lastWs();
    ws.fireOpen();

    // 50s pass, then a message arrives — well within the 60s budget.
    vi.advanceTimersByTime(50_000);
    ws.fireMessage({ type: "lab_result:new", payload: { id: "x" } });
    expect(onMessage).toHaveBeenCalledTimes(1);

    // 50s more — would have been 100s total without the bump, but the
    // bump reset the clock so we're at "50s since last message".
    vi.advanceTimersByTime(50_000);
    expect(ws.closeCalls).toBe(0);

    // 20s more pushes total time-since-message past 60s → close.
    vi.advanceTimersByTime(20_000);
    expect(ws.closeCalls).toBeGreaterThanOrEqual(1);

    client.close();
  });

  test("heartbeat-type messages are NOT forwarded to onMessage but DO bump the clock", () => {
    const onMessage = vi.fn();
    const client = new WsClient({
      url: "ws://localhost/ws",
      getAccessToken: () => null,
      onMessage,
      heartbeatTimeoutMs: 60_000,
      reconnectDelayMs: 999_999,
    });
    client.connect();
    const ws = lastWs();
    ws.fireOpen();

    vi.advanceTimersByTime(45_000);
    ws.fireMessage({ type: "heartbeat" });
    expect(onMessage).not.toHaveBeenCalled();

    // The heartbeat reset the timer — 45s more would have hit 90s total
    // but should still be under 60s since the last bump.
    vi.advanceTimersByTime(45_000);
    expect(ws.closeCalls).toBe(0);

    client.close();
  });

  test("malformed JSON does not throw and does not surface a message", () => {
    const onMessage = vi.fn();
    const client = new WsClient({
      url: "ws://localhost/ws",
      getAccessToken: () => null,
      onMessage,
    });
    client.connect();
    const ws = lastWs();
    ws.fireOpen();

    // Drive a raw onmessage with garbage — must not throw.
    expect(() => ws.onmessage?.({ data: "not-json {" })).not.toThrow();
    expect(onMessage).not.toHaveBeenCalled();

    client.close();
  });
});

describe("WsClient — reconnect scheduling", () => {
  test("server close triggers a reconnect after reconnectDelayMs", () => {
    const client = new WsClient({
      url: "ws://localhost/ws",
      getAccessToken: () => null,
      onMessage: vi.fn(),
      reconnectDelayMs: 5_000,
    });
    client.connect();
    lastWs().fireOpen();
    lastWs().fireServerClose();

    expect(FakeWebSocket.instances).toHaveLength(1);
    vi.advanceTimersByTime(4_999);
    expect(FakeWebSocket.instances).toHaveLength(1);
    vi.advanceTimersByTime(2);
    expect(FakeWebSocket.instances).toHaveLength(2);

    client.close();
  });

  test("client.close() prevents further reconnects", () => {
    const client = new WsClient({
      url: "ws://localhost/ws",
      getAccessToken: () => null,
      onMessage: vi.fn(),
      reconnectDelayMs: 1_000,
    });
    client.connect();
    lastWs().fireOpen();
    client.close();
    expect(FakeWebSocket.instances).toHaveLength(1);
    vi.advanceTimersByTime(10_000);
    expect(FakeWebSocket.instances).toHaveLength(1);
  });

  test("heartbeat timeout → forced close → reconnect", () => {
    const client = new WsClient({
      url: "ws://localhost/ws",
      getAccessToken: () => null,
      onMessage: vi.fn(),
      heartbeatTimeoutMs: 30_000,
      reconnectDelayMs: 5_000,
    });
    client.connect();
    lastWs().fireOpen();

    // Idle past timeout. Heartbeat checker (15s interval) only force-closes
    // on a tick *after* `elapsed > heartbeatTimeoutMs`. With 30s timeout
    // that means we need to reach the 45s tick (elapsed=45s > 30s).
    vi.advanceTimersByTime(46_000);
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(lastWs().closeCalls).toBeGreaterThanOrEqual(1);

    // Reconnect timer fires.
    vi.advanceTimersByTime(5_000);
    expect(FakeWebSocket.instances).toHaveLength(2);

    client.close();
  });
});
