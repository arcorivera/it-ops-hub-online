import { api } from "./client";

export interface Team {
  id: string;
  name: string;
  description: string;
  memberIds: string[] | null;
  leadIds: string[] | null;
  createdAt: string;
  updatedAt: string;
}

export const teamsApi = {
  list: () => api.get<Team[]>("/api/v1/teams/"),
  create: (payload: { name: string; description?: string }) =>
    api.post<Team>("/api/v1/teams/", payload),
};
