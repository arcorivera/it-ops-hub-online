"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Plus, X, AlertTriangle } from "lucide-react";

import { incidentsApi, INCIDENT_STATUS_LABELS } from "@/lib/api/incidents";
import { severityVariant } from "@/lib/ticket-display";
import { ApiError } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const schema = z.object({
  title: z.string().min(1, "Required"),
  severity: z.enum(["S1", "S2", "S3", "S4"]),
  impact: z.string().optional(),
  affectedSystem: z.string().optional(),
  environment: z.enum(["DEV", "UAT", "PRE-PROD", "PRODUCTION"]),
});
type FormData = z.infer<typeof schema>;

const statusTone: Record<string, "slate" | "blue" | "amber" | "emerald" | "red"> = {
  OPEN: "red", INVESTIGATING: "amber", MITIGATED: "blue", RESOLVED: "emerald", CLOSED: "slate",
};

export default function IncidentsPage() {
  const [showForm, setShowForm] = useState(false);
  const queryClient = useQueryClient();

  const { data: incidents, isLoading } = useQuery({ queryKey: ["incidents"], queryFn: () => incidentsApi.list() });

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors },
  } = useForm<FormData>({ resolver: zodResolver(schema), defaultValues: { severity: "S3", environment: "PRODUCTION" } });

  const createMutation = useMutation({
    mutationFn: incidentsApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["incidents"] });
      reset({ severity: "S3", environment: "PRODUCTION" });
      setShowForm(false);
    },
    onError: (err) => {
      if (err instanceof ApiError) setError("root", { message: err.message });
    },
  });

  return (
    <div className="mx-auto max-w-5xl px-8 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">Incidents</h1>
          <p className="mt-1 text-sm text-slate-500">S1/S2 incidents require a completed RCA before closure.</p>
        </div>
        <Button onClick={() => setShowForm((s) => !s)}>
          {showForm ? <X className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
          {showForm ? "Cancel" : "Report Incident"}
        </Button>
      </div>

      {showForm && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Report Incident</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="space-y-4" onSubmit={handleSubmit((data) => createMutation.mutate(data))}>
              <div className="space-y-1.5">
                <Label htmlFor="title">Title</Label>
                <Input id="title" {...register("title")} />
                {errors.title && <p className="text-xs text-red-600">{errors.title.message}</p>}
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <Label htmlFor="severity">Severity</Label>
                  <select id="severity" className="h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm" {...register("severity")}>
                    <option value="S1">S1 Critical</option>
                    <option value="S2">S2 High</option>
                    <option value="S3">S3 Medium</option>
                    <option value="S4">S4 Low</option>
                  </select>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="environment">Environment</Label>
                  <select id="environment" className="h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm" {...register("environment")}>
                    <option value="DEV">DEV</option>
                    <option value="UAT">UAT</option>
                    <option value="PRE-PROD">PRE-PROD</option>
                    <option value="PRODUCTION">PRODUCTION</option>
                  </select>
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="affectedSystem">Affected System</Label>
                <Input id="affectedSystem" {...register("affectedSystem")} />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="impact">Impact</Label>
                <Input id="impact" {...register("impact")} />
              </div>
              {errors.root && <p className="text-xs text-red-600">{errors.root.message}</p>}
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : "Report Incident"}
              </Button>
            </form>
          </CardContent>
        </Card>
      )}

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : incidents && incidents.length > 0 ? (
        <Card>
          <CardContent className="p-0">
            {incidents.map((inc) => (
              <Link
                key={inc.id}
                href={`/incidents/${inc.id}`}
                className="flex items-center justify-between border-b border-slate-100 px-5 py-3 last:border-0 hover:bg-slate-50"
              >
                <div className="flex items-center gap-2">
                  <span className="font-mono text-xs text-slate-400">{inc.incidentNumber}</span>
                  <span className="text-sm text-slate-800">{inc.title}</span>
                  {inc.rcaRequired && <span className="text-xs text-purple-500">RCA required</span>}
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant={severityVariant(inc.severity)}>{inc.severity}</Badge>
                  <Badge variant={statusTone[inc.status]}>{INCIDENT_STATUS_LABELS[inc.status]}</Badge>
                </div>
              </Link>
            ))}
          </CardContent>
        </Card>
      ) : (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-slate-200 py-16 text-center text-sm text-slate-400">
          <AlertTriangle className="h-6 w-6" />
          No incidents reported.
        </div>
      )}
    </div>
  );
}
