// WebSocket client with client-side heartbeat timeout detection.
// Caliptic had a P0 cache-staleness bug because browsers don't expose
// ping/pong frames to JS — half-open TCP connections never trigger onclose.
// We mitigate by tracking lastMessageTime and force-closing when stale.

import { logger } from "../logger";

export type WsMessage = {
  type: string;
  scope?: string;
  payload?: unknown;
  ts?: number;
};

export type WsClientOptions = {
  url: string;
  getAccessToken: () => string | null;
  onMessage: (msg: WsMessage) => void;
  onConnect?: () => void;
  onReconnect?: () => void;
  heartbeatTimeoutMs?: number;
  reconnectDelayMs?: number;
};

export class WsClient {
  private opts: Required<Omit<WsClientOptions, "onConnect" | "onReconnect">> & {
    onConnect?: () => void;
    onReconnect?: () => void;
  };
  private ws: WebSocket | null = null;
  private lastMessageTime = 0;
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private closed = false;
  private hasConnectedOnce = false;

  constructor(opts: WsClientOptions) {
    this.opts = {
      url: opts.url,
      getAccessToken: opts.getAccessToken,
      onMessage: opts.onMessage,
      onConnect: opts.onConnect,
      onReconnect: opts.onReconnect,
      heartbeatTimeoutMs: opts.heartbeatTimeoutMs ?? 60_000,
      reconnectDelayMs: opts.reconnectDelayMs ?? 3_000,
    };
  }

  connect() {
    if (this.closed) return;
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    const token = this.opts.getAccessToken();
    const url = token ? `${this.opts.url}?token=${encodeURIComponent(token)}` : this.opts.url;

    try {
      this.ws = new WebSocket(url);
    } catch (err) {
      logger.error("WS construction failed", { err: String(err) });
      this.scheduleReconnect();
      return;
    }

    this.ws.onopen = () => {
      logger.info("WS connected");
      this.lastMessageTime = Date.now();
      this.startHeartbeat();
      if (this.hasConnectedOnce) {
        this.opts.onReconnect?.();
      } else {
        this.opts.onConnect?.();
        this.hasConnectedOnce = true;
      }
    };

    this.ws.onmessage = (event) => {
      this.lastMessageTime = Date.now();
      let msg: WsMessage;
      try {
        msg = JSON.parse(event.data);
      } catch {
        return;
      }
      if (msg.type === "heartbeat") return;
      this.opts.onMessage(msg);
    };

    this.ws.onerror = (event) => {
      logger.warn("WS error", { event: String(event) });
    };

    this.ws.onclose = () => {
      logger.info("WS closed");
      this.stopHeartbeat();
      this.ws = null;
      this.scheduleReconnect();
    };
  }

  close() {
    this.closed = true;
    this.stopHeartbeat();
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  private startHeartbeat() {
    this.stopHeartbeat();
    this.heartbeatInterval = setInterval(() => {
      const elapsed = Date.now() - this.lastMessageTime;
      if (elapsed > this.opts.heartbeatTimeoutMs) {
        logger.warn("WS heartbeat timeout, force-closing", { elapsed });
        this.ws?.close();
      }
    }, Math.min(15_000, this.opts.heartbeatTimeoutMs / 2));
  }

  private stopHeartbeat() {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }

  private scheduleReconnect() {
    if (this.closed) return;
    if (this.reconnectTimer) return;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, this.opts.reconnectDelayMs);
  }
}
