"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Loader2, Download, TrendingUp } from "lucide-react";

import { reportsApi } from "@/lib/api/reports";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export default function ReportsPage() {
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");

  const { data: management, isLoading: loadingMgmt } = useQuery({
    queryKey: ["reports", "management"],
    queryFn: reportsApi.management,
  });

  const { data: summary, isLoading: loadingSummary } = useQuery({
    queryKey: ["reports", "summary", startDate, endDate],
    queryFn: () => reportsApi.summary({ startDate: startDate || undefined, endDate: endDate || undefined }),
  });

  const safeSummary = summary
    ? {
        ...summary,
        ticketVolume: summary.ticketVolume ?? [],
        byTeam: summary.byTeam ?? [],
        byAssignee: summary.byAssignee ?? [],
        byCategory: summary.byCategory ?? [],
        byProject: summary.byProject ?? [],
      }
    : null;

  return (
    <div className="mx-auto max-w-5xl px-8 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">Reports</h1>
          <p className="mt-1 text-sm text-slate-500">Executive KPIs and detailed breakdowns, computed live.</p>
        </div>
        <a href={reportsApi.exportUrl({ startDate: startDate || undefined, endDate: endDate || undefined })}>
          <Button variant="outline">
            <Download className="h-4 w-4" />
            Export CSV
          </Button>
        </a>
      </div>

      <div className="mb-6 grid grid-cols-4 gap-3">
        {loadingMgmt || !management ? (
          <div className="col-span-4 flex justify-center py-8">
            <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
          </div>
        ) : (
          <>
            <KPI label="SLA Compliance" value={`${management.slaCompliancePercent}%`} />
            <KPI label="Open Critical Issues" value={management.openCriticalIssues} tone="red" />
            <KPI label="SLA Breaches" value={management.slaBreaches} tone="red" />
            <KPI label="Tickets This Month" value={management.ticketsThisMonth} />
            <KPI label="Avg Response Time" value={formatMinutes(management.avgResponseMinutes)} />
            <KPI label="Avg Resolution Time" value={formatMinutes(management.avgResolutionMinutes)} />
            <KPI label="Open Incidents" value={management.openIncidents} tone="amber" />
            <KPI label="Production Incidents" value={management.productionIncidents} tone="amber" />
          </>
        )}
      </div>

      <Card className="mb-6">
        <CardContent className="flex items-end gap-3 p-4">
          <div className="space-y-1.5">
            <Label htmlFor="startDate">From</Label>
            <Input id="startDate" type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="endDate">To</Label>
            <Input id="endDate" type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
          </div>
        </CardContent>
      </Card>

      {loadingSummary || !safeSummary ? (
        <div className="flex justify-center py-10">
          <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : (
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2">
                <TrendingUp className="h-4 w-4 text-indigo-600" />
                <CardTitle className="text-base">Ticket Volume ({safeSummary.totalTickets} total)</CardTitle>
              </div>
            </CardHeader>
            <CardContent>
              {safeSummary.ticketVolume.length > 0 ? (
                <div className="flex h-32 items-end gap-1">
                  {safeSummary.ticketVolume.map((v) => {
                    const max = Math.max(...safeSummary.ticketVolume.map((x) => x.count));
                    return (
                      <div key={v.date} className="flex flex-1 flex-col items-center gap-1">
                        <div
                          className="w-full rounded-t bg-indigo-400"
                          style={{ height: `${(v.count / max) * 100}%`, minHeight: 4 }}
                          title={`${v.date}: ${v.count}`}
                        />
                      </div>
                    );
                  })}
                </div>
              ) : (
                <p className="py-4 text-center text-sm text-slate-400">No ticket data in this range.</p>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">SLA Compliance</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-6">
                <div>
                  <p className="text-3xl font-semibold text-slate-900">{safeSummary.slaCompliance.compliancePercent}%</p>
                  <p className="text-xs text-slate-400">
                    {safeSummary.slaCompliance.withinSLA} within / {safeSummary.slaCompliance.breached} breached
                  </p>
                </div>
                <div className="h-2 flex-1 overflow-hidden rounded-full bg-red-100">
                  <div
                    className="h-full rounded-full bg-emerald-500"
                    style={{ width: `${safeSummary.slaCompliance.compliancePercent}%` }}
                  />
                </div>
              </div>
            </CardContent>
          </Card>

          <div className="grid grid-cols-2 gap-4">
            <BreakdownCard title="By Team" items={safeSummary.byTeam} />
            <BreakdownCard title="By Assignee" items={safeSummary.byAssignee} />
            <BreakdownCard title="By Category" items={safeSummary.byCategory} />
            <BreakdownCard title="By Project" items={safeSummary.byProject} />
          </div>
        </div>
      )}
    </div>
  );
}

function KPI({ label, value, tone }: { label: string; value: string | number; tone?: "red" | "amber" }) {
  const color = tone === "red" ? "text-red-600" : tone === "amber" ? "text-amber-600" : "text-slate-900";
  return (
    <Card>
      <CardContent className="p-4">
        <p className={`text-2xl font-semibold ${color}`}>{value}</p>
        <p className="text-xs text-slate-400">{label}</p>
      </CardContent>
    </Card>
  );
}

function BreakdownCard({ title, items }: { title: string; items: { label: string; count: number }[] }) {
  const max = items.length > 0 ? Math.max(...items.map((i) => i.count)) : 1;
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {items.length > 0 ? (
          items.map((item) => (
            <div key={item.label} className="flex items-center gap-2 text-sm">
              <span className="w-24 flex-shrink-0 truncate text-slate-600">{item.label}</span>
              <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-100">
                <div className="h-full rounded-full bg-indigo-400" style={{ width: `${(item.count / max) * 100}%` }} />
              </div>
              <span className="w-6 text-right text-xs text-slate-400">{item.count}</span>
            </div>
          ))
        ) : (
          <p className="text-xs text-slate-400">No data.</p>
        )}
      </CardContent>
    </Card>
  );
}

function formatMinutes(mins: number): string {
  if (!mins) return "—";
  if (mins < 60) return `${mins.toFixed(0)}m`;
  return `${(mins / 60).toFixed(1)}h`;
}
