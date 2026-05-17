// HTTP client with tenant headers and JSON envelope.
// Auth + organization/branch IDs are injected via setters so the platform
// layer can keep references when stores change.

export type ApiClientOptions = {
  baseUrl: string;
};

export type RequestInit = {
  method?: string;
  body?: unknown;
  // When true, body is sent as-is (string / FormData / Blob). Caller is
  // responsible for setting a matching Content-Type header.
  rawBody?: boolean;
  headers?: Record<string, string>;
  signal?: AbortSignal;
};

export class ApiError extends Error {
  status: number;
  code: string;
  details?: Record<string, unknown>;

  constructor(status: number, code: string, message: string, details?: Record<string, unknown>) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export class ApiClient {
  private baseUrl: string;
  private getAccessToken: () => string | null = () => null;
  private getOrgId: () => string | null = () => null;
  private getBranchId: () => string | null = () => null;
  private onUnauthorized: () => void = () => {};

  constructor(opts: ApiClientOptions) {
    this.baseUrl = opts.baseUrl.replace(/\/$/, "");
  }

  setAuthGetter(fn: () => string | null) { this.getAccessToken = fn; }
  setOrgGetter(fn: () => string | null) { this.getOrgId = fn; }
  setBranchGetter(fn: () => string | null) { this.getBranchId = fn; }
  setOnUnauthorized(fn: () => void) { this.onUnauthorized = fn; }

  async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...init.headers,
    };
    const token = this.getAccessToken();
    if (token) headers["Authorization"] = `Bearer ${token}`;
    const orgId = this.getOrgId();
    if (orgId) headers["X-Organization-ID"] = orgId;
    const branchId = this.getBranchId();
    if (branchId) headers["X-Branch-ID"] = branchId;

    let bodyToSend: BodyInit | undefined;
    if (init.body !== undefined) {
      if (init.rawBody) {
        bodyToSend = init.body as BodyInit;
      } else {
        bodyToSend = JSON.stringify(init.body);
      }
    }
    const res = await fetch(`${this.baseUrl}${path}`, {
      method: init.method ?? "GET",
      headers,
      body: bodyToSend,
      signal: init.signal,
      credentials: "include",
    });

    if (res.status === 401) this.onUnauthorized();

    if (res.status === 204) return undefined as T;

    let payload: unknown;
    try {
      payload = await res.json();
    } catch {
      payload = null;
    }

    if (!res.ok) {
      const err = (payload as { error?: { code?: string; message?: string; details?: Record<string, unknown> } })?.error;
      throw new ApiError(
        res.status,
        err?.code ?? "unknown",
        err?.message ?? `HTTP ${res.status}`,
        err?.details,
      );
    }

    return payload as T;
  }

  get<T>(path: string, init?: RequestInit) { return this.request<T>(path, { ...init, method: "GET" }); }
  post<T>(path: string, body?: unknown, init?: RequestInit) { return this.request<T>(path, { ...init, method: "POST", body }); }
  put<T>(path: string, body?: unknown, init?: RequestInit) { return this.request<T>(path, { ...init, method: "PUT", body }); }
  patch<T>(path: string, body?: unknown, init?: RequestInit) { return this.request<T>(path, { ...init, method: "PATCH", body }); }
  delete<T>(path: string, init?: RequestInit) { return this.request<T>(path, { ...init, method: "DELETE" }); }
}

let _client: ApiClient | null = null;

export function configureApi(opts: ApiClientOptions): ApiClient {
  _client = new ApiClient(opts);
  return _client;
}

export function api(): ApiClient {
  if (!_client) throw new Error("ApiClient not configured. Call configureApi() first.");
  return _client;
}
