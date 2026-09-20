import { api } from "./client";

export type IncidentStatus = "OPEN" | "INVESTIGATING" | "MITIGATED" | "RESOLVED" | "CLOSED";

export interface Incident {
  id: string;
  incidentNumber: string;
  title: string;
  description: string;
  severity: "S1" | "S2" | "S3" | "S4";
  impact: string;
  affectedSystem: string;
  environment: string;
  ownerId: string | null;
  status: IncidentStatus;
  ticketId: string | null;
  startedAt: string;
  resolvedAt: string | null;
  createdAt: string;
  updatedAt: string;
  rcaRequired: boolean;
}

export const incidentsApi = {
  list: (status?: string, severity?: string) => {
    const params = new URLSearchParams();
    if (status) params.set("status", status);
    if (severity) params.set("severity", severity);
    return api.get<Incident[]>(`/api/v1/incidents/?${params.toString()}`);
  },
  get: (id: string) => api.get<Incident>(`/api/v1/incidents/${id}`),
  create: (payload: {
    title: string;
    description?: string;
    severity: string;
    impact?: string;
    affectedSystem?: string;
    environment?: string;
    ticketId?: string | null;
  }) => api.post<Incident>("/api/v1/incidents/", payload),
  changeStatus: (id: string, status: IncidentStatus, force?: boolean) =>
    api.post<Incident>(`/api/v1/incidents/${id}/status`, { status, force }),
};

export const INCIDENT_STATUS_LABELS: Record<IncidentStatus, string> = {
  OPEN: "Open",
  INVESTIGATING: "Investigating",
  MITIGATED: "Mitigated",
  RESOLVED: "Resolved",
  CLOSED: "Closed",
};
