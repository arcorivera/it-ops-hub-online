import { api } from "./client";

export type Role = "ADMIN" | "IT_MANAGER" | "TEAM_LEAD" | "AGENT" | "REQUESTER" | "VIEWER";

export interface User {
  id: string;
  username: string;
  fullName: string;
  email: string;
  isActive: boolean;
  roles: Role[];
  createdAt: string;
  updatedAt: string;
}

export interface AuthStatus {
  needsSetup: boolean;
}

export interface SetupPayload {
  username: string;
  fullName: string;
  email: string;
  password: string;
  confirmPassword: string;
}

export interface LoginPayload {
  username: string;
  password: string;
}

export const authApi = {
  status: () => api.get<AuthStatus>("/api/v1/auth/status"),
  setup: (payload: SetupPayload) => api.post<User>("/api/v1/auth/setup", payload),
  login: (payload: LoginPayload) => api.post<User>("/api/v1/auth/login", payload),
  logout: () => api.post<{ loggedOut: boolean }>("/api/v1/auth/logout"),
  me: () => api.get<User>("/api/v1/auth/me"),
};
