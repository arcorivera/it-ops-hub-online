import { api } from "./client";
import { getApiBase } from "./base";

export type Severity = "S1" | "S2" | "S3" | "S4";
export type Priority = "Critical" | "High" | "Medium" | "Low";
export type Environment = "DEV" | "UAT" | "PRE-PROD" | "PRODUCTION";
export type TicketStatus =
  | "NEW"
  | "ACKNOWLEDGED"
  | "ASSIGNED"
  | "IN_PROGRESS"
  | "PENDING"
  | "FOR_UAT"
  | "UAT_FAILED"
  | "UAT_PASSED"
  | "FOR_PRE_PROD"
  | "PRE_PROD_FAILED"
  | "PRE_PROD_PASSED"
  | "FOR_PRODUCTION"
  | "PRODUCTION_FAILED"
  | "RESOLVED"
  | "CLOSED"
  | "CANCELLED";

export interface Ticket {
  id: string;
  ticketNumber: string;
  title: string;
  description: string;
  requesterId: string | null;
  assigneeId: string | null;
  teamId: string | null;
  projectId: string | null;
  categoryId: string | null;
  severity: Severity;
  priority: Priority;
  status: TicketStatus;
  environment: Environment;
  slaPolicyId: string | null;
  responseDeadline: string | null;
  resolutionDeadline: string | null;
  createdAt: string;
  acknowledgedAt: string | null;
  resolvedAt: string | null;
  closedAt: string | null;
  updatedAt: string;
  rcaRequired: boolean;
  escalationLevel: number;
}

export interface Comment {
  id: string;
  ticketId: string;
  userId: string | null;
  content: string;
  createdAt: string;
  updatedAt: string;
}

export interface HistoryEvent {
  id: string;
  ticketId: string;
  userId: string | null;
  eventType: string;
  field: string;
  oldValue: string;
  newValue: string;
  note: string;
  createdAt: string;
}

export interface Attachment {
  id: string;
  ticketId: string;
  userId: string | null;
  fileName: string;
  contentType: string;
  sizeBytes: number;
  createdAt: string;
}

export interface TicketListFilter {
  status?: string[];
  severity?: string[];
  priority?: string[];
  assigneeId?: string;
  assignee?: "me";
  teamId?: string;
  projectId?: string;
  search?: string;
  page?: number;
  pageSize?: number;
}

export interface TicketListResponse {
  tickets: Ticket[];
  total: number;
  page: number;
  pageSize: number;
}

const API_BASE = getApiBase();

export const ticketsApi = {
  list: async (filter: TicketListFilter = {}): Promise<TicketListResponse> => {
    const params = new URLSearchParams();
    if (filter.status?.length) params.set("status", filter.status.join(","));
    if (filter.severity?.length) params.set("severity", filter.severity.join(","));
    if (filter.priority?.length) params.set("priority", filter.priority.join(","));
    if (filter.assigneeId) params.set("assigneeId", filter.assigneeId);
    if (filter.assignee) params.set("assignee", filter.assignee);
    if (filter.teamId) params.set("teamId", filter.teamId);
    if (filter.projectId) params.set("projectId", filter.projectId);
    if (filter.search) params.set("search", filter.search);
    if (filter.page) params.set("page", String(filter.page));
    if (filter.pageSize) params.set("pageSize", String(filter.pageSize));

    const res = await fetch(`${API_BASE}/api/v1/tickets/?${params.toString()}`, { credentials: "include" });
    const body = await res.json();
    if (!res.ok) throw new Error(body?.error?.message ?? "Failed to load tickets");
    return {
      tickets: body.data ?? [],
      total: body.meta?.total ?? 0,
      page: body.meta?.page ?? 1,
      pageSize: body.meta?.pageSize ?? 25,
    };
  },

  get: (id: string) => api.get<Ticket>(`/api/v1/tickets/${id}`),

  create: (payload: {
    title: string;
    description?: string;
    severity?: Severity;
    priority?: Priority;
    environment?: Environment;
    teamId?: string | null;
    categoryId?: string | null;
    assigneeId?: string | null;
    projectId?: string | null;
  }) => api.post<Ticket>("/api/v1/tickets/", payload),

  assign: (id: string, assigneeId: string | null) =>
    api.post<Ticket>(`/api/v1/tickets/${id}/assign`, { assigneeId }),

  changeStatus: (id: string, status: TicketStatus, note?: string, force?: boolean) =>
    api.post<Ticket>(`/api/v1/tickets/${id}/status`, { status, note, force }),

  changeSeverity: (id: string, severity: Severity) =>
    api.post<Ticket>(`/api/v1/tickets/${id}/severity`, { severity }),

  changePriority: (id: string, priority: Priority) =>
    api.post<Ticket>(`/api/v1/tickets/${id}/priority`, { priority }),

  listComments: (id: string) => api.get<Comment[]>(`/api/v1/tickets/${id}/comments`),
  addComment: (id: string, content: string) =>
    api.post<Comment>(`/api/v1/tickets/${id}/comments`, { content }),
  deleteComment: (ticketId: string, commentId: string) =>
    api.delete<void>(`/api/v1/tickets/${ticketId}/comments/${commentId}`),

  listHistory: (id: string) => api.get<HistoryEvent[]>(`/api/v1/tickets/${id}/history`),

  listAttachments: (id: string) => api.get<Attachment[]>(`/api/v1/tickets/${id}/attachments`),

  uploadAttachment: async (id: string, file: File): Promise<Attachment> => {
    const formData = new FormData();
    formData.append("file", file);
    const res = await fetch(`${API_BASE}/api/v1/tickets/${id}/attachments`, {
      method: "POST",
      credentials: "include",
      body: formData,
    });
    const body = await res.json();
    if (!res.ok) throw new Error(body?.error?.message ?? "Upload failed");
    return body.data;
  },

  downloadAttachmentUrl: (ticketId: string, attachmentId: string) =>
    `${API_BASE}/api/v1/tickets/${ticketId}/attachments/${attachmentId}/download`,

  importCsv: async (file: File): Promise<{ imported: number; skipped: number; failed: { row: number; title: string; message: string }[] }> => {
    const formData = new FormData();
    formData.append("file", file);
    const res = await fetch(`${API_BASE}/api/v1/tickets/import`, {
      method: "POST",
      credentials: "include",
      body: formData,
    });
    const body = await res.json();
    if (!res.ok) throw new Error(body?.error?.message ?? "Import failed");
    return body.data;
  },
};

export const SEVERITY_LABELS: Record<Severity, string> = {
  S1: "S1 Critical",
  S2: "S2 High",
  S3: "S3 Medium",
  S4: "S4 Low",
};

export const STATUS_LABELS: Record<TicketStatus, string> = {
  NEW: "New",
  ACKNOWLEDGED: "Acknowledged",
  ASSIGNED: "Assigned",
  IN_PROGRESS: "In Progress",
  PENDING: "Pending",
  FOR_UAT: "For UAT",
  UAT_FAILED: "UAT Failed",
  UAT_PASSED: "UAT Passed",
  FOR_PRE_PROD: "For Pre-Prod",
  PRE_PROD_FAILED: "Pre-Prod Failed",
  PRE_PROD_PASSED: "Pre-Prod Passed",
  FOR_PRODUCTION: "For Production",
  PRODUCTION_FAILED: "Production Failed",
  RESOLVED: "Resolved",
  CLOSED: "Closed",
  CANCELLED: "Cancelled",
};
