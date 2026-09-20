import { api } from "./client";

export interface SearchResult {
  type: "ticket" | "incident" | "comment";
  id: string;
  entityId: string;
  number: string;
  title: string;
  snippet: string;
  status: string;
  severity?: string;
}

export const searchApi = {
  search: (q: string) => api.get<SearchResult[]>(`/api/v1/search?q=${encodeURIComponent(q)}`),
};
