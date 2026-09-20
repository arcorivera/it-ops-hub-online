"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Sparkles, Loader2, ListOrdered, Briefcase } from "lucide-react";

import { useCurrentUser } from "@/hooks/use-current-user";
import { dashboardApi, type DashboardTicketSummary } from "@/lib/api/dashboard";
import { severityVariant } from "@/lib/ticket-display";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export default function DashboardPage() {
  const { data: user } = useCurrentUser();

  const { data: attention, isLoading } = useQuery({
    queryKey: ["dashboard", "attention"],
    queryFn: dashboardApi.attention,
    refetchInterval: 30_000,
  });

  // API responses may contain null arrays (for example when the backend has no
  // matching tickets). Normalize them before rendering so .length and .map
  // are always safe, even if an older backend is still running.
  const safeAttention = attention
    ? {
        ...attention,
        slaBreached: attention.slaBreached ?? [],
        slaCritical: attention.slaCritical ?? [],
        slaWarning: attention.slaWarning ?? [],
        followUpRequired: attention.followUpRequired ?? [],
        rcaRequired: attention.rcaRequired ?? [],
      }
    : null;

  return (
    <div className="mx-auto max-w-5xl px-8 py-8">
      <div className="mb-8 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">
            Welcome back{user?.fullName ? `, ${user.fullName.split(" ")[0]}` : ""}
          </h1>
          <p className="mt-1 text-sm text-slate-500">
            Signed in as <span className="font-medium text-slate-700">{user?.username}</span> ·{" "}
            {(user?.roles ?? []).join(", ")}
          </p>
        </div>
        <div className="flex gap-2">
          <Link
            href="/my-work"
            className="flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            <Briefcase className="h-4 w-4" /> My Work
          </Link>
          <Link
            href="/priority-queue"
            className="flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            <ListOrdered className="h-4 w-4" /> Priority Queue
          </Link>
        </div>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Sparkles className="h-5 w-5 text-indigo-600" />
            <CardTitle>What needs your attention?</CardTitle>
          </div>
          <CardDescription>Computed live from real SLA, follow-up, and escalation data.</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading || !safeAttention ? (
            <div className="flex justify-center py-10">
              <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
            </div>
          ) : (
            <div className="space-y-6">
              <div className="grid grid-cols-3 gap-3">
                <StatBox label="SLA Breached" count={safeAttention.slaBreached.length} tone="red" />
                <StatBox label="SLA Critical" count={safeAttention.slaCritical.length} tone="orange" />
                <StatBox label="SLA Warning" count={safeAttention.slaWarning.length} tone="amber" />
                <StatBox label="Follow-up Required" count={safeAttention.followUpRequired.length} tone="purple" />
                <StatBox label="Waiting for My Action" count={attention.waitingForMyActionCount} tone="blue" />
                <StatBox label="RCA Required" count={safeAttention.rcaRequired.length} tone="slate" />
              </div>

              <div className="grid grid-cols-3 gap-3 border-t border-slate-100 pt-4 text-center">
                <MiniStat label="Waiting for Dev" count={attention.waitingForDevelopmentCount} />
                <MiniStat label="Ready for UAT" count={attention.readyForUATCount} />
                <MiniStat label="Ready for Pre-Prod" count={attention.readyForPreProdCount} />
              </div>

              {safeAttention.slaBreached.length > 0 && (
                <TicketGroup title="SLA Breached" tickets={safeAttention.slaBreached} />
              )}
              {safeAttention.slaCritical.length > 0 && (
                <TicketGroup title="SLA Critical" tickets={safeAttention.slaCritical} />
              )}
              {safeAttention.followUpRequired.length > 0 && (
                <TicketGroup title="Follow-up Required" tickets={safeAttention.followUpRequired} />
              )}

              {safeAttention.slaBreached.length === 0 &&
                safeAttention.slaCritical.length === 0 &&
                safeAttention.slaWarning.length === 0 &&
                safeAttention.followUpRequired.length === 0 && (
                  <p className="rounded-lg border border-dashed border-slate-200 py-8 text-center text-sm text-slate-400">
                    Nothing urgent right now.
                  </p>
                )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatBox({ label, count, tone }: { label: string; count: number; tone: string }) {
  const toneClasses: Record<string, string> = {
    red: "bg-red-50 text-red-700",
    orange: "bg-orange-50 text-orange-700",
    amber: "bg-amber-50 text-amber-700",
    purple: "bg-purple-50 text-purple-700",
    blue: "bg-blue-50 text-blue-700",
    slate: "bg-slate-50 text-slate-700",
  };
  return (
    <div className={`rounded-lg p-4 ${toneClasses[tone]}`}>
      <p className="text-2xl font-semibold">{count}</p>
      <p className="text-xs font-medium">{label}</p>
    </div>
  );
}

function MiniStat({ label, count }: { label: string; count: number }) {
  return (
    <div>
      <p className="text-lg font-semibold text-slate-900">{count}</p>
      <p className="text-xs text-slate-400">{label}</p>
    </div>
  );
}

function TicketGroup({ title, tickets }: { title: string; tickets: DashboardTicketSummary[] }) {
  return (
    <div className="border-t border-slate-100 pt-4">
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-400">{title}</h3>
      <div className="space-y-1">
        {tickets.map((t) => (
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
              <span className="text-xs text-slate-400">{t.slaPercentage.toFixed(0)}%</span>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
