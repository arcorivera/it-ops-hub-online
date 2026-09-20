import { api } from "./client";

export type ProjectStatus = "PLANNING" | "ACTIVE" | "ON_HOLD" | "COMPLETED" | "CANCELLED";
export type MilestoneStatus = "NOT_STARTED" | "IN_PROGRESS" | "COMPLETED" | "OVERDUE";

export interface Project {
  id: string;
  projectKey: string;
  name: string;
  description: string;
  ownerId: string | null;
  status: ProjectStatus;
  startDate: string | null;
  targetDate: string | null;
  memberIds: string[] | null;
  createdAt: string;
  updatedAt: string;
}

export interface Milestone {
  id: string;
  projectId: string;
  name: string;
  description: string;
  dueDate: string | null;
  status: MilestoneStatus;
  createdAt: string;
  updatedAt: string;
}

export interface ProjectStats {
  openTickets: number;
  criticalTickets: number;
  completedTickets: number;
  totalTickets: number;
  milestonesTotal: number;
  milestonesDone: number;
}

export const projectsApi = {
  list: (status?: string) => api.get<Project[]>(`/api/v1/projects/${status ? `?status=${status}` : ""}`),
  get: (id: string) => api.get<Project>(`/api/v1/projects/${id}`),
  stats: (id: string) => api.get<ProjectStats>(`/api/v1/projects/${id}/stats`),
  create: (payload: { projectKey: string; name: string; description?: string; startDate?: string; targetDate?: string }) =>
    api.post<Project>("/api/v1/projects/", payload),
  update: (id: string, payload: Partial<{ name: string; description: string; status: ProjectStatus; ownerId: string }>) =>
    api.put<Project>(`/api/v1/projects/${id}`, payload),

  listMilestones: (projectId: string) => api.get<Milestone[]>(`/api/v1/projects/${projectId}/milestones`),
  createMilestone: (projectId: string, payload: { name: string; description?: string; dueDate?: string }) =>
    api.post<Milestone>(`/api/v1/projects/${projectId}/milestones`, payload),
  updateMilestone: (projectId: string, milestoneId: string, payload: Partial<{ status: MilestoneStatus; name: string; dueDate: string }>) =>
    api.put<Milestone>(`/api/v1/projects/${projectId}/milestones/${milestoneId}`, payload),
};

export const PROJECT_STATUS_LABELS: Record<ProjectStatus, string> = {
  PLANNING: "Planning",
  ACTIVE: "Active",
  ON_HOLD: "On Hold",
  COMPLETED: "Completed",
  CANCELLED: "Cancelled",
};

export const MILESTONE_STATUS_LABELS: Record<MilestoneStatus, string> = {
  NOT_STARTED: "Not Started",
  IN_PROGRESS: "In Progress",
  COMPLETED: "Completed",
  OVERDUE: "Overdue",
};
