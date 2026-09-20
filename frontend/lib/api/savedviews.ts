import { api } from "./client";

export interface SavedView {
  id: string;
  userId: string | null;
  name: string;
  filters: Record<string, unknown>;
  isSystem: boolean;
  createdAt: string;
}

export const savedViewsApi = {
  list: () => api.get<SavedView[]>("/api/v1/saved-views/"),
  create: (name: string, filters: Record<string, unknown>) =>
    api.post<SavedView>("/api/v1/saved-views/", { name, filters }),
  delete: (id: string) => api.delete<void>(`/api/v1/saved-views/${id}`),
};
