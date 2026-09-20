"use client";

import { use, useEffect, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, ArrowLeft, ShieldCheck, ShieldAlert } from "lucide-react";

import { incidentsApi, INCIDENT_STATUS_LABELS, type IncidentStatus } from "@/lib/api/incidents";
import { rcaApi, RCA_STATUS_LABELS, type RCAStatus } from "@/lib/api/rca";
import { severityVariant } from "@/lib/ticket-display";
import { ApiError } from "@/lib/api/client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";

const statusTone: Record<string, "slate" | "blue" | "amber" | "emerald" | "red"> = {
  OPEN: "red", INVESTIGATING: "amber", MITIGATED: "blue", RESOLVED: "emerald", CLOSED: "slate",
};
const rcaTone: Record<string, "slate" | "amber" | "emerald"> = {
  DRAFT: "slate", IN_REVIEW: "amber", COMPLETED: "emerald",
};

const RCA_FIELDS: { key: keyof RCAFormState; label: string; multiline?: boolean }[] = [
  { key: "incidentSummary", label: "Incident Summary", multiline: true },
  { key: "businessImpact", label: "Business Impact", multiline: true },
  { key: "rootCause", label: "Root Cause", multiline: true },
  { key: "contributingFactors", label: "Contributing Factors", multiline: true },
  { key: "timeline", label: "Timeline", multiline: true },
  { key: "immediateFix", label: "Immediate Fix", multiline: true },
  { key: "permanentFix", label: "Permanent Fix", multiline: true },
  { key: "preventiveAction", label: "Preventive Action", multiline: true },
  { key: "deploymentReference", label: "Deployment Reference" },
];

interface RCAFormState {
  incidentSummary: string;
  businessImpact: string;
  rootCause: string;
  contributingFactors: string;
  timeline: string;
  immediateFix: string;
  permanentFix: string;
  preventiveAction: string;
  deploymentReference: string;
}

const emptyForm: RCAFormState = {
  incidentSummary: "", businessImpact: "", rootCause: "", contributingFactors: "",
  timeline: "", immediateFix: "", permanentFix: "", preventiveAction: "", deploymentReference: "",
};

export default function IncidentDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const queryClient = useQueryClient();
  const [statusError, setStatusError] = useState<string | null>(null);
  const [form, setForm] = useState<RCAFormState>(emptyForm);
  const [saved, setSaved] = useState(false);

  const { data: incident, isLoading } = useQuery({ queryKey: ["incident", id], queryFn: () => incidentsApi.get(id) });
  const { data: rca } = useQuery({ queryKey: ["rca", id], queryFn: () => rcaApi.get(id) });

  useEffect(() => {
    if (rca) {
      setForm({
        incidentSummary: rca.incidentSummary, businessImpact: rca.businessImpact, rootCause: rca.rootCause,
        contributingFactors: rca.contributingFactors, timeline: rca.timeline, immediateFix: rca.immediateFix,
        permanentFix: rca.permanentFix, preventiveAction: rca.preventiveAction, deploymentReference: rca.deploymentReference,
      });
    }
  }, [rca]);

  const statusMutation = useMutation({
    mutationFn: (status: IncidentStatus) => incidentsApi.changeStatus(id, status),
    onSuccess: () => {
      setStatusError(null);
      queryClient.invalidateQueries({ queryKey: ["incident", id] });
    },
    onError: (err) => setStatusError(err instanceof ApiError ? err.message : "Failed to change status"),
  });

  const forceCloseMutation = useMutation({
    mutationFn: () => incidentsApi.changeStatus(id, "CLOSED", true),
    onSuccess: () => {
      setStatusError(null);
      queryClient.invalidateQueries({ queryKey: ["incident", id] });
    },
  });

  const saveRcaMutation = useMutation({
    mutationFn: () => rcaApi.update(id, form),
    onSuccess: () => {
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
      queryClient.invalidateQueries({ queryKey: ["rca", id] });
    },
  });

  const [rcaStatusError, setRcaStatusError] = useState<string | null>(null);
  const rcaStatusMutation = useMutation({
    mutationFn: (status: RCAStatus) => rcaApi.changeStatus(id, status),
    onSuccess: () => {
      setRcaStatusError(null);
      queryClient.invalidateQueries({ queryKey: ["rca", id] });
    },
    onError: (err) => setRcaStatusError(err instanceof ApiError ? err.message : "Failed to update RCA status"),
  });

  if (isLoading || !incident) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-4xl px-8 py-8">
      <Link href="/incidents" className="mb-4 flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700">
        <ArrowLeft className="h-4 w-4" /> Back to Incidents
      </Link>

      <div className="mb-6">
        <div className="mb-2 flex items-center gap-2">
          <span className="font-mono text-sm text-slate-400">{incident.incidentNumber}</span>
          <Badge variant={severityVariant(incident.severity)}>{incident.severity}</Badge>
          <Badge variant={statusTone[incident.status]}>{INCIDENT_STATUS_LABELS[incident.status]}</Badge>
          {incident.rcaRequired && (
            <span className="flex items-center gap-1 text-xs text-purple-600">
              {rca?.status === "COMPLETED" ? <ShieldCheck className="h-3.5 w-3.5" /> : <ShieldAlert className="h-3.5 w-3.5" />}
              RCA required
            </span>
          )}
        </div>
        <h1 className="text-2xl font-semibold text-slate-900">{incident.title}</h1>
        {incident.affectedSystem && (
          <p className="mt-1 text-sm text-slate-500">Affected: {incident.affectedSystem}</p>
        )}
        {incident.impact && <p className="text-sm text-slate-500">{incident.impact}</p>}
      </div>

      <Card className="mb-6">
        <CardContent className="p-4">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs font-medium text-slate-500">Status:</span>
            {(["OPEN", "INVESTIGATING", "MITIGATED", "RESOLVED", "CLOSED"] as IncidentStatus[]).map((s) => (
              <Button
                key={s}
                size="sm"
                variant={incident.status === s ? "default" : "outline"}
                onClick={() => statusMutation.mutate(s)}
                disabled={statusMutation.isPending || incident.status === s}
              >
                {INCIDENT_STATUS_LABELS[s]}
              </Button>
            ))}
          </div>
          {statusError && (
            <div className="mt-2 flex items-center justify-between rounded-md bg-red-50 px-3 py-2">
              <p className="text-xs text-red-700">{statusError}</p>
              <Button size="sm" variant="destructive" onClick={() => forceCloseMutation.mutate()}>
                Admin Override &amp; Close
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-5">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-slate-900">Root Cause Analysis</h2>
            {rca && <Badge variant={rcaTone[rca.status]}>{RCA_STATUS_LABELS[rca.status]}</Badge>}
          </div>

          <div className="space-y-4">
            {RCA_FIELDS.map((f) => (
              <div key={f.key} className="space-y-1.5">
                <Label htmlFor={f.key}>{f.label}</Label>
                {f.multiline ? (
                  <textarea
                    id={f.key}
                    rows={2}
                    className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
                    value={form[f.key]}
                    onChange={(e) => setForm((prev) => ({ ...prev, [f.key]: e.target.value }))}
                  />
                ) : (
                  <input
                    id={f.key}
                    className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
                    value={form[f.key]}
                    onChange={(e) => setForm((prev) => ({ ...prev, [f.key]: e.target.value }))}
                  />
                )}
              </div>
            ))}
          </div>

          <div className="mt-5 flex items-center gap-3 border-t border-slate-100 pt-4">
            <Button onClick={() => saveRcaMutation.mutate()} disabled={saveRcaMutation.isPending}>
              {saveRcaMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : "Save"}
            </Button>
            {saved && <span className="text-xs text-emerald-600">Saved.</span>}

            <div className="ml-auto flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => rcaStatusMutation.mutate("IN_REVIEW")}
                disabled={rcaStatusMutation.isPending || rca?.status === "IN_REVIEW"}
              >
                Submit for Review
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => rcaStatusMutation.mutate("COMPLETED")}
                disabled={rcaStatusMutation.isPending || rca?.status === "COMPLETED"}
              >
                Mark Complete
              </Button>
            </div>
          </div>
          {rcaStatusError && <p className="mt-2 text-xs text-red-600">{rcaStatusError}</p>}
        </CardContent>
      </Card>
    </div>
  );
}
