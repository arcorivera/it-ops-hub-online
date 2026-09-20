import { api } from "./client";

export interface Backup {
  id: string;
  fileName: string;
  sizeBytes: number;
  createdBy: string | null;
  isSafety: boolean;
  createdAt: string;
}

export const backupsApi = {
  list: () => api.get<Backup[]>("/api/v1/backups/"),
  create: () => api.post<Backup>("/api/v1/backups/"),
  restore: (id: string) => api.post<{ restoring: boolean; message: string }>(`/api/v1/backups/${id}/restore`),
};
