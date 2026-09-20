import { api } from "./client";

export interface Notification {
  id: string;
  userId: string;
  type: string;
  title: string;
  message: string;
  entityType: string;
  entityId: string;
  isRead: boolean;
  createdAt: string;
}

export const notificationsApi = {
  list: (unreadOnly = false) => api.get<Notification[]>(`/api/v1/notifications/${unreadOnly ? "?unread=true" : ""}`),
  unreadCount: () => api.get<{ count: number }>("/api/v1/notifications/unread-count"),
  markRead: (id: string) => api.post<{ ok: boolean }>(`/api/v1/notifications/${id}/read`),
  markAllRead: () => api.post<{ ok: boolean }>("/api/v1/notifications/read-all"),
};
