import { api } from "./client";

export interface Setting {
  key: string;
  value: string;
  updatedAt: string;
}

export const settingsApi = {
  list: () => api.get<Setting[]>("/api/v1/settings/"),
  set: (key: string, value: string) => api.put<Setting>(`/api/v1/settings/${encodeURIComponent(key)}`, { value }),
  delete: (key: string) => api.delete<void>(`/api/v1/settings/${encodeURIComponent(key)}`),
};
