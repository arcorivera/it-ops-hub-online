"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Plus, X, FolderKanban } from "lucide-react";

import { projectsApi, PROJECT_STATUS_LABELS } from "@/lib/api/projects";
import { ApiError } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const schema = z.object({
  projectKey: z.string().min(1, "Required").max(20, "Keep it short"),
  name: z.string().min(1, "Required"),
  description: z.string().optional(),
  startDate: z.string().optional(),
  targetDate: z.string().optional(),
});
type FormData = z.infer<typeof schema>;

const statusTone: Record<string, "slate" | "blue" | "amber" | "emerald" | "red"> = {
  PLANNING: "slate",
  ACTIVE: "blue",
  ON_HOLD: "amber",
  COMPLETED: "emerald",
  CANCELLED: "red",
};

export default function ProjectsPage() {
  const [showForm, setShowForm] = useState(false);
  const queryClient = useQueryClient();

  const { data: projects, isLoading } = useQuery({ queryKey: ["projects"], queryFn: () => projectsApi.list() });

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors },
  } = useForm<FormData>({ resolver: zodResolver(schema) });

  const createMutation = useMutation({
    mutationFn: projectsApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["projects"] });
      reset();
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
          <h1 className="text-2xl font-semibold text-slate-900">Projects</h1>
          <p className="mt-1 text-sm text-slate-500">Track initiatives and the tickets that belong to them.</p>
        </div>
        <Button onClick={() => setShowForm((s) => !s)}>
          {showForm ? <X className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
          {showForm ? "Cancel" : "New Project"}
        </Button>
      </div>

      {showForm && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Create Project</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="space-y-4" onSubmit={handleSubmit((data) => createMutation.mutate(data))}>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <Label htmlFor="projectKey">Project Key</Label>
                  <Input id="projectKey" placeholder="ICARE-MOB" {...register("projectKey")} />
                  {errors.projectKey && <p className="text-xs text-red-600">{errors.projectKey.message}</p>}
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="name">Name</Label>
                  <Input id="name" {...register("name")} />
                  {errors.name && <p className="text-xs text-red-600">{errors.name.message}</p>}
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="description">Description</Label>
                <Input id="description" {...register("description")} />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <Label htmlFor="startDate">Start Date</Label>
                  <Input id="startDate" type="date" {...register("startDate")} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="targetDate">Target Date</Label>
                  <Input id="targetDate" type="date" {...register("targetDate")} />
                </div>
              </div>
              {errors.root && <p className="text-xs text-red-600">{errors.root.message}</p>}
              <Button type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : "Create Project"}
              </Button>
            </form>
          </CardContent>
        </Card>
      )}

      {isLoading ? (
        <div className="flex justify-center py-10">
          <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : projects && projects.length > 0 ? (
        <div className="grid grid-cols-2 gap-4">
          {projects.map((p) => (
            <Link key={p.id} href={`/projects/${p.id}`}>
              <Card className="transition-shadow hover:shadow-md">
                <CardContent className="p-5">
                  <div className="mb-2 flex items-center justify-between">
                    <span className="font-mono text-xs text-slate-400">{p.projectKey}</span>
                    <Badge variant={statusTone[p.status]}>{PROJECT_STATUS_LABELS[p.status]}</Badge>
                  </div>
                  <h3 className="font-medium text-slate-900">{p.name}</h3>
                  {p.description && <p className="mt-1 line-clamp-2 text-sm text-slate-500">{p.description}</p>}
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-slate-200 py-16 text-center text-sm text-slate-400">
          <FolderKanban className="h-6 w-6" />
          No projects yet.
        </div>
      )}
    </div>
  );
}
