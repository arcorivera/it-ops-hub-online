import { getApiBase } from "./base";

const API_BASE = getApiBase();

export class ApiError extends Error {
  status: number;
  code: string;
  details?: Record<string, string>;

  constructor(status: number, code: string, message: string, details?: Record<string, string>) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

interface Envelope<T> {
  data: T;
  message?: string;
  meta?: unknown;
}

interface ErrorEnvelope {
  error: {
    code: string;
    message: string;
    details?: Record<string, string>;
  };
}

const COLLECTION_KEYS = new Set([
  "tickets", "urgent", "slaBreached", "slaCritical", "slaWarning", "followUpRequired", "rcaRequired",
  "assignedToMe", "waitingForMyAction", "readyForUAT", "readyForPreProd", "readyForProduction", "recentlyUpdated",
  "entries", "projects", "teams", "incidents", "users", "settings", "backups", "comments", "history",
  "attachments", "milestones", "testCases", "deployments", "followups", "escalations", "notifications",
  "savedViews", "results", "ticketVolume", "byTeam", "byAssignee", "byCategory", "byProject", "roles",
  "members", "leads", "failed", "searchResults", "items",
]);

/**
 * The Go JSON encoder emits nil slices as null. The frontend API contract
 * treats collections as arrays, including when they are empty. Normalize that
 * contract once at the HTTP boundary so every module is protected.
 */
function normalizeCollections(value: unknown, key?: string): unknown {
  if (value === null) {
    return key && COLLECTION_KEYS.has(key) ? [] : null;
  }
  if (Array.isArray(value)) {
    return value.map((item) => normalizeCollections(item));
  }
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    const out: Record<string, unknown> = {};
    for (const [childKey, childValue] of Object.entries(record)) {
      out[childKey] = normalizeCollections(childValue, childKey);
    }
    return out;
  }
  return value;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });

  let body: unknown = null;
  const text = await res.text();
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      // non-JSON response body; leave as null
    }
  }

  if (!res.ok) {
    const errBody = body as ErrorEnvelope | null;
    throw new ApiError(
      res.status,
      errBody?.error?.code ?? "UNKNOWN_ERROR",
      errBody?.error?.message ?? `Request failed with status ${res.status}`,
      errBody?.error?.details
    );
  }

  const envelope = body as Envelope<T>;
  // A successful list endpoint may legitimately have data: null when there
  // are no rows. Treat that as an empty collection at the boundary.
  if (envelope?.data === null) return [] as unknown as T;
  return normalizeCollections(envelope?.data) as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path, { method: "GET" }),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "PUT", body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "PATCH", body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};
