"use client";

import { use, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, ArrowLeft, Plus, CheckCircle2, Circle, AlertCircle } from "lucide-react";

import { projectsApi, PROJECT_STATUS_LABELS, MILESTONE_STATUS_LABELS } from "@/lib/api/projects";
import { ticketsApi, STATUS_LABELS } from "@/lib/api/tickets";
import { severityVariant, statusVariant } from "@/lib/ticket-display";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent } from "@/components/ui/card";

const TABS = ["Overview", "Tickets", "Milestones"] as const;
type Tab = (typeof TABS)[number];

const statusTone: Record<string, "slate" | "blue" | "amber" | "emerald" | "red"> = {
  PLANNING: "slate", ACTIVE: "blue", ON_HOLD: "amber", COMPLETED: "emerald", CANCELLED: "red",
};
const milestoneTone: Record<string, "slate" | "blue" | "emerald" | "red"> = {
  NOT_STARTED: "slate", IN_PROGRESS: "blue", COMPLETED: "emerald", OVERDUE: "red",
};

export default function ProjectDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [tab, setTab] = useState<Tab>("Overview");
  const [newMilestoneName, setNewMilestoneName] = useState("");
  const [newMilestoneDue, setNewMilestoneDue] = useState("");
  const queryClient = useQueryClient();

  const { data: project, isLoading } = useQuery({ queryKey: ["project", id], queryFn: () => projectsApi.get(id) });
  const { data: stats } = useQuery({ queryKey: ["project-stats", id], queryFn: () => projectsApi.stats(id) });
  const { data: milestones } = useQuery({
    queryKey: ["project-milestones", id],
    queryFn: () => projectsApi.listMilestones(id),
    enabled: tab === "Milestones" || tab === "Overview",
  });
  const { data: ticketsResult } = useQuery({
    queryKey: ["project-tickets", id],
    queryFn: () => ticketsApi.list({ projectId: id, pageSize: 50 }),
    enabled: tab === "Tickets",
  });

  const createMilestone = useMutation({
    mutationFn: () => projectsApi.createMilestone(id, { name: newMilestoneName, dueDate: newMilestoneDue || undefined }),
    onSuccess: () => {
      setNewMilestoneName("");
      setNewMilestoneDue("");
      queryClient.invalidateQueries({ queryKey: ["project-milestones", id] });
    },
  });

  const completeMilestone = useMutation({
    mutationFn: (milestoneId: string) => projectsApi.updateMilestone(id, milestoneId, { status: "COMPLETED" }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["project-milestones", id] }),
  });

  if (isLoading || !project) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
      </div>
    );
  }

  const projectTickets = ticketsResult?.tickets ?? [];

  return (
    <div className="mx-auto max-w-5xl px-8 py-8">
      <Link href="/projects" className="mb-4 flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700">
        <ArrowLeft className="h-4 w-4" /> Back to Projects
      </Link>

      <div className="mb-6">
        <div className="mb-2 flex items-center gap-2">
          <span className="font-mono text-sm text-slate-400">{project.projectKey}</span>
          <Badge variant={statusTone[project.status]}>{PROJECT_STATUS_LABELS[project.status]}</Badge>
        </div>
        <h1 className="text-2xl font-semibold text-slate-900">{project.name}</h1>
        {project.description && <p className="mt-1 text-sm text-slate-500">{project.description}</p>}
      </div>

      <div className="mb-4 flex gap-1 border-b border-slate-200">
        {TABS.map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-medium transition-colors ${
              tab === t ? "border-b-2 border-indigo-600 text-indigo-600" : "text-slate-500 hover:text-slate-700"
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === "Overview" && stats && (
        <div className="space-y-6">
          <div className="grid grid-cols-4 gap-3">
            <StatCard label="Open Tickets" value={stats.openTickets} />
            <StatCard label="Critical (S1/S2)" value={stats.criticalTickets} tone="red" />
            <StatCard label="Completed" value={stats.completedTickets} tone="emerald" />
            <StatCard label="Total Tickets" value={stats.totalTickets} />
          </div>
          <Card>
            <CardContent className="p-5">
              <div className="mb-2 flex items-center justify-between text-sm">
                <span className="font-medium text-slate-700">Milestone Progress</span>
                <span className="text-slate-500">
                  {stats.milestonesDone} / {stats.milestonesTotal}
                </span>
              </div>
              <div className="h-2 w-full overflow-hidden rounded-full bg-slate-100">
                <div
                  className="h-full rounded-full bg-indigo-500"
                  style={{
                    width: `${stats.milestonesTotal > 0 ? (stats.milestonesDone / stats.milestonesTotal) * 100 : 0}%`,
                  }}
                />
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {tab === "Tickets" && (
        <Card>
          <CardContent className="p-0">
            {projectTickets.length > 0 ? (
              projectTickets.map((t) => (
                <Link
                  key={t.id}
                  href={`/tickets/${t.id}`}
                  className="flex items-center justify-between border-b border-slate-100 px-5 py-3 last:border-0 hover:bg-slate-50"
                >
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs text-slate-400">{t.ticketNumber}</span>
                    <span className="text-sm text-slate-800">{t.title}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant={severityVariant(t.severity)}>{t.severity}</Badge>
                    <Badge variant={statusVariant(t.status)}>{STATUS_LABELS[t.status]}</Badge>
                  </div>
                </Link>
              ))
            ) : (
              <p className="py-10 text-center text-sm text-slate-400">No tickets linked to this project yet.</p>
            )}
          </CardContent>
        </Card>
      )}

      {tab === "Milestones" && (
        <div className="space-y-4">
          <Card>
            <CardContent className="flex gap-2 p-4">
              <Input
                placeholder="Milestone name"
                value={newMilestoneName}
                onChange={(e) => setNewMilestoneName(e.target.value)}
                className="flex-1"
              />
              <Input
                type="date"
                value={newMilestoneDue}
                onChange={(e) => setNewMilestoneDue(e.target.value)}
                className="w-40"
              />
              <Button
                onClick={() => createMilestone.mutate()}
                disabled={!newMilestoneName.trim() || createMilestone.isPending}
              >
                {createMilestone.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
              </Button>
            </CardContent>
          </Card>

          {milestones?.length ? (
            milestones.map((m) => (
              <Card key={m.id}>
                <CardContent className="flex items-center justify-between p-4">
                  <div className="flex items-center gap-3">
                    {m.status === "COMPLETED" ? (
                      <CheckCircle2 className="h-5 w-5 text-emerald-500" />
                    ) : m.status === "OVERDUE" ? (
                      <AlertCircle className="h-5 w-5 text-red-500" />
                    ) : (
                      <Circle className="h-5 w-5 text-slate-300" />
                    )}
                    <div>
                      <p className="text-sm font-medium text-slate-900">{m.name}</p>
                      {m.dueDate && (
                        <p className="text-xs text-slate-400">Due {new Date(m.dueDate).toLocaleDateString()}</p>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant={milestoneTone[m.status]}>{MILESTONE_STATUS_LABELS[m.status]}</Badge>
                    {m.status !== "COMPLETED" && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => completeMilestone.mutate(m.id)}
                        disabled={completeMilestone.isPending}
                      >
                        Mark Complete
                      </Button>
                    )}
                  </div>
                </CardContent>
              </Card>
            ))
          ) : (
            <p className="py-10 text-center text-sm text-slate-400">No milestones yet.</p>
          )}
        </div>
      )}
    </div>
  );
}

function StatCard({ label, value, tone }: { label: string; value: number; tone?: "red" | "emerald" }) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className={`text-2xl font-semibold ${tone === "red" ? "text-red-600" : tone === "emerald" ? "text-emerald-600" : "text-slate-900"}`}>
          {value}
        </p>
        <p className="text-xs text-slate-400">{label}</p>
      </CardContent>
    </Card>
  );
}
