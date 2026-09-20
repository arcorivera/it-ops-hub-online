"use client";

import { useQuery } from "@tanstack/react-query";
import { Loader2, ScrollText } from "lucide-react";

import { auditApi } from "@/lib/api/audit";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

const actionTone: Record<string, "slate" | "blue" | "amber" | "emerald" | "red"> = {
  CREATE: "emerald",
  UPDATE: "blue",
  DELETE: "red",
  LOGIN: "slate",
  LOGOUT: "slate",
  LOGIN_FAILED: "red",
  ASSIGN: "blue",
  RESTORE: "amber",
  IMPORT: "blue",
};

function toneFor(action: string): "slate" | "blue" | "amber" | "emerald" | "red" {
  if (action in actionTone) return actionTone[action];
  if (action.includes("OVERRIDE")) return "amber";
  if (action.includes("STATUS")) return "blue";
  return "slate";
}

export default function AuditLogPage() {
  const { data, isLoading } = useQuery({ queryKey: ["audit-log"], queryFn: () => auditApi.list(100) });

  return (
    <div className="mx-auto max-w-4xl px-8 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">Audit Log</h1>
        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
          {data ? `${data.total} recorded actions` : "Loading…"}
        </p>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : data && data.entries.length > 0 ? (
        <Card>
          <CardContent className="p-0">
            {data.entries.map((e) => (
              <div
                key={e.id}
                className="flex items-center justify-between border-b border-slate-100 px-5 py-3 last:border-0 dark:border-slate-800"
              >
                <div className="flex items-center gap-3">
                  <Badge variant={toneFor(e.action)}>{e.action}</Badge>
                  <span className="text-sm text-slate-700 dark:text-slate-300">
                    {e.entityType}
                    {e.entityId ? ` · ${e.entityId.slice(0, 12)}…` : ""}
                  </span>
                </div>
                <span className="text-xs text-slate-400">{new Date(e.createdAt).toLocaleString()}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      ) : (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-slate-200 py-16 text-center text-sm text-slate-400 dark:border-slate-800">
          <ScrollText className="h-6 w-6" />
          No audit entries yet.
        </div>
      )}
    </div>
  );
}
