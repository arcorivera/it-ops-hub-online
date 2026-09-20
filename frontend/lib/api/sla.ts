import { api } from "./client";

export interface SLAStatus {
  ticketId: string;
  status: "WITHIN_SLA" | "WARNING" | "CRITICAL" | "BREACHED" | "PAUSED" | "RESOLVED";
  percentage: number;
  elapsedMinutes: number;
  resolutionMinutes: number;
  remainingMinutes: number;
  projectedDeadline: string;
  isPaused: boolean;
  usesBusinessHours: boolean;
}

export const slaApi = {
  get: (ticketId: string) => api.get<SLAStatus>(`/api/v1/tickets/${ticketId}/sla`),
};

export function slaStatusVariant(s: SLAStatus["status"]): "emerald" | "amber" | "orange" | "red" | "purple" | "slate" {
  switch (s) {
    case "WITHIN_SLA": return "emerald";
    case "WARNING": return "amber";
    case "CRITICAL": return "orange";
    case "BREACHED": return "red";
    case "PAUSED": return "purple";
    default: return "slate";
  }
}

export const SLA_STATUS_LABELS: Record<SLAStatus["status"], string> = {
  WITHIN_SLA: "Within SLA",
  WARNING: "Warning",
  CRITICAL: "Critical",
  BREACHED: "Breached",
  PAUSED: "Paused",
  RESOLVED: "Resolved",
};
