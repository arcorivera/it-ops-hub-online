import { api } from "./client";
import { getApiBase } from "./base";

export interface AuditEntry {
  id: string;
  userId?: string;
  action: string;
  entityType: string;
  entityId: string;
  details: Record<string, unknown>;
  ipAddress: string;
  createdAt: string;
}

const API_BASE = getApiBase();

export const auditApi = {
  list: async (limit = 50): Promise<{ entries: AuditEntry[]; total: number }> => {
    const res = await fetch(`${API_BASE}/api/v1/audit-logs?limit=${limit}`, { credentials: "include" });
    const body = await res.json();
    if (!res.ok) throw new Error(body?.error?.message ?? "Failed to load audit log");
    return { entries: body.data, total: body.meta?.total ?? 0 };
  },
};
