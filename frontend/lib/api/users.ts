import { api } from "./client";
import type { Role, User } from "./auth";

export interface CreateUserPayload {
  username: string;
  fullName: string;
  email: string;
  password: string;
  roles: Role[];
}

export interface UpdateUserPayload {
  fullName?: string;
  email?: string;
  isActive?: boolean;
  roles?: Role[];
  password?: string;
}

export const usersApi = {
  list: () => api.get<User[]>("/api/v1/users/"),
  get: (id: string) => api.get<User>(`/api/v1/users/${id}`),
  create: (payload: CreateUserPayload) => api.post<User>("/api/v1/users/", payload),
  update: (id: string, payload: UpdateUserPayload) => api.put<User>(`/api/v1/users/${id}`, payload),
};

export const ALL_ROLES: Role[] = ["ADMIN", "IT_MANAGER", "TEAM_LEAD", "AGENT", "REQUESTER", "VIEWER"];
