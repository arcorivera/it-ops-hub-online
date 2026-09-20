"use client";

import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";

import { ticketsApi } from "@/lib/api/tickets";
import { teamsApi } from "@/lib/api/teams";
import { projectsApi } from "@/lib/api/projects";
import { ApiError } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";

const schema = z.object({
  title: z.string().min(1, "Title is required"),
  description: z.string().optional(),
  severity: z.enum(["S1", "S2", "S3", "S4"]),
  priority: z.enum(["Critical", "High", "Medium", "Low"]),
  environment: z.enum(["DEV", "UAT", "PRE-PROD", "PRODUCTION"]),
  teamId: z.string().optional(),
  projectId: z.string().optional(),
});

type FormData = z.infer<typeof schema>;

export default function NewTicketPage() {
  const router = useRouter();

  const { data: teams } = useQuery({ queryKey: ["teams"], queryFn: teamsApi.list });
  const { data: projects } = useQuery({ queryKey: ["projects"], queryFn: () => projectsApi.list() });

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { severity: "S3", priority: "Medium", environment: "PRODUCTION" },
  });

  const mutation = useMutation({
    mutationFn: (data: FormData) =>
      ticketsApi.create({
        title: data.title,
        description: data.description,
        severity: data.severity,
        priority: data.priority,
        environment: data.environment,
        teamId: data.teamId || null,
        projectId: data.projectId || null,
      }),
    onSuccess: (ticket) => {
      router.push(`/tickets/${ticket.id}`);
    },
    onError: (err) => {
      if (err instanceof ApiError && err.details) {
        for (const [field, message] of Object.entries(err.details)) {
          setError(field as keyof FormData, { message });
        }
      } else if (err instanceof ApiError) {
        setError("root", { message: err.message });
      }
    },
  });

  return (
    <div className="mx-auto max-w-2xl px-8 py-8">
      <h1 className="mb-6 text-2xl font-semibold text-slate-900">Create Ticket</h1>

      <Card>
        <CardHeader>
          <CardTitle>Ticket Details</CardTitle>
          <CardDescription>
            A ticket number and SLA deadlines will be assigned automatically based on severity.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={handleSubmit((data) => mutation.mutate(data))}>
            <div className="space-y-1.5">
              <Label htmlFor="title">Title</Label>
              <Input id="title" autoFocus {...register("title")} />
              {errors.title && <p className="text-xs text-red-600">{errors.title.message}</p>}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="description">Description</Label>
              <textarea
                id="description"
                rows={4}
                className="flex w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
                {...register("description")}
              />
            </div>

            <div className="grid grid-cols-3 gap-4">
              <div className="space-y-1.5">
                <Label htmlFor="severity">Severity</Label>
                <select
                  id="severity"
                  className="h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm"
                  {...register("severity")}
                >
                  <option value="S1">S1 Critical</option>
                  <option value="S2">S2 High</option>
                  <option value="S3">S3 Medium</option>
                  <option value="S4">S4 Low</option>
                </select>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="priority">Priority</Label>
                <select
                  id="priority"
                  className="h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm"
                  {...register("priority")}
                >
                  <option value="Critical">Critical</option>
                  <option value="High">High</option>
                  <option value="Medium">Medium</option>
                  <option value="Low">Low</option>
                </select>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="environment">Environment</Label>
                <select
                  id="environment"
                  className="h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm"
                  {...register("environment")}
                >
                  <option value="DEV">DEV</option>
                  <option value="UAT">UAT</option>
                  <option value="PRE-PROD">PRE-PROD</option>
                  <option value="PRODUCTION">PRODUCTION</option>
                </select>
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="teamId">Team (optional)</Label>
              <select
                id="teamId"
                className="h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm"
                {...register("teamId")}
              >
                <option value="">Unassigned</option>
                {teams?.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name}
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="projectId">Project (optional)</Label>
              <select
                id="projectId"
                className="h-10 w-full rounded-md border border-slate-300 bg-white px-3 text-sm"
                {...register("projectId")}
              >
                <option value="">None</option>
                {projects?.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.projectKey} — {p.name}
                  </option>
                ))}
              </select>
            </div>

            {errors.root && <p className="text-xs text-red-600">{errors.root.message}</p>}

            <div className="flex gap-3 pt-2">
              <Button type="submit" disabled={mutation.isPending}>
                {mutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : "Create Ticket"}
              </Button>
              <Button type="button" variant="outline" onClick={() => router.push("/tickets")}>
                Cancel
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
