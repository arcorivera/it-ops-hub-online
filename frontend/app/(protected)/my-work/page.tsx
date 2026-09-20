"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";

import { dashboardApi, type DashboardTicketSummary } from "@/lib/api/dashboard";
import { severityVariant, statusVariant } from "@/lib/ticket-display";
import { STATUS_LABELS } from "@/lib/api/tickets";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";

export default function MyWorkPage() {
  const { data, isLoading } = useQuery({
    queryKey: ["dashboard", "my-work"],
    queryFn: dashboardApi.myWork,
    refetchInterval: 30_000,
  });

  if (isLoading || !data) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
      </div>
    );
  }

  const sections: { title: string; tickets: DashboardTicketSummary[]; emphasis?: boolean }[] = [
    { title: "Urgent", tickets: data.urgent ?? [], emphasis: true },
    { title: "SLA Breached", tickets: data.slaBreached ?? [] },
    { title: "SLA Critical", tickets: data.slaCritical ?? [] },
    { title: "SLA Warning", tickets: data.slaWarning ?? [] },
    { title: "Follow-up Required", tickets: data.followUpRequired ?? [] },
    { title: "Waiting for My Action", tickets: data.waitingForMyAction ?? [] },
    { title: "Ready for UAT", tickets: data.readyForUAT ?? [] },
    { title: "Ready for Pre-Prod", tickets: data.readyForPreProd ?? [] },
    { title: "Ready for Production", tickets: data.readyForProduction ?? [] },
    { title: "Recently Updated", tickets: data.recentlyUpdated ?? [] },
    { title: "Assigned to Me (all)", tickets: data.assignedToMe ?? [] },
  ];

  return (
    <div className="mx-auto max-w-5xl px-8 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold text-slate-900">My Work</h1>
        <p className="mt-1 text-sm text-slate-500">Everything on your plate, grouped by what it needs.</p>
      </div>

      <div className="space-y-6">
        {sections.map(
          (s) =>
            s.tickets.length > 0 && (
              <Card key={s.title} className={s.emphasis ? "border-red-200" : undefined}>
                <CardContent className="p-4">
                  <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-slate-700">
                    {s.title}
                    <span className="rounded-full bg-slate-100 px-1.5 py-0.5 text-xs font-medium text-slate-500">
                      {s.tickets.length}
                    </span>
                  </h3>
                  <div className="space-y-1">
                    {s.tickets.map((t) => (
                      <Link
                        key={t.id}
                        href={`/tickets/${t.id}`}
                        className="flex items-center justify-between rounded-md px-2 py-1.5 hover:bg-slate-50"
                      >
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-xs text-slate-400">{t.ticketNumber}</span>
                          <span className="text-sm text-slate-700">{t.title}</span>
                        </div>
                        <div className="flex items-center gap-2">
                          <Badge variant={severityVariant(t.severity)}>{t.severity}</Badge>
                          <Badge variant={statusVariant(t.status)}>{STATUS_LABELS[t.status]}</Badge>
                        </div>
                      </Link>
                    ))}
                  </div>
                </CardContent>
              </Card>
            )
        )}

        {sections.every((s) => s.tickets.length === 0) && (
          <p className="rounded-lg border border-dashed border-slate-200 py-12 text-center text-sm text-slate-400">
            Nothing assigned to you right now.
          </p>
        )}
      </div>
    </div>
  );
}
