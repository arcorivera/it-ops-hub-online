import { api } from "./client";
import type { Severity, Priority, TicketStatus } from "./tickets";

export interface DashboardTicketSummary {
  id: string;
  ticketNumber: string;
  title: string;
  severity: Severity;
  priority: Priority;
  status: TicketStatus;
  assigneeId: string | null;
  slaStatus: string;
  slaPercentage: number;
  escalationLevel: number;
  hasUnresolvedFollowUp: boolean;
}

export interface Attention {
  slaBreached: DashboardTicketSummary[];
  slaCritical: DashboardTicketSummary[];
  slaWarning: DashboardTicketSummary[];
  followUpRequired: DashboardTicketSummary[];
  rcaRequired: DashboardTicketSummary[];
  waitingForMyActionCount: number;
  waitingForDevelopmentCount: number;
  readyForUATCount: number;
  readyForPreProdCount: number;
  readyForProductionCount: number;
}

export interface MyWork {
  urgent: DashboardTicketSummary[];
  slaBreached: DashboardTicketSummary[];
  slaCritical: DashboardTicketSummary[];
  slaWarning: DashboardTicketSummary[];
  followUpRequired: DashboardTicketSummary[];
  assignedToMe: DashboardTicketSummary[];
  waitingForMyAction: DashboardTicketSummary[];
  readyForUAT: DashboardTicketSummary[];
  readyForPreProd: DashboardTicketSummary[];
  readyForProduction: DashboardTicketSummary[];
  recentlyUpdated: DashboardTicketSummary[];
}

export interface PriorityItem extends DashboardTicketSummary {
  rank: number;
  score: number;
  ageHours: number;
  sinceUpdateHours: number;
}

export const dashboardApi = {
  attention: async () => {
    const data = await api.get<Partial<Attention>>("/api/v1/dashboard/attention");
    return {
      slaBreached: data.slaBreached ?? [],
      slaCritical: data.slaCritical ?? [],
      slaWarning: data.slaWarning ?? [],
      followUpRequired: data.followUpRequired ?? [],
      rcaRequired: data.rcaRequired ?? [],
      waitingForMyActionCount: data.waitingForMyActionCount ?? 0,
      waitingForDevelopmentCount: data.waitingForDevelopmentCount ?? 0,
      readyForUATCount: data.readyForUATCount ?? 0,
      readyForPreProdCount: data.readyForPreProdCount ?? 0,
      readyForProductionCount: data.readyForProductionCount ?? 0,
    } satisfies Attention;
  },
  myWork: async () => {
    const data = await api.get<Partial<MyWork>>("/api/v1/dashboard/my-work");
    return {
      urgent: data?.urgent ?? [],
      slaBreached: data?.slaBreached ?? [],
      slaCritical: data?.slaCritical ?? [],
      slaWarning: data?.slaWarning ?? [],
      followUpRequired: data?.followUpRequired ?? [],
      assignedToMe: data?.assignedToMe ?? [],
      waitingForMyAction: data?.waitingForMyAction ?? [],
      readyForUAT: data?.readyForUAT ?? [],
      readyForPreProd: data?.readyForPreProd ?? [],
      readyForProduction: data?.readyForProduction ?? [],
      recentlyUpdated: data?.recentlyUpdated ?? [],
    } satisfies MyWork;
  },
  priorityQueue: (scopeAll = false) =>
    api.get<PriorityItem[]>(`/api/v1/dashboard/priority-queue${scopeAll ? "?scope=all" : ""}`),
};
