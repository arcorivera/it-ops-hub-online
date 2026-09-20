"use client";

import { useState, useRef } from "react";
import Link from "next/link";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Loader2, Plus, Search, Upload } from "lucide-react";

import { ticketsApi, SEVERITY_LABELS, STATUS_LABELS, type Severity, type TicketStatus } from "@/lib/api/tickets";
import { severityVariant, priorityVariant, statusVariant } from "@/lib/ticket-display";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";

const SEVERITIES: Severity[] = ["S1", "S2", "S3", "S4"];
const STATUSES: TicketStatus[] = [
  "NEW", "ACKNOWLEDGED", "ASSIGNED", "IN_PROGRESS", "PENDING", "FOR_UAT", "UAT_FAILED",
  "UAT_PASSED", "FOR_PRE_PROD", "PRE_PROD_FAILED", "PRE_PROD_PASSED", "FOR_PRODUCTION",
  "PRODUCTION_FAILED", "RESOLVED", "CLOSED", "CANCELLED",
];

export default function TicketsPage() {
  const [search, setSearch] = useState("");
  const [severityFilter, setSeverityFilter] = useState<Severity | "">("");
  const [statusFilter, setStatusFilter] = useState<TicketStatus | "">("");
  const [page, setPage] = useState(1);
  const [importResult, setImportResult] = useState<{ imported: number; skipped: number; failed: { row: number; title: string; message: string }[] } | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const queryClient = useQueryClient();

  const importMutation = useMutation({
    mutationFn: (file: File) => ticketsApi.importCsv(file),
    onSuccess: (result) => {
      setImportResult(result);
      queryClient.invalidateQueries({ queryKey: ["tickets"] });
    },
  });

  const { data, isLoading } = useQuery({
    queryKey: ["tickets", { search, severityFilter, statusFilter, page }],
    queryFn: () =>
      ticketsApi.list({
        search: search || undefined,
        severity: severityFilter ? [severityFilter] : undefined,
        status: statusFilter ? [statusFilter] : undefined,
        page,
        pageSize: 25,
      }),
  });

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.pageSize)) : 1;

  return (
    <div className="mx-auto max-w-6xl px-8 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">Tickets</h1>
          <p className="mt-1 text-sm text-slate-500">
            {data ? `${data.total} ticket${data.total === 1 ? "" : "s"}` : "Loading…"}
          </p>
        </div>
        <div className="flex gap-2">
          <input
            ref={fileInputRef}
            type="file"
            accept=".csv"
            className="hidden"
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) importMutation.mutate(f);
              e.target.value = "";
            }}
          />
          <Button variant="outline" onClick={() => fileInputRef.current?.click()} disabled={importMutation.isPending}>
            {importMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
            Import CSV
          </Button>
          <Link href="/tickets/new">
            <Button>
              <Plus className="h-4 w-4" />
              Create Ticket
            </Button>
          </Link>
        </div>
      </div>

      {importResult && (
        <Card className="mb-4 border-indigo-200">
          <CardContent className="p-4 text-sm">
            <p className="font-medium text-slate-800">
              Imported {importResult.imported}, skipped {importResult.skipped}
              {importResult.failed.length > 0 && `, ${importResult.failed.length} row(s) had errors`}
            </p>
            {importResult.failed.length > 0 && (
              <ul className="mt-2 space-y-1 text-xs text-red-600">
                {importResult.failed.map((f, i) => (
                  <li key={i}>
                    Row {f.row}{f.title ? ` (${f.title})` : ""}: {f.message}
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      )}

      <div className="mb-4 flex flex-wrap items-center gap-3">
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <Input
            placeholder="Search title, description, ticket #..."
            className="w-72 pl-9"
            value={search}
            onChange={(e) => {
              setPage(1);
              setSearch(e.target.value);
            }}
          />
        </div>

        <select
          className="h-10 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700"
          value={severityFilter}
          onChange={(e) => {
            setPage(1);
            setSeverityFilter(e.target.value as Severity | "");
          }}
        >
          <option value="">All Severities</option>
          {SEVERITIES.map((s) => (
            <option key={s} value={s}>
              {SEVERITY_LABELS[s]}
            </option>
          ))}
        </select>

        <select
          className="h-10 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700"
          value={statusFilter}
          onChange={(e) => {
            setPage(1);
            setStatusFilter(e.target.value as TicketStatus | "");
          }}
        >
          <option value="">All Statuses</option>
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {STATUS_LABELS[s]}
            </option>
          ))}
        </select>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="flex justify-center py-10">
              <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
            </div>
          ) : data && data.tickets.length > 0 ? (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 text-left text-xs font-semibold uppercase tracking-wider text-slate-400">
                  <th className="px-6 py-3">Ticket</th>
                  <th className="px-6 py-3">Severity</th>
                  <th className="px-6 py-3">Priority</th>
                  <th className="px-6 py-3">Status</th>
                  <th className="px-6 py-3">Environment</th>
                  <th className="px-6 py-3">Created</th>
                </tr>
              </thead>
              <tbody>
                {data.tickets.map((t) => (
                  <tr
                    key={t.id}
                    className="cursor-pointer border-b border-slate-100 last:border-0 hover:bg-slate-50"
                  >
                    <td className="px-6 py-3">
                      <Link href={`/tickets/${t.id}`} className="block">
                        <span className="font-mono text-xs text-slate-400">{t.ticketNumber}</span>
                        <p className="font-medium text-slate-900">{t.title}</p>
                      </Link>
                    </td>
                    <td className="px-6 py-3">
                      <Badge variant={severityVariant(t.severity)}>{t.severity}</Badge>
                    </td>
                    <td className="px-6 py-3">
                      <Badge variant={priorityVariant(t.priority)}>{t.priority}</Badge>
                    </td>
                    <td className="px-6 py-3">
                      <Badge variant={statusVariant(t.status)}>{STATUS_LABELS[t.status]}</Badge>
                    </td>
                    <td className="px-6 py-3 text-slate-600">{t.environment}</td>
                    <td className="px-6 py-3 text-slate-500">
                      {new Date(t.createdAt).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <div className="px-6 py-12 text-center text-sm text-slate-500">
              No tickets match these filters.
            </div>
          )}
        </CardContent>
      </Card>

      {data && data.total > data.pageSize && (
        <div className="mt-4 flex items-center justify-between">
          <p className="text-sm text-slate-500">
            Page {page} of {totalPages}
          </p>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
