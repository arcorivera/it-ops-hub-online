import { api } from "./client";
import { getApiBase } from "./base";

export interface VolumePoint { date: string; count: number; }
export interface BreakdownItem { label: string; count: number; }
export interface SLACompliance { withinSLA: number; breached: number; totalMeasured: number; compliancePercent: number; }

export interface ReportSummary {
  ticketVolume: VolumePoint[];
  slaCompliance: SLACompliance;
  avgResponseMinutes: number;
  avgResolutionMinutes: number;
  byTeam: BreakdownItem[];
  byAssignee: BreakdownItem[];
  byCategory: BreakdownItem[];
  byProject: BreakdownItem[];
  productionIncidents: number;
  totalTickets: number;
}

export interface ManagementDashboard {
  slaCompliancePercent: number;
  openCriticalIssues: number;
  slaBreaches: number;
  avgResponseMinutes: number;
  avgResolutionMinutes: number;
  ticketsThisMonth: number;
  openIncidents: number;
  productionIncidents: number;
}

export interface ReportFilter {
  startDate?: string;
  endDate?: string;
  teamId?: string;
  projectId?: string;
  severity?: string;
}

const API_BASE = getApiBase();

function buildParams(f: ReportFilter): string {
  const params = new URLSearchParams();
  if (f.startDate) params.set("startDate", f.startDate);
  if (f.endDate) params.set("endDate", f.endDate);
  if (f.teamId) params.set("teamId", f.teamId);
  if (f.projectId) params.set("projectId", f.projectId);
  if (f.severity) params.set("severity", f.severity);
  return params.toString();
}

export const reportsApi = {
  summary: (f: ReportFilter = {}) => api.get<ReportSummary>(`/api/v1/reports/summary?${buildParams(f)}`),
  management: () => api.get<ManagementDashboard>("/api/v1/reports/management"),
  exportUrl: (f: ReportFilter = {}) => `${API_BASE}/api/v1/reports/export.csv?${buildParams(f)}`,
};
