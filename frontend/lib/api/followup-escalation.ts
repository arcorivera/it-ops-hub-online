import { api } from "./client";

export interface FollowUp {
  id: string;
  ticketId: string;
  createdBy: string | null;
  reason: string;
  isSystem: boolean;
  resolved: boolean;
  createdAt: string;
}

export interface EscalationEvent {
  id: string;
  ticketId: string;
  thresholdPercent: number;
  escalatedToRole: string;
  escalatedToUser: string | null;
  createdAt: string;
}

export const followupApi = {
  list: (ticketId: string) => api.get<FollowUp[]>(`/api/v1/tickets/${ticketId}/followups`),
  trigger: (ticketId: string, reason?: string) =>
    api.post<{ created: boolean }>(`/api/v1/tickets/${ticketId}/follow-up`, { reason }),
};

export const escalationApi = {
  list: (ticketId: string) => api.get<EscalationEvent[]>(`/api/v1/tickets/${ticketId}/escalations`),
  trigger: (ticketId: string) => api.post<{ created: boolean }>(`/api/v1/tickets/${ticketId}/escalate`, {}),
};
