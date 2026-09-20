import { api } from "./client";

export type RCAStatus = "DRAFT" | "IN_REVIEW" | "COMPLETED";

export interface RCA {
  id: string;
  incidentId: string;
  incidentSummary: string;
  businessImpact: string;
  rootCause: string;
  contributingFactors: string;
  timeline: string;
  immediateFix: string;
  permanentFix: string;
  preventiveAction: string;
  responsibleTeamId: string | null;
  responsibleUserId: string | null;
  deploymentReference: string;
  status: RCAStatus;
  overrideBy: string | null;
  overrideReason: string;
  createdAt: string;
  updatedAt: string;
}

export const rcaApi = {
  get: (incidentId: string) => api.get<RCA>(`/api/v1/incidents/${incidentId}/rca`),
  update: (
    incidentId: string,
    payload: Partial<{
      incidentSummary: string;
      businessImpact: string;
      rootCause: string;
      contributingFactors: string;
      timeline: string;
      immediateFix: string;
      permanentFix: string;
      preventiveAction: string;
      deploymentReference: string;
    }>
  ) => api.put<RCA>(`/api/v1/incidents/${incidentId}/rca`, payload),
  changeStatus: (incidentId: string, status: RCAStatus) =>
    api.post<RCA>(`/api/v1/incidents/${incidentId}/rca/status`, { status }),
};

export const RCA_STATUS_LABELS: Record<RCAStatus, string> = {
  DRAFT: "Draft",
  IN_REVIEW: "In Review",
  COMPLETED: "Completed",
};
