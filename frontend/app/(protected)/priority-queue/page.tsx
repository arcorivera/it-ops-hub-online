"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Loader2, Info } from "lucide-react";

import { dashboardApi } from "@/lib/api/dashboard";
import { severityVariant, priorityVariant } from "@/lib/ticket-display";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

export default function PriorityQueuePage() {
  const [scopeAll, setScopeAll] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["dashboard", "priority-queue", scopeAll],
    queryFn: () => dashboardApi.priorityQueue(scopeAll),
  });

  return (
    <div className="mx-auto max-w-4xl px-8 py-8">
      <div className="mb-2 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">What should I handle first?</h1>
          <p className="mt-1 text-sm text-slate-500">
            Ranked by severity, priority, live SLA percentage, escalation level, follow-up status, and
            ticket age — computed fresh on every load.
          </p>
        </div>
        <div className="flex gap-1 rounded-lg border border-slate-200 bg-white p-1">
          <button
            onClick={() => setScopeAll(false)}
            className={`rounded-md px-3 py-1.5 text-xs font-medium ${
              !scopeAll ? "bg-indigo-600 text-white" : "text-slate-600"
            }`}
          >
            My Tickets
          </button>
          <button
            onClick={() => setScopeAll(true)}
            className={`rounded-md px-3 py-1.5 text-xs font-medium ${
              scopeAll ? "bg-indigo-600 text-white" : "text-slate-600"
            }`}
          >
            All Open Tickets
          </button>
        </div>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
        </div>
      ) : data && data.length > 0 ? (
        <div className="mt-6 space-y-2">
          {data.map((item) => (
            <Link key={item.id} href={`/tickets/${item.id}`}>
              <Card className="transition-shadow hover:shadow-md">
                <CardContent className="flex items-center gap-4 p-4">
                  <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-slate-900 text-sm font-bold text-white">
                    {item.rank}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-xs text-slate-400">{item.ticketNumber}</span>
                      <p className="truncate font-medium text-slate-900">{item.title}</p>
                    </div>
                    <div className="mt-1 flex items-center gap-2">
                      <Badge variant={severityVariant(item.severity)}>{item.severity}</Badge>
                      <Badge variant={priorityVariant(item.priority)}>{item.priority}</Badge>
                      {item.slaPercentage > 0 && (
                        <span className="text-xs text-slate-400">SLA {item.slaPercentage.toFixed(0)}%</span>
                      )}
                      {item.escalationLevel > 0 && (
                        <span className="text-xs text-red-500">Escalated ×{item.escalationLevel}</span>
                      )}
                      {item.hasUnresolvedFollowUp && (
                        <span className="text-xs text-purple-500">Follow-up pending</span>
                      )}
                    </div>
                  </div>
                  <div className="flex-shrink-0 text-right">
                    <p className="text-lg font-semibold text-slate-900">{item.score}</p>
                    <p className="text-xs text-slate-400">score</p>
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      ) : (
        <div className="mt-8 flex items-center gap-2 rounded-lg border border-dashed border-slate-200 py-12 justify-center text-sm text-slate-400">
          <Info className="h-4 w-4" />
          No open tickets to prioritize.
        </div>
      )}
    </div>
  );
}
