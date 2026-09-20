"use client";

import { use, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Loader2,
  Paperclip,
  Send,
  ArrowLeft,
  Clock,
  UserCircle,
  History as HistoryIcon,
  Trash2,
  Bell,
  ArrowUpCircle,
} from "lucide-react";

import { ticketsApi, STATUS_LABELS, type TicketStatus, type Severity, type Priority } from "@/lib/api/tickets";
import { usersApi } from "@/lib/api/users";
import { severityVariant, priorityVariant, statusVariant } from "@/lib/ticket-display";
import { useCurrentUser } from "@/hooks/use-current-user";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ApiError } from "@/lib/api/client";
import { slaApi, slaStatusVariant, SLA_STATUS_LABELS } from "@/lib/api/sla";
import { followupApi, escalationApi } from "@/lib/api/followup-escalation";
import { testingApi, type TestCaseStatus, type DeploymentStage } from "@/lib/api/testing";

const TABS = ["Overview", "Comments", "Activity", "Attachments", "Testing"] as const;
type Tab = (typeof TABS)[number];

export default function TicketDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<Tab>("Overview");
  const [commentText, setCommentText] = useState("");
  const [statusError, setStatusError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const { data: user } = useCurrentUser();

  const { data: ticket, isLoading } = useQuery({
    queryKey: ["ticket", id],
    queryFn: () => ticketsApi.get(id),
  });

  const { data: allUsers } = useQuery({ queryKey: ["users"], queryFn: usersApi.list });

  const { data: slaStatus } = useQuery({
    queryKey: ["ticket-sla", id],
    queryFn: () => slaApi.get(id),
    refetchInterval: 30_000, // live countdown reflects real backend calculation
  });

  const followUpMutation = useMutation({
    mutationFn: () => followupApi.trigger(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["ticket-followups", id] }),
  });

  const escalateMutation = useMutation({
    mutationFn: () => escalationApi.trigger(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["ticket-escalations", id] }),
  });

  const [newTestDesc, setNewTestDesc] = useState("");
  const [deployVersion, setDeployVersion] = useState("");
  const [deployRef, setDeployRef] = useState("");

  const { data: testCases } = useQuery({
    queryKey: ["ticket-testcases", id],
    queryFn: () => testingApi.listTestCases(id),
    enabled: tab === "Testing",
  });
  const { data: deployments } = useQuery({
    queryKey: ["ticket-deployments", id],
    queryFn: () => testingApi.listDeployments(id),
    enabled: tab === "Testing",
  });

  const createTestCaseMutation = useMutation({
    mutationFn: () => testingApi.createTestCase(id, { description: newTestDesc }),
    onSuccess: () => {
      setNewTestDesc("");
      queryClient.invalidateQueries({ queryKey: ["ticket-testcases", id] });
    },
  });

  const executeTestMutation = useMutation({
    mutationFn: ({ caseId, status }: { caseId: string; status: TestCaseStatus }) =>
      testingApi.execute(id, caseId, { status }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["ticket-testcases", id] }),
  });

  const createDeploymentMutation = useMutation({
    mutationFn: (stage: DeploymentStage) =>
      testingApi.createDeployment(id, { stage, version: deployVersion, deploymentReference: deployRef }),
    onSuccess: () => {
      setDeployVersion("");
      setDeployRef("");
      queryClient.invalidateQueries({ queryKey: ["ticket-deployments", id] });
    },
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["ticket", id] });
    queryClient.invalidateQueries({ queryKey: ["ticket-history", id] });
  };

  const assignMutation = useMutation({
    mutationFn: (assigneeId: string | null) => ticketsApi.assign(id, assigneeId),
    onSuccess: invalidate,
  });

  const statusMutation = useMutation({
    mutationFn: (status: TicketStatus) => ticketsApi.changeStatus(id, status),
    onSuccess: () => {
      setStatusError(null);
      invalidate();
    },
    onError: (err) => {
      setStatusError(err instanceof ApiError ? err.message : "Failed to change status");
    },
  });

  const severityMutation = useMutation({
    mutationFn: (severity: Severity) => ticketsApi.changeSeverity(id, severity),
    onSuccess: invalidate,
  });

  const priorityMutation = useMutation({
    mutationFn: (priority: Priority) => ticketsApi.changePriority(id, priority),
    onSuccess: invalidate,
  });

  const commentMutation = useMutation({
    mutationFn: (content: string) => ticketsApi.addComment(id, content),
    onSuccess: () => {
      setCommentText("");
      queryClient.invalidateQueries({ queryKey: ["ticket-comments", id] });
      queryClient.invalidateQueries({ queryKey: ["ticket-history", id] });
    },
  });

  const deleteCommentMutation = useMutation({
    mutationFn: (commentId: string) => ticketsApi.deleteComment(id, commentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["ticket-comments", id] }),
  });

  const uploadMutation = useMutation({
    mutationFn: (file: File) => ticketsApi.uploadAttachment(id, file),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ticket-attachments", id] });
      queryClient.invalidateQueries({ queryKey: ["ticket-history", id] });
    },
  });

  const { data: comments } = useQuery({
    queryKey: ["ticket-comments", id],
    queryFn: () => ticketsApi.listComments(id),
    enabled: tab === "Comments",
  });

  const { data: history } = useQuery({
    queryKey: ["ticket-history", id],
    queryFn: () => ticketsApi.listHistory(id),
    enabled: tab === "Activity",
  });

  const { data: attachments } = useQuery({
    queryKey: ["ticket-attachments", id],
    queryFn: () => ticketsApi.listAttachments(id),
    enabled: tab === "Attachments" || tab === "Overview",
  });

  if (isLoading || !ticket) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
      </div>
    );
  }

  const userName = (userId: string | null) => {
    if (!userId) return "Unassigned";
    const u = allUsers?.find((u) => u.id === userId);
    return u?.fullName ?? userId;
  };

  return (
    <div className="mx-auto max-w-5xl px-8 py-8">
      <button
        onClick={() => router.push("/tickets")}
        className="mb-4 flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700"
      >
        <ArrowLeft className="h-4 w-4" /> Back to Tickets
      </button>

      <div className="mb-6">
        <div className="mb-2 flex flex-wrap items-center gap-2">
          <span className="font-mono text-sm text-slate-400">{ticket.ticketNumber}</span>
          <Badge variant={severityVariant(ticket.severity)}>{ticket.severity}</Badge>
          <Badge variant={priorityVariant(ticket.priority)}>{ticket.priority}</Badge>
          <Badge variant={statusVariant(ticket.status)}>{STATUS_LABELS[ticket.status]}</Badge>
          <Badge variant="slate">{ticket.environment}</Badge>
        </div>
        <h1 className="text-2xl font-semibold text-slate-900">{ticket.title}</h1>
      </div>

      <div className="grid grid-cols-3 gap-6">
        <div className="col-span-2">
          <div className="mb-4 flex gap-1 border-b border-slate-200">
            {TABS.map((t) => (
              <button
                key={t}
                onClick={() => setTab(t)}
                className={`px-4 py-2 text-sm font-medium transition-colors ${
                  tab === t
                    ? "border-b-2 border-indigo-600 text-indigo-600"
                    : "text-slate-500 hover:text-slate-700"
                }`}
              >
                {t}
              </button>
            ))}
          </div>

          {tab === "Overview" && (
            <Card>
              <CardContent className="space-y-4 p-6">
                <div>
                  <h3 className="mb-1 text-xs font-semibold uppercase tracking-wider text-slate-400">
                    Description
                  </h3>
                  <p className="whitespace-pre-wrap text-sm text-slate-700">
                    {ticket.description || "No description provided."}
                  </p>
                </div>
                <div className="grid grid-cols-2 gap-4 border-t border-slate-100 pt-4 text-sm">
                  <div>
                    <p className="text-xs text-slate-400">Requester</p>
                    <p className="text-slate-700">{userName(ticket.requesterId)}</p>
                  </div>
                  <div>
                    <p className="text-xs text-slate-400">Assignee</p>
                    <p className="text-slate-700">{userName(ticket.assigneeId)}</p>
                  </div>
                  <div>
                    <p className="text-xs text-slate-400">Created</p>
                    <p className="text-slate-700">{new Date(ticket.createdAt).toLocaleString()}</p>
                  </div>
                  <div>
                    <p className="text-xs text-slate-400">Last Updated</p>
                    <p className="text-slate-700">{new Date(ticket.updatedAt).toLocaleString()}</p>
                  </div>
                </div>
                {attachments && attachments.length > 0 && (
                  <div className="border-t border-slate-100 pt-4">
                    <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-400">
                      Attachments ({attachments.length})
                    </h3>
                    <div className="space-y-1">
                      {attachments.map((a) => (
                        <a
                          key={a.id}
                          href={ticketsApi.downloadAttachmentUrl(id, a.id)}
                          className="flex items-center gap-2 text-sm text-indigo-600 hover:underline"
                        >
                          <Paperclip className="h-3.5 w-3.5" />
                          {a.fileName}
                        </a>
                      ))}
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>
          )}

          {tab === "Comments" && (
            <div className="space-y-4">
              <Card>
                <CardContent className="p-4">
                  <div className="flex gap-2">
                    <textarea
                      rows={2}
                      placeholder="Add a comment..."
                      className="flex-1 rounded-md border border-slate-300 bg-white px-3 py-2 text-sm placeholder:text-slate-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
                      value={commentText}
                      onChange={(e) => setCommentText(e.target.value)}
                    />
                    <Button
                      size="icon"
                      disabled={!commentText.trim() || commentMutation.isPending}
                      onClick={() => commentMutation.mutate(commentText)}
                    >
                      {commentMutation.isPending ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Send className="h-4 w-4" />
                      )}
                    </Button>
                  </div>
                </CardContent>
              </Card>

              {comments?.length ? (
                comments.map((c) => (
                  <Card key={c.id}>
                    <CardContent className="p-4">
                      <div className="mb-1 flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <UserCircle className="h-5 w-5 text-slate-400" />
                          <span className="text-sm font-medium text-slate-900">{userName(c.userId)}</span>
                          <span className="text-xs text-slate-400">
                            {new Date(c.createdAt).toLocaleString()}
                          </span>
                        </div>
                        {(c.userId === user?.id || user?.roles.includes("ADMIN")) && (
                          <button
                            onClick={() => deleteCommentMutation.mutate(c.id)}
                            className="text-slate-400 hover:text-red-600"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </button>
                        )}
                      </div>
                      <p className="whitespace-pre-wrap text-sm text-slate-700">{c.content}</p>
                    </CardContent>
                  </Card>
                ))
              ) : (
                <p className="py-6 text-center text-sm text-slate-400">No comments yet.</p>
              )}
            </div>
          )}

          {tab === "Activity" && (
            <Card>
              <CardContent className="p-6">
                {history?.length ? (
                  <ol className="space-y-4">
                    {history.map((h) => (
                      <li key={h.id} className="flex gap-3">
                        <HistoryIcon className="mt-0.5 h-4 w-4 flex-shrink-0 text-slate-400" />
                        <div>
                          <p className="text-sm text-slate-700">
                            <span className="font-medium">{userName(h.userId)}</span>{" "}
                            {describeEvent(h)}
                          </p>
                          <p className="text-xs text-slate-400">{new Date(h.createdAt).toLocaleString()}</p>
                        </div>
                      </li>
                    ))}
                  </ol>
                ) : (
                  <p className="py-6 text-center text-sm text-slate-400">No activity recorded.</p>
                )}
              </CardContent>
            </Card>
          )}

          {tab === "Attachments" && (
            <div className="space-y-4">
              <Card>
                <CardContent className="p-4">
                  <input
                    ref={fileInputRef}
                    type="file"
                    className="hidden"
                    onChange={(e) => {
                      const f = e.target.files?.[0];
                      if (f) uploadMutation.mutate(f);
                      e.target.value = "";
                    }}
                  />
                  <Button
                    variant="outline"
                    onClick={() => fileInputRef.current?.click()}
                    disabled={uploadMutation.isPending}
                  >
                    {uploadMutation.isPending ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Paperclip className="h-4 w-4" />
                    )}
                    Upload File
                  </Button>
                  <p className="mt-2 text-xs text-slate-400">
                    Allowed: png, jpg, jpeg, pdf, docx, xlsx, txt, zip (max 25MB)
                  </p>
                  {uploadMutation.isError && (
                    <p className="mt-2 text-xs text-red-600">
                      {uploadMutation.error instanceof Error ? uploadMutation.error.message : "Upload failed"}
                    </p>
                  )}
                </CardContent>
              </Card>

              {attachments?.length ? (
                <Card>
                  <CardContent className="divide-y divide-slate-100 p-0">
                    {attachments.map((a) => (
                      <a
                        key={a.id}
                        href={ticketsApi.downloadAttachmentUrl(id, a.id)}
                        className="flex items-center justify-between px-4 py-3 hover:bg-slate-50"
                      >
                        <div className="flex items-center gap-2">
                          <Paperclip className="h-4 w-4 text-slate-400" />
                          <span className="text-sm text-slate-700">{a.fileName}</span>
                        </div>
                        <span className="text-xs text-slate-400">{(a.sizeBytes / 1024).toFixed(1)} KB</span>
                      </a>
                    ))}
                  </CardContent>
                </Card>
              ) : (
                <p className="py-6 text-center text-sm text-slate-400">No attachments yet.</p>
              )}
            </div>
          )}

          {tab === "Testing" && (
            <div className="space-y-6">
              <Card>
                <CardContent className="p-4">
                  <h3 className="mb-3 text-sm font-semibold text-slate-700">UAT Test Cases</h3>
                  <div className="mb-3 flex gap-2">
                    <input
                      placeholder="New test case description..."
                      className="flex-1 rounded-md border border-slate-300 px-3 py-2 text-sm placeholder:text-slate-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
                      value={newTestDesc}
                      onChange={(e) => setNewTestDesc(e.target.value)}
                    />
                    <Button
                      size="sm"
                      onClick={() => createTestCaseMutation.mutate()}
                      disabled={!newTestDesc.trim() || createTestCaseMutation.isPending}
                    >
                      {createTestCaseMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : "Add"}
                    </Button>
                  </div>

                  {testCases?.length ? (
                    <div className="space-y-2">
                      {testCases.map((tc) => (
                        <div key={tc.id} className="rounded-md border border-slate-100 p-3">
                          <div className="mb-2 flex items-center justify-between">
                            <p className="text-sm text-slate-800">{tc.description}</p>
                            <TestStatusBadge status={tc.status} />
                          </div>
                          {tc.actualResult && (
                            <p className="mb-2 text-xs text-slate-500">Result: {tc.actualResult}</p>
                          )}
                          <div className="flex gap-1.5">
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-7 text-xs"
                              onClick={() => executeTestMutation.mutate({ caseId: tc.id, status: "PASS" })}
                              disabled={executeTestMutation.isPending}
                            >
                              Pass
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-7 text-xs"
                              onClick={() => executeTestMutation.mutate({ caseId: tc.id, status: "FAIL" })}
                              disabled={executeTestMutation.isPending}
                            >
                              Fail
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-7 text-xs"
                              onClick={() => executeTestMutation.mutate({ caseId: tc.id, status: "BLOCKED" })}
                              disabled={executeTestMutation.isPending}
                            >
                              Block
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="py-4 text-center text-xs text-slate-400">
                      No test cases yet — ticket can still move to UAT_PASSED freely until one is added.
                    </p>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardContent className="p-4">
                  <h3 className="mb-3 text-sm font-semibold text-slate-700">Deployments</h3>
                  <div className="mb-3 grid grid-cols-2 gap-2">
                    <input
                      placeholder="Version (e.g. 1.4.0)"
                      className="rounded-md border border-slate-300 px-3 py-2 text-sm placeholder:text-slate-400"
                      value={deployVersion}
                      onChange={(e) => setDeployVersion(e.target.value)}
                    />
                    <input
                      placeholder="Deployment reference"
                      className="rounded-md border border-slate-300 px-3 py-2 text-sm placeholder:text-slate-400"
                      value={deployRef}
                      onChange={(e) => setDeployRef(e.target.value)}
                    />
                  </div>
                  <div className="flex gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => createDeploymentMutation.mutate("PRE_PROD")}
                      disabled={!deployVersion.trim() || createDeploymentMutation.isPending}
                    >
                      Record Pre-Prod Deployment
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => createDeploymentMutation.mutate("PRODUCTION")}
                      disabled={!deployVersion.trim() || createDeploymentMutation.isPending}
                    >
                      Record Production Deployment
                    </Button>
                  </div>

                  {deployments?.length ? (
                    <div className="mt-4 space-y-2">
                      {deployments.map((d) => (
                        <div key={d.id} className="flex items-center justify-between rounded-md border border-slate-100 p-3 text-sm">
                          <div>
                            <span className="font-medium text-slate-800">{d.stage}</span>{" "}
                            <span className="text-slate-500">v{d.version}</span>
                            {d.deploymentReference && (
                              <span className="ml-2 font-mono text-xs text-slate-400">{d.deploymentReference}</span>
                            )}
                          </div>
                          <ValidationBadge result={d.validationResult} />
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="mt-4 py-2 text-center text-xs text-slate-400">No deployments recorded yet.</p>
                  )}
                </CardContent>
              </Card>
            </div>
          )}
        </div>

        <div className="space-y-4">
          <Card>
            <CardContent className="space-y-4 p-4">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400">Actions</h3>

              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-500">Assignee</label>
                <select
                  className="h-9 w-full rounded-md border border-slate-300 bg-white px-2 text-sm"
                  value={ticket.assigneeId ?? ""}
                  onChange={(e) => assignMutation.mutate(e.target.value || null)}
                  disabled={assignMutation.isPending}
                >
                  <option value="">Unassigned</option>
                  {allUsers?.map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.fullName}
                    </option>
                  ))}
                </select>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-500">Status</label>
                <select
                  className="h-9 w-full rounded-md border border-slate-300 bg-white px-2 text-sm"
                  value={ticket.status}
                  onChange={(e) => statusMutation.mutate(e.target.value as TicketStatus)}
                  disabled={statusMutation.isPending}
                >
                  {Object.entries(STATUS_LABELS).map(([value, label]) => (
                    <option key={value} value={value}>
                      {label}
                    </option>
                  ))}
                </select>
                {statusError && <p className="text-xs text-red-600">{statusError}</p>}
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-500">Severity</label>
                <select
                  className="h-9 w-full rounded-md border border-slate-300 bg-white px-2 text-sm"
                  value={ticket.severity}
                  onChange={(e) => severityMutation.mutate(e.target.value as Severity)}
                  disabled={severityMutation.isPending}
                >
                  <option value="S1">S1 Critical</option>
                  <option value="S2">S2 High</option>
                  <option value="S3">S3 Medium</option>
                  <option value="S4">S4 Low</option>
                </select>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-500">Priority</label>
                <select
                  className="h-9 w-full rounded-md border border-slate-300 bg-white px-2 text-sm"
                  value={ticket.priority}
                  onChange={(e) => priorityMutation.mutate(e.target.value as Priority)}
                  disabled={priorityMutation.isPending}
                >
                  <option value="Critical">Critical</option>
                  <option value="High">High</option>
                  <option value="Medium">Medium</option>
                  <option value="Low">Low</option>
                </select>
              </div>
            </CardContent>
          </Card>

          {(ticket.responseDeadline || ticket.resolutionDeadline) && (
            <Card>
              <CardContent className="space-y-3 p-4">
                <div className="flex items-center justify-between">
                  <h3 className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider text-slate-400">
                    <Clock className="h-3.5 w-3.5" /> SLA
                  </h3>
                  {slaStatus && (
                    <Badge variant={slaStatusVariant(slaStatus.status)}>
                      {SLA_STATUS_LABELS[slaStatus.status]}
                    </Badge>
                  )}
                </div>

                {slaStatus && slaStatus.status !== "RESOLVED" && (
                  <>
                    <div>
                      <div className="mb-1 flex items-center justify-between text-xs text-slate-500">
                        <span>{slaStatus.percentage.toFixed(1)}% consumed</span>
                        <span>
                          {slaStatus.remainingMinutes >= 0
                            ? formatMinutes(slaStatus.remainingMinutes) + " remaining"
                            : formatMinutes(-slaStatus.remainingMinutes) + " over"}
                        </span>
                      </div>
                      <div className="h-2 w-full overflow-hidden rounded-full bg-slate-100">
                        <div
                          className={`h-full rounded-full ${slaBarColor(slaStatus.status)}`}
                          style={{ width: `${Math.min(100, slaStatus.percentage)}%` }}
                        />
                      </div>
                    </div>
                    <p className="text-xs text-slate-400">
                      {slaStatus.usesBusinessHours ? "Business hours" : "Wall clock"} ·{" "}
                      {slaStatus.isPaused ? "Paused" : `Deadline ${new Date(slaStatus.projectedDeadline).toLocaleString()}`}
                    </p>
                  </>
                )}

                <div className="flex gap-2 border-t border-slate-100 pt-3">
                  <Button
                    variant="outline"
                    size="sm"
                    className="flex-1"
                    onClick={() => followUpMutation.mutate()}
                    disabled={followUpMutation.isPending}
                  >
                    {followUpMutation.isPending ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Bell className="h-3.5 w-3.5" />
                    )}
                    Follow Up
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="flex-1"
                    onClick={() => escalateMutation.mutate()}
                    disabled={escalateMutation.isPending}
                  >
                    {escalateMutation.isPending ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <ArrowUpCircle className="h-3.5 w-3.5" />
                    )}
                    Escalate
                  </Button>
                </div>
                {followUpMutation.isSuccess && (
                  <p className="text-xs text-emerald-600">Follow-up recorded.</p>
                )}
                {escalateMutation.isSuccess && (
                  <p className="text-xs text-emerald-600">Escalation recorded.</p>
                )}
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}

function TestStatusBadge({ status }: { status: string }) {
  const variant =
    status === "PASS" ? "emerald" : status === "FAIL" ? "red" : status === "BLOCKED" ? "orange" : "slate";
  return <Badge variant={variant}>{status.replace("_", " ")}</Badge>;
}

function ValidationBadge({ result }: { result: string }) {
  const variant = result === "PASSED" ? "emerald" : result === "FAILED" ? "red" : "slate";
  return <Badge variant={variant}>{result}</Badge>;
}

function formatMinutes(mins: number): string {
  if (mins < 60) return `${mins}m`;
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  if (h < 24) return m > 0 ? `${h}h ${m}m` : `${h}h`;
  const d = Math.floor(h / 24);
  const remH = h % 24;
  return remH > 0 ? `${d}d ${remH}h` : `${d}d`;
}

function slaBarColor(status: string): string {
  switch (status) {
    case "WITHIN_SLA": return "bg-emerald-500";
    case "WARNING": return "bg-amber-500";
    case "CRITICAL": return "bg-orange-500";
    case "BREACHED": return "bg-red-600";
    case "PAUSED": return "bg-purple-400";
    default: return "bg-slate-300";
  }
}

function describeEvent(h: { eventType: string; field: string; oldValue: string; newValue: string; note: string }): string {
  switch (h.eventType) {
    case "CREATED":
      return "created this ticket";
    case "ASSIGNED":
      return h.newValue ? `assigned this ticket` : "unassigned this ticket";
    case "STATUS_CHANGED":
      return `changed status from ${STATUS_LABELS[h.oldValue as TicketStatus] ?? h.oldValue} to ${STATUS_LABELS[h.newValue as TicketStatus] ?? h.newValue}`;
    case "SEVERITY_CHANGED":
      return `changed severity from ${h.oldValue} to ${h.newValue}`;
    case "PRIORITY_CHANGED":
      return `changed priority from ${h.oldValue} to ${h.newValue}`;
    case "COMMENT_ADDED":
      return "added a comment";
    case "ATTACHMENT_ADDED":
      return `uploaded ${h.newValue}`;
    default:
      return h.eventType.toLowerCase().replace(/_/g, " ");
  }
}
